package models

import (
	"time"

	"github.com/google/uuid"
)

// Invitation is a request for a user to join an organization
type Invitation struct {
	Base
	UserID         *uuid.UUID `json:"user_id,omitempty"`
	Email          *string    `json:"email,omitempty"` // The email address to invite
	OrganizationID uuid.UUID  `json:"organization_id"`
	ExpiresAt      time.Time  `json:"expires_at"`
}

type AddInvitation struct {
	Email          *string    `json:"email"`   // The email address of the user to invite (one of email or user_id is required)
	UserID         *uuid.UUID `json:"user_id"` // The user id to invite (one of email or user_id is required)
	OrganizationID uuid.UUID  `json:"organization_id"`
}
