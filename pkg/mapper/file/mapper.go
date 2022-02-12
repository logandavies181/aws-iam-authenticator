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
		if m.RoleARN == "" && m.RoleARNLike == "" {
			return nil, fmt.Errorf("One of roleARN or roleARNLike must be supplied")
		}

		if m.RoleARN != "" {
			canonicalizedARN, err := arn.Canonicalize(strings.ToLower(m.RoleARN))
			if err != nil {
				return nil, fmt.Errorf("error canonicalizing ARN: %v", err)
			}
			fileMapper.lowercaseRoleMap[canonicalizedARN] = m
		}

		if m.RoleARNLike != "" {
			ok, err := arnlike.ArnLike(m.RoleARNLike, "arn:*:iam:*:*:role/*")
			if err != nil {
				return nil, err
			} else if !ok {
				return nil, fmt.Errorf("RoleARNLike '%s' did not match an ARN for an IAM Role", m.RoleARNLike)
			}
			fileMapper.roleArnLikeMap[m.RoleARNLike] = m
		}
	}
	for _, m := range cfg.UserMappings {
		if m.UserARN == "" && m.UserARNLike == "" {
			return nil, fmt.Errorf("One of userARN or userARNLike must be supplied")
		}

		if m.UserARN != "" {
			canonicalizedARN, err := arn.Canonicalize(strings.ToLower(m.UserARN))
			if err != nil {
				return nil, fmt.Errorf("error canonicalizing ARN: %v", err)
			}
			fileMapper.lowercaseUserMap[canonicalizedARN] = m
		}

		if m.UserARNLike != "" {
			ok, err := arnlike.ArnLike(m.UserARNLike, "arn:*:iam:*:*:user/*")
			if err != nil {
				return nil, err
			} else if !ok {
				return nil, fmt.Errorf("UserARNLike '%s' did not match an ARN for an IAM User", m.UserARNLike)
			}
			fileMapper.userArnLikeMap[m.UserARNLike] = m
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
