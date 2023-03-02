package models

import (
	"time"

	"github.com/google/uuid"
)

// Invitation is a request for a user to join an organization
type Invitation struct {
	Base
	UserID         string    `json:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Expiry         time.Time `json:"expiry"`
}

func NewInvitation(userID string, orgID uuid.UUID) Invitation {
	// invitation expires after 1 week
	expiry := time.Now().Add(time.Hour * 24 * 7)
	return Invitation{
		UserID:         userID,
		OrganizationID: orgID,
		Expiry:         expiry,
	}
}

type AddInvitation struct {
	UserID         string    `json:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
}
