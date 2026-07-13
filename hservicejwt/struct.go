package hservicejwt

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ServiceJwt struct {
	// The IssuedAt contains the time the token was issued. It must always be non zero.
	IssuedAt time.Time `json:"iat"`
	// The ExpiresAt time contains the time the token should be considered expired. It must always be set and must be
	// later than IssuedAt.
	ExpiresAt time.Time `json:"exp"`
	// SubjectUserId contains the subject id of the user this token represents.
	SubjectUserId string `json:"sub"`

	// DelegatedRoles contains a set of object roles that have been delegated to this token by a user.
	DelegatedRoles map[string]string `json:"delegated_roles"`
}

// ValidateStateless validates the stateless claims of the JWT that must always apply. Any more complex validation must
// be done in separate methods. This does not check expiration.
func (s ServiceJwt) ValidateStateless() error {
	// validate stateless claims
	if s.IssuedAt.IsZero() {
		return &ErrInvalidJwtClaim{Inner: errors.New("issued-at time is zero")}
	} else if s.ExpiresAt.Before(s.IssuedAt) {
		return &ErrInvalidJwtClaim{Inner: errors.New("expires-at time is not before issued-at time")}
	} else if _, err := uuid.Parse(strings.TrimPrefix(s.SubjectUserId, "s-")); err != nil {
		return &ErrInvalidJwtClaim{Inner: errors.New("subject user id is not valid")}
	}
	return nil
}
