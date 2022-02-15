package file

import (
	"reflect"
	"testing"

	"sigs.k8s.io/aws-iam-authenticator/pkg/config"
)

func newConfig() config.Config {
	return config.Config{
		RoleMappings: []config.RoleMapping{
			{
				RoleARN:  "arn:aws:iam::0123456789012:role/test-role",
				Username: "roland",
				Groups:   []string{"system:masters"},
			},
			{
				RoleARNLike: "arn:aws:iam::0123456789012:role/cookie-cutt*",
				Username:    "cookie-cutter",
				Groups:      []string{"system:masters"},
			},
		},
		UserMappings: []config.UserMapping{
			{
				UserARN:  "arn:aws:iam::0123456789012:user/donald",
				Username: "donald",
				Groups:   []string{"system:masters"},
			},
			{
				UserARNLike: "arn:aws:iam::0123456789012:user/shrey*",
				Username:    "shreyas",
				Groups:      []string{"system:masters"},
			},
		},
		AutoMappedAWSAccounts: []string{"000000000000"},
	}
}

func TestNewFileMapper(t *testing.T) {
	cfg := newConfig()

	expected := &FileMapper{
		lowercaseRoleMap: map[string]config.RoleMapping{
			"arn:aws:iam::0123456789012:role/test-role": {
				RoleARN:  "arn:aws:iam::0123456789012:role/test-role",
				Username: "roland",
				Groups:   []string{"system:masters"},
			},
			"arn:aws:iam::0123456789012:role/cookie-cutt*": {
				RoleARNLike: "arn:aws:iam::0123456789012:role/cookie-cutt*",
				Username:    "cookie-cutter",
				Groups:      []string{"system:masters"},
			},
		},
		lowercaseUserMap: map[string]config.UserMapping{
			"arn:aws:iam::0123456789012:user/donald": {
				UserARN:  "arn:aws:iam::0123456789012:user/donald",
				Username: "donald",
				Groups:   []string{"system:masters"},
			},
			"arn:aws:iam::0123456789012:user/shrey*": {
				UserARNLike: "arn:aws:iam::0123456789012:user/shrey*",
				Username:    "shreyas",
				Groups:      []string{"system:masters"},
			},
		},
		accountMap: map[string]bool{
			"000000000000": true,
		},
	}

	actual, err := NewFileMapper(cfg)
	if err != nil {
		t.Errorf("Could not build FileMapper from test config: %v", err)
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("FileMapper does not match expected value.\nActual:   %v\nExpected: %v", actual, expected)
	}
}

func TestMap(t *testing.T) {
	fm, err := NewFileMapper(newConfig())
	if err != nil {
		t.Errorf("Could not build FileMapper from test config: %v", err)
	}

	identityArn := "arn:aws:iam::0123456789012:role/test-role"
	expected := &config.IdentityMapping{
		IdentityARN: identityArn,
		Username:    "roland",
		Groups:      []string{"system:masters"},
	}
	actual, err := fm.Map(identityArn)
	if err != nil {
		t.Errorf("Could not map %s: %s", identityArn, err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("FileMapper.Map() does not match expected value for roleMapping:\nActual:   %v\nExpected: %v", actual, expected)
	}

	identityArn = "arn:aws:iam::0123456789012:role/cookie-cutt*"
	expected = &config.IdentityMapping{
		IdentityARN: identityArn,
		Username:    "cookie-cutter",
		Groups:      []string{"system:masters"},
	}
	actual, err = fm.Map(identityArn)
	if err != nil {
		t.Errorf("Could not map %s: %s", identityArn, err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("FileMapper.Map() does not match expected value for roleArnLikeMapping:\nActual:   %v\nExpected: %v", actual, expected)
	}

	identityArn = "arn:aws:iam::0123456789012:user/donald"
	expected = &config.IdentityMapping{
		IdentityARN: identityArn,
		Username:    "donald",
		Groups:      []string{"system:masters"},
	}
	actual, err = fm.Map(identityArn)
	if err != nil {
		t.Errorf("Could not map %s: %s", identityArn, err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("FileMapper.Map() does not match expected value for userMapping:\nActual:   %v\nExpected: %v", actual, expected)
	}

	identityArn = "arn:aws:iam::0123456789012:user/shrey*"
	expected = &config.IdentityMapping{
		IdentityARN: identityArn,
		Username:    "shreyas",
		Groups:      []string{"system:masters"},
	}
	actual, err = fm.Map(identityArn)
	if err != nil {
		t.Errorf("Could not map %s: %s", identityArn, err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("FileMapper.Map() does not match expected value for userArnLikeMapping:\nActual:   %v\nExpected: %v", actual, expected)
	}
}
