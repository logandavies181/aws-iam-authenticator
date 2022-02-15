package config

import (
	"fmt"

	"github.com/logandavies181/arnlike"
)

// Validate returns an error if the RoleMapping is not valid after being unmarshaled
func (m *RoleMapping) Validate() error {
	if m == nil {
		return fmt.Errorf("RoleMapping is nil")
	}

	if m.RoleARN == "" && m.RoleARNLike == "" {
		return fmt.Errorf("One of rolearn or rolearnLike must be supplied")
	} else if m.RoleARN != "" && m.RoleARNLike != "" {
		return fmt.Errorf("Only one of rolearn or rolearnLike can be supplied")
	}

	if m.RoleARNLike != "" {
		ok, err := arnlike.ArnLike(m.RoleARNLike, "arn:*:iam:*:*:role/*")
		if err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("RoleARNLike '%s' did not match an ARN for an IAM Role", m.RoleARNLike)
		}
	}

	return nil
}

// Matches returns true if the supplied ARN or ARN-like string matches
// this RoleMapping
func (m *RoleMapping) Matches(subject string) bool {
	if m.RoleARN != "" {
		return m.RoleARN == subject
	}

	// Assume the caller has called Validate(), which parses m.RoleARNLike
	// If subject is not parsable, then it cannot be a valid ARN anyway so
	// we can ignore the error here
	ok, _ := arnlike.ArnLike(subject, m.RoleARNLike)
	return ok
}

// Key returns RoleARN or RoleARNLike, whichever is not empty.
// Used to get a Key name for map[string]RoleMapping
func (m *RoleMapping) Key() string {
	if m.RoleARN != "" {
		return m.RoleARN
	}
	return m.RoleARNLike
}

// Validate returns an error if the UserMapping is not valid after being unmarshaled
func (m *UserMapping) Validate() error {
	if m == nil {
		return fmt.Errorf("RoleMapping is nil")
	}

	if m.UserARN == "" && m.UserARNLike == "" {
		return fmt.Errorf("One of userarn or userarnLike must be supplied")
	} else if m.UserARN != "" && m.UserARNLike != "" {
		return fmt.Errorf("Only one of userarn or userarnLike can be supplied")
	}

	if m.UserARNLike != "" {
		ok, err := arnlike.ArnLike(m.UserARNLike, "arn:*:iam:*:*:user/*")
		if err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("RoleARNLike '%s' did not match an ARN for an IAM User", m.UserARNLike)
		}
	}

	return nil
}

// Matches returns true if the supplied ARN or ARN-like string matches
// this UserMapping
func (m *UserMapping) Matches(subject string) bool {
	if m.UserARN != "" {
		return m.UserARN == subject
	}

	// As per RoleMapping.Matches, we can ignore the error here
	ok, _ := arnlike.ArnLike(subject, m.UserARNLike)
	return ok
}

// Key returns UserARN or UserARNLike, whichever is not empty.
// Used to get a Key name for map[string]UserMapping
func (m *UserMapping) Key() string {
	if m.UserARN != "" {
		return m.UserARN
	}
	return m.UserARNLike
}
