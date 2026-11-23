package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"issue-tracker/internal/database/sqlc"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgtype"
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

// sessionFromRow converts a sqlc row (pgtype fields) into the Session model.
// It fails if a column that is NOT NULL in the schema arrives as NULL.
func sessionFromRow(row sqlc.Session) (*Session, error) {
	id, err := uuidString(row.ID)
	if err != nil {
		return nil, fmt.Errorf("session id: %w", err)
	}
	userID, err := uuidString(row.UserID)
	if err != nil {
		return nil, fmt.Errorf("session user id: %w", err)
	}
	if !row.CreatedAt.Valid {
		return nil, fmt.Errorf("session created_at: null")
	}
	if !row.ExpiresAt.Valid {
		return nil, fmt.Errorf("session expires_at: null")
	}

	var revokedAt *time.Time
	if row.RevokedAt.Valid {
		t := row.RevokedAt.Time
		revokedAt = &t
	}

	return &Session{
		ID:        id,
		UserID:    userID,
		TokenHash: row.TokenHash,
		CreatedAt: row.CreatedAt.Time,
		ExpiresAt: row.ExpiresAt.Time,
		RevokedAt: revokedAt,
	}, nil
}

// uuidString formats a pgtype.UUID as the canonical 8-4-4-4-12 string.
func uuidString(u pgtype.UUID) (string, error) {
	if !u.Valid {
		return "", fmt.Errorf("uuid: null")
	}
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// GenerateSessionToken returns a new random session token for the client and
// its SHA-256 hash for storage. The database requires token_hash to be exactly
// 32 bytes, which SHA-256 produces.
func GenerateSessionToken() (token string, tokenHash []byte, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate session token: %w", err)
	}

	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashSessionToken(token), nil
}

// HashSessionToken returns the value stored in sessions.token_hash for a token.
// To look up a session from a token presented by the client.
func HashSessionToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
