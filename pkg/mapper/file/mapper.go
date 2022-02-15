package file

import (
	"fmt"
	"strings"

	"sigs.k8s.io/aws-iam-authenticator/pkg/arn"
	"sigs.k8s.io/aws-iam-authenticator/pkg/config"
	"sigs.k8s.io/aws-iam-authenticator/pkg/mapper"
)

type FileMapper struct {
	lowercaseRoleMap map[string]config.RoleMapping
	lowercaseUserMap map[string]config.UserMapping
	accountMap       map[string]bool
}

var _ mapper.Mapper = &FileMapper{}

func NewFileMapper(cfg config.Config) (*FileMapper, error) {
	fileMapper := &FileMapper{
		lowercaseRoleMap: make(map[string]config.RoleMapping),
		lowercaseUserMap: make(map[string]config.UserMapping),
		accountMap:       make(map[string]bool),
	}

	for _, m := range cfg.RoleMappings {
		err := m.Validate()
		if err != nil {
			return nil, err
		}
		var key string
		if m.RoleARN != "" {
			canonicalizedARN, err := arn.Canonicalize(strings.ToLower(m.RoleARN))
			if err != nil {
				return nil, fmt.Errorf("error canonicalizing ARN: %v", err)
			}
			key = canonicalizedARN
		} else {
			key = m.Key()
		}
		fileMapper.lowercaseRoleMap[key] = m
	}
	for _, m := range cfg.UserMappings {
		err := m.Validate()
		if err != nil {
			return nil, err
		}
		var key string
		if m.UserARN != "" {
			canonicalizedARN, err := arn.Canonicalize(strings.ToLower(m.Key()))
			if err != nil {
				return nil, fmt.Errorf("error canonicalizing ARN: %v", err)
			}
			key = canonicalizedARN
		} else {
			key = m.Key()
		}
		fileMapper.lowercaseUserMap[key] = m
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

	for _, roleMapping := range m.lowercaseRoleMap {
		if roleMapping.Matches(canonicalARN) {
			return &config.IdentityMapping{
				IdentityARN: canonicalARN,
				Username:    roleMapping.Username,
				Groups:      roleMapping.Groups,
			}, nil
		}
	}

	for _, userMapping := range m.lowercaseUserMap {
		if userMapping.Matches(canonicalARN) {
			return &config.IdentityMapping{
				IdentityARN: canonicalARN,
				Username:    userMapping.Username,
				Groups:      userMapping.Groups,
			}, nil
		}
	}

	return nil, mapper.ErrNotMapped
}

func (m *FileMapper) IsAccountAllowed(accountID string) bool {
	return m.accountMap[accountID]
}
