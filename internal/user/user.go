package user

import (
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

// User mirrors the "users" table (migrations/001_initial_schema.sql).
type User struct {
	ID          string    `json:"id" db:"id" validate:"required,uuid"`
	Email       string    `json:"email" db:"email" validate:"required,email,lowercase,max=320"`
	DisplayName string    `json:"displayName" db:"display_name" validate:"required,notblank,max=255"`
	AvatarURL   *string   `json:"avatarUrl,omitempty" db:"avatar_url" validate:"omitempty,url,max=2048"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at" validate:"required"`
	UpdatedAt   time.Time `json:"updatedAt" db:"updated_at" validate:"required,gtefield=CreatedAt"`
}

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New()
	// Mirrors CHECK (length(btrim(display_name)) > 0): whitespace-only is invalid.
	_ = v.RegisterValidation("notblank", func(fl validator.FieldLevel) bool {
		return strings.TrimSpace(fl.Field().String()) != ""
	})
	return v
}

// Validate checks the User against the same rules the database enforces.
func (u *User) Validate() error {
	return validate.Struct(u)
}
