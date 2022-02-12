package file

import (
	"fmt"
	"strings"

	"github.com/logandavies181/arnlike"
	"sigs.k8s.io/aws-iam-authenticator/pkg/arn"
	"sigs.k8s.io/aws-iam-authenticator/pkg/config"
	"sigs.k8s.io/aws-iam-authenticator/pkg/mapper"
)

type FileMapper struct {
	lowercaseRoleMap map[string]config.RoleMapping
	roleArnLikeMap   map[string]config.RoleMapping
	lowercaseUserMap map[string]config.UserMapping
	userArnLikeMap   map[string]config.UserMapping
	accountMap       map[string]bool
}

var _ mapper.Mapper = &FileMapper{}

func NewFileMapper(cfg config.Config) (*FileMapper, error) {
	fileMapper := &FileMapper{
		lowercaseRoleMap: make(map[string]config.RoleMapping),
		roleArnLikeMap:   make(map[string]config.RoleMapping),
		lowercaseUserMap: make(map[string]config.UserMapping),
		userArnLikeMap:   make(map[string]config.UserMapping),
		accountMap:       make(map[string]bool),
	}

	for _, m := range cfg.RoleMappings {
		switch {
		case m.RoleARN == "" && m.RoleARNLike == "":
			return nil, fmt.Errorf("One of rolearn or rolearnLike must be supplied")
		case m.RoleARN != "" && m.RoleARNLike != "":
			return nil, fmt.Errorf("Only of rolearn or rolearnLike can be supplied")
		case m.RoleARN != "":
			canonicalizedARN, err := arn.Canonicalize(strings.ToLower(m.RoleARN))
			if err != nil {
				return nil, fmt.Errorf("error canonicalizing ARN: %v", err)
			}
			fileMapper.lowercaseRoleMap[canonicalizedARN] = m
		case m.RoleARNLike != "":
			ok, err := arnlike.ArnLike(m.RoleARNLike, "arn:*:iam:*:*:role/*")
			if err != nil {
				return nil, err
			} else if !ok {
				return nil, fmt.Errorf("RoleARNLike '%s' did not match an ARN for an IAM Role", m.RoleARNLike)
			}
			fileMapper.roleArnLikeMap[m.RoleARNLike] = m
		default:
			return nil, fmt.Errorf("Unexpected error parsing roleMapping: %v", m)
		}
	}
	for _, m := range cfg.UserMappings {
		switch {
		case m.UserARN == "" && m.UserARNLike == "":
			return nil, fmt.Errorf("One of userarn or userarnLike must be supplied")
		case m.UserARN != "" && m.UserARNLike != "":
			return nil, fmt.Errorf("Only one of userarn or userarnLike can be supplied")
		case m.UserARN != "":
			canonicalizedARN, err := arn.Canonicalize(strings.ToLower(m.UserARN))
			if err != nil {
				return nil, fmt.Errorf("error canonicalizing ARN: %v", err)
			}
			fileMapper.lowercaseUserMap[canonicalizedARN] = m
		case m.UserARNLike != "":
			ok, err := arnlike.ArnLike(m.UserARNLike, "arn:*:iam:*:*:user/*")
			if err != nil {
				return nil, err
			} else if !ok {
				return nil, fmt.Errorf("UserARNLike '%s' did not match an ARN for an IAM User", m.UserARNLike)
			}
			fileMapper.userArnLikeMap[m.UserARNLike] = m
		default:
			return nil, fmt.Errorf("Unexpected error parsing userMapping: %v", m)
		}

	}
	for _, m := range cfg.AutoMappedAWSAccounts {
		fileMapper.accountMap[m] = true
	}

	return fileMapper, nil
}

func NewFileMapperWithMaps(
	lowercaseRoleMap map[string]config.RoleMapping,
	lowercaseUserMap map[string]config.UserMapping,
	accountMap map[string]bool) *FileMapper {
	return &FileMapper{
		lowercaseRoleMap: lowercaseRoleMap,
		lowercaseUserMap: lowercaseUserMap,
		accountMap:       accountMap,
	}
}

func (m *FileMapper) Name() string {
	return mapper.ModeMountedFile
}

func (m *FileMapper) Start(_ <-chan struct{}) error {
	return nil
}

func (m *FileMapper) Map(canonicalARN string) (*config.IdentityMapping, error) {
	canonicalARN = strings.ToLower(canonicalARN)

	if roleMapping, exists := m.lowercaseRoleMap[canonicalARN]; exists {
		return &config.IdentityMapping{
			IdentityARN: canonicalARN,
			Username:    roleMapping.Username,
			Groups:      roleMapping.Groups,
		}, nil
	}

	if userMapping, exists := m.lowercaseUserMap[canonicalARN]; exists {
		return &config.IdentityMapping{
			IdentityARN: canonicalARN,
			Username:    userMapping.Username,
			Groups:      userMapping.Groups,
		}, nil
	}

	for _, roleArnLikeMapping := range m.roleArnLikeMap {
		matched, err := arnlike.ArnLike(canonicalARN, roleArnLikeMapping.RoleARNLike)
		if err != nil {
			return nil, err
		}

		if matched {
			return &config.IdentityMapping{
				IdentityARN: canonicalARN,
				Username:    roleArnLikeMapping.Username,
				Groups:      roleArnLikeMapping.Groups,
			}, nil
		}
	}

	for _, userArnLikeMapping := range m.userArnLikeMap {
		matched, err := arnlike.ArnLike(canonicalARN, userArnLikeMapping.UserARNLike)
		if err != nil {
			return nil, err
		}

		if matched {
			return &config.IdentityMapping{
				IdentityARN: canonicalARN,
				Username:    userArnLikeMapping.Username,
				Groups:      userArnLikeMapping.Groups,
			}, nil
		}
	}

	return nil, mapper.ErrNotMapped
}

func (m *FileMapper) IsAccountAllowed(accountID string) bool {
	return m.accountMap[accountID]
}
