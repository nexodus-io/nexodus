package models

import (
	"github.com/google/uuid"
)

// UserIdentity is an identity of a user.  A user can have multiple identities.
type UserIdentity struct {
	Kind   string    `gorm:"primary_key"            json:"kind"  example:"email"`                        // email, phone, keycloak-id, etc
	Value  string    `gorm:"primary_key"            json:"value" example:"hiram@example.com"`            // the value of the identity
	UserID uuid.UUID `gorm:"type:uuid" json:"user_id"    example:"aa22666c-0f57-45cb-a449-16efecc04f2e"` // the id of the user
}
