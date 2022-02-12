package configmap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/logandavies181/arnlike"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"

	"sigs.k8s.io/aws-iam-authenticator/pkg/config"
	"sigs.k8s.io/aws-iam-authenticator/pkg/metrics"
)

type MapStore struct {
	mutex        sync.RWMutex
	users        map[string]config.UserMapping
	userArnLikes map[string]config.UserMapping
	roles        map[string]config.RoleMapping
	roleArnLikes map[string]config.RoleMapping
	// Used as set.
	awsAccounts map[string]interface{}
	configMap   v1.ConfigMapInterface
	metrics     metrics.Metrics
}

func New(masterURL, kubeConfig string, authenticatorMetrics metrics.Metrics) (*MapStore, error) {
	clientconfig, err := clientcmd.BuildConfigFromFlags(masterURL, kubeConfig)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(clientconfig)
	if err != nil {
		return nil, err
	}

	ms := MapStore{}
	ms.configMap = clientset.CoreV1().ConfigMaps("kube-system")
	ms.metrics = authenticatorMetrics
	return &ms, nil
}

// Starts a go routine which will watch the configmap and update the in memory data
// when the values change.
func (ms *MapStore) startLoadConfigMap(stopCh <-chan struct{}) {
	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
				watcher, err := ms.configMap.Watch(context.TODO(), metav1.ListOptions{
					Watch:         true,
					FieldSelector: fields.OneTermEqualSelector("metadata.name", "aws-auth").String(),
				})
				if err != nil {
					logrus.Errorf("Unable to re-establish watch: %v, sleeping for 5 seconds.", err)
					ms.metrics.ConfigMapWatchFailures.Inc()
					time.Sleep(5 * time.Second)
					continue
				}

				for r := range watcher.ResultChan() {
					switch r.Type {
					case watch.Error:
						logrus.WithFields(logrus.Fields{"error": r}).Error("recieved a watch error")
					case watch.Deleted:
						logrus.Info("Resetting configmap on delete")
						userMappings := make([]config.UserMapping, 0)
						userArnLikeMappings := make([]config.UserMapping, 0)
						roleMappings := make([]config.RoleMapping, 0)
						roleArnLikeMappings := make([]config.RoleMapping, 0)
						awsAccounts := make([]string, 0)
						ms.saveMap(userMappings, userArnLikeMappings, roleMappings, roleArnLikeMappings, awsAccounts)
					case watch.Added, watch.Modified:
						switch cm := r.Object.(type) {
						case *core_v1.ConfigMap:
							if cm.Name != "aws-auth" {
								break
							}
							logrus.Info("Received aws-auth watch event")
							userMappings, userArnLikeMappings, roleMappings, roleArnLikeMappings, awsAccounts, err := ParseMap(cm.Data)
							if err != nil {
								logrus.Errorf("There was an error parsing the config maps.  Only saving data that was good, %+v", err)
							}
							ms.saveMap(userMappings, userArnLikeMappings, roleMappings, roleArnLikeMappings, awsAccounts)
							if err != nil {
								logrus.Error(err)
							}
						}

					}
				}
				logrus.Error("Watch channel closed.")
			}
		}
	}()
}

type ErrParsingMap struct {
	errors []error
}

func (err ErrParsingMap) Error() string {
	return fmt.Sprintf("error parsing config map: %v", err.errors)
}

func ParseMap(m map[string]string) (userMappings, userArnLikeMappings []config.UserMapping, roleMappings, roleArnLikeMappings []config.RoleMapping, awsAccounts []string, err error) {
	errs := make([]error, 0)
	rawUserMappings := make([]config.UserMapping, 0)
	userMappings = make([]config.UserMapping, 0)
	userArnLikeMappings = make([]config.UserMapping, 0)
	if userData, ok := m["mapUsers"]; ok {
		userJson, err := utilyaml.ToJSON([]byte(userData))
		if err != nil {
			errs = append(errs, err)
		} else {
			err = json.Unmarshal(userJson, &rawUserMappings)
			if err != nil {
				errs = append(errs, err)
			}

			for _, userMapping := range rawUserMappings {
				switch {
				case userMapping.UserARN == "" && userMapping.UserARNLike == "":
					errs = append(errs, fmt.Errorf("One of userarn or userarnLike must be supplied"))
				case userMapping.UserARN != "" && userMapping.UserARNLike != "":
					errs = append(errs, fmt.Errorf("Only one of userarn or userarnLike can be supplied"))
				case userMapping.UserARN != "":
					userMappings = append(userMappings, userMapping)
				case userMapping.UserARNLike != "":
					ok, err := arnlike.ArnLike(userMapping.UserARNLike, "arn:*:iam:*:*:user/*")
					if err != nil {
						errs = append(errs, err)
					} else if !ok {
						errs = append(errs, fmt.Errorf("UserARNLike '%s' did not match an ARN for an IAM User", userMapping.UserARNLike))
					}
					userArnLikeMappings = append(userArnLikeMappings, userMapping)
				default:
					errs = append(errs, fmt.Errorf("Unexpected error parsing userMapping: %v", userMapping))
				}
			}
		}
	}

	rawRoleMappings := make([]config.RoleMapping, 0)
	roleMappings = make([]config.RoleMapping, 0)
	roleArnLikeMappings = make([]config.RoleMapping, 0)
	if roleData, ok := m["mapRoles"]; ok {
		roleJson, err := utilyaml.ToJSON([]byte(roleData))
		if err != nil {
			errs = append(errs, err)
		} else {
			err = json.Unmarshal(roleJson, &rawRoleMappings)
			if err != nil {
				errs = append(errs, err)
			}

			for _, roleMapping := range rawRoleMappings {
				switch {
				case roleMapping.RoleARN == "" && roleMapping.RoleARNLike == "":
					errs = append(errs, fmt.Errorf("One of rolearn or rolearnLike must be supplied"))
				case roleMapping.RoleARN != "" && roleMapping.RoleARNLike != "":
					errs = append(errs, fmt.Errorf("Only one of rolearn or rolearnLike can be supplied"))
				case roleMapping.RoleARN != "":
					roleMappings = append(roleMappings, roleMapping)
				case roleMapping.RoleARNLike != "":
					ok, err := arnlike.ArnLike(roleMapping.RoleARNLike, "arn:*:iam:*:*:role/*")
					if err != nil {
						errs = append(errs, err)
					} else if !ok {
						errs = append(errs, fmt.Errorf("RoleARNLike '%s' did not match an ARN for an IAM Role", roleMapping.RoleARNLike))
					}
					roleArnLikeMappings = append(roleArnLikeMappings, roleMapping)
				default:
					errs = append(errs, fmt.Errorf("Unexpected error parsing roleMapping: %v", roleMapping))
				}
			}
		}
	}

	awsAccounts = make([]string, 0)
	if accountsData, ok := m["mapAccounts"]; ok {
		err := yaml.Unmarshal([]byte(accountsData), &awsAccounts)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		logrus.Warnf("Errors parsing configmap: %+v", errs)
		err = ErrParsingMap{errors: errs}
	}
	return userMappings, userArnLikeMappings, roleMappings, roleArnLikeMappings, awsAccounts, err
}

