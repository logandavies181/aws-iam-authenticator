// Package client implements client-side operations on auth configmap.
package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	core_v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	client_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/aws-iam-authenticator/pkg/config"
	"sigs.k8s.io/aws-iam-authenticator/pkg/mapper/configmap"
)

// Client defines configmap client methods.
type Client interface {
	AddRole(role *config.RoleMapping) (*core_v1.ConfigMap, error)
	AddUser(user *config.UserMapping) (*core_v1.ConfigMap, error)
}

const mapName = "aws-auth"

// New creates a new "Client".
func New(cli client_v1.ConfigMapInterface) Client {
	return &client{
		getMap: func() (*core_v1.ConfigMap, error) {
			return cli.Get(context.TODO(), mapName, meta_v1.GetOptions{})
		},
		updateMap: func(m *core_v1.ConfigMap) (cm *core_v1.ConfigMap, err error) {
			cm, err = cli.Update(context.TODO(), m, meta_v1.UpdateOptions{})
			return cm, err
		},
	}
}

type client struct {
	// define as function types for testing
	getMap    func() (*core_v1.ConfigMap, error)
	updateMap func(m *core_v1.ConfigMap) (cm *core_v1.ConfigMap, err error)
}

func (cli *client) AddRole(role *config.RoleMapping) (*core_v1.ConfigMap, error) {
	if role == nil {
		return nil, errors.New("empty role")
	}
	return cli.add(role, nil, nil)
}

func (cli *client) AddUser(user *config.UserMapping) (*core_v1.ConfigMap, error) {
	if user == nil {
		return nil, errors.New("empty user")
	}
	return cli.add(nil, user, nil)
}

func (cli *client) AddARNLikeMapping(arnLikeMapping *config.ARNLikeMapping) (*core_v1.ConfigMap, error) {
	if arnLikeMapping == nil {
		return nil, errors.New("empty arnLikeMapping")
	}
	return cli.add(nil, nil, arnLikeMapping)
}

func (cli *client) add(role *config.RoleMapping, user *config.UserMapping, arnLikeMapping *config.ARNLikeMapping) (cm *core_v1.ConfigMap, err error) {
	if role == nil && user == nil {
		return nil, errors.New("empty role/user")
	}
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cm, err = cli.getMap()
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				logrus.WithError(err).Warn("not found map " + mapName)
			}
			return err
		}

		data := cm.Data

		userMappings, roleMappings, arnLikeMappings, awsAccounts, err := configmap.ParseMap(data)
		if err != nil {
			return fmt.Errorf("failed to parse configmap %v", err)
		}

		if role != nil {
			for _, r := range roleMappings {
				if r.RoleARN == role.RoleARN {
					return fmt.Errorf("cannot add duplicate role ARN %q", role.RoleARN)
				}
			}
			roleMappings = append(roleMappings, *role)
		}

		if user != nil {
			for _, r := range userMappings {
				if r.UserARN == user.UserARN {
					return fmt.Errorf("cannot add duplicate user ARN %q", user.UserARN)
				}
			}
			userMappings = append(userMappings, *user)
		}

		if arnLikeMapping != nil {
			for _, a := range arnLikeMappings {
				if a.ARNLike == arnLikeMapping.ARNLike {
					return fmt.Errorf("cannot add duplicate ARN mapping pattern %q", arnLikeMapping.ARNLike)
				}
			}
			roleMappings = append(roleMappings, *role)

		}

		data, err = configmap.EncodeMap(userMappings, roleMappings, awsAccounts)
		if err != nil {
			return err
		}

		cm.Data = data

		updatedCm, err := cli.updateMap(cm)
		if err != nil {
			return err
		}

		cm = updatedCm
		return nil
	})
	return cm, err
}
