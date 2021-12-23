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
	lowercaseUserMap map[string]config.UserMapping
	arnLikeList      []config.ARNLikeMapping
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
		canonicalizedARN, err := arn.Canonicalize(strings.ToLower(m.RoleARN))
		if err != nil {
			return nil, fmt.Errorf("error canonicalizing ARN: %v", err)
		}
		fileMapper.lowercaseRoleMap[canonicalizedARN] = m
	}
	for _, m := range cfg.UserMappings {
		canonicalizedARN, err := arn.Canonicalize(strings.ToLower(m.UserARN))
		if err != nil {
			return nil, fmt.Errorf("error canonicalizing ARN: %v", err)
		}
		fileMapper.lowercaseUserMap[canonicalizedARN] = m
	}
	for _, m := range cfg.ARNLikeMappings {
		// TODO: canonicalize or validate the ARNLike strings
		if fileMapper.arnLikeList == nil {
			fileMapper.arnLikeList = []config.ARNLikeMapping{m}
		} else {
			fileMapper.arnLikeList = append(fileMapper.arnLikeList, m)
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

	for _, arnLikeMapping := range m.arnLikeList {
		//g := glob.MustCompile(arnLikeMapping.ARNLike)
		matched, err := arnlike.ArnLike(canonicalARN, arnLikeMapping.ARNLike)
		if err != nil {
			return nil, err
		}

		if matched {
			return &config.IdentityMapping{
				IdentityARN: canonicalARN,
				Username:    arnLikeMapping.Username,
				Groups:      arnLikeMapping.Groups,
			}, nil
		}
	}

	return nil, mapper.ErrNotMapped
}

func (m *FileMapper) IsAccountAllowed(accountID string) bool {
	return m.accountMap[accountID]
}