func EncodeMap(userMappings []config.UserMapping, roleMappings []config.RoleMapping, awsAccounts []string) (m map[string]string, err error) {
	m = make(map[string]string)

	if len(userMappings) > 0 {
		body, err := yaml.Marshal(userMappings)
		if err != nil {
			return nil, err
		}
		m["mapUsers"] = string(body)
	}

	if len(roleMappings) > 0 {
		body, err := yaml.Marshal(roleMappings)
		if err != nil {
			return nil, err
		}
		m["mapRoles"] = string(body)
	}

	if len(awsAccounts) > 0 {
		body, err := yaml.Marshal(awsAccounts)
		if err != nil {
			return nil, err
		}
		m["mapAccounts"] = string(body)
	}

	return m, nil
}

func (ms *MapStore) saveMap(
	userMappings []config.UserMapping,
	userArnLikeMappings []config.UserMapping,
	roleMappings []config.RoleMapping,
	roleArnLikeMappings []config.RoleMapping,
	awsAccounts []string) {

	ms.mutex.Lock()
	defer ms.mutex.Unlock()
	ms.users = make(map[string]config.UserMapping)
	ms.userArnLikes = make(map[string]config.UserMapping)
	ms.roles = make(map[string]config.RoleMapping)
	ms.roleArnLikes = make(map[string]config.RoleMapping)
	ms.awsAccounts = make(map[string]interface{})

	for _, user := range userMappings {
		ms.users[strings.ToLower(user.UserARN)] = user
	}
	for _, userArnLike := range userArnLikeMappings {
		ms.userArnLikes[userArnLike.UserARNLike] = userArnLike
	}
	for _, role := range roleMappings {
		ms.roles[strings.ToLower(role.RoleARN)] = role
	}
	for _, roleArnLike := range roleArnLikeMappings {
		ms.roleArnLikes[roleArnLike.RoleARNLike] = roleArnLike
	}
	for _, awsAccount := range awsAccounts {
		ms.awsAccounts[awsAccount] = nil
	}
}

// UserNotFound is the error returned when the user is not found in the config map.
var UserNotFound = errors.New("User not found in configmap")

// UserARNLikeNotMatched is the error returned when an ARN is not matched to any UserArnLike patterns in the configmap
var UserARNLikeNotMatched = errors.New("User not matched to any UserARNLike strings in configmap")

// RoleNotFound is the error returned when the role is not found in the config map.
var RoleNotFound = errors.New("Role not found in configmap")

// RoleARNLikeNotMatched is the error returned when an ARN is not matched to any RoleArnLike patterns in the configmap
var RoleARNLikeNotMatched = errors.New("Role not matched to any RoleARNLike strings in configmap")

func (ms *MapStore) UserMapping(arn string) (config.UserMapping, error) {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	if user, ok := ms.users[arn]; !ok {
		return config.UserMapping{}, UserNotFound
	} else {
		return user, nil
	}
}

func (ms *MapStore) UserArnLikeMapping(arn string) (config.UserMapping, error) {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	for _, userArnLike := range ms.userArnLikes {
		ok, err := arnlike.ArnLike(arn, userArnLike.UserARNLike)
		if err != nil {
			return config.UserMapping{}, err
		}

		if ok {
			return userArnLike, nil
		}
	}

	return config.UserMapping{}, UserARNLikeNotMatched
}

func (ms *MapStore) RoleMapping(arn string) (config.RoleMapping, error) {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	if role, ok := ms.roles[arn]; !ok {
		return config.RoleMapping{}, RoleNotFound
	} else {
		return role, nil
	}
}

func (ms *MapStore) RoleArnLikeMapping(arn string) (config.RoleMapping, error) {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	for _, roleArnLike := range ms.roleArnLikes {
		ok, err := arnlike.ArnLike(arn, roleArnLike.RoleARNLike)
		if err != nil {
			return config.RoleMapping{}, err
		}

		if ok {
			return roleArnLike, nil
		}
	}

	return config.RoleMapping{}, RoleARNLikeNotMatched
}

func (ms *MapStore) AWSAccount(id string) bool {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	_, ok := ms.awsAccounts[id]
	return ok
}
