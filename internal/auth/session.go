package auth

import (
	"time"

	"github.com/go-playground/validator/v10"
)

// Session mirrors the "sessions" table (migrations/001_initial_schema.sql).
//
// TokenHash is never serialized to JSON (json:"-"): it is server-side secret
// material and must not leak into API responses or logs.
type Session struct {
	ID        string     `json:"id" db:"id" validate:"required,uuid"`
	UserID    string     `json:"userId" db:"user_id" validate:"required,uuid"`
	TokenHash []byte     `json:"-" db:"token_hash" validate:"required,len=32"`
	CreatedAt time.Time  `json:"createdAt" db:"created_at" validate:"required"`
	ExpiresAt time.Time  `json:"expiresAt" db:"expires_at" validate:"required,gtfield=CreatedAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty" db:"revoked_at" validate:"omitempty,gtefield=CreatedAt"`
}

var validate = validator.New()

// Validate checks the Session against the same rules the database enforces.
func (s *Session) Validate() error {
	return validate.Struct(s)
}
