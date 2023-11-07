package models

import (
	"time"

	"github.com/google/uuid"
)

// Invitation is a request for a user to join an organization
type Invitation struct {
	Base
	UserID         uuid.UUID `json:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type AddInvitation struct {
	UserName       *string    `json:"user_name"` // The username to invite (one of username or user_id is required)
	UserID         *uuid.UUID `json:"user_id"`   // The user id to invite (one of username or user_id is required)
	OrganizationID uuid.UUID  `json:"organization_id"`
}
