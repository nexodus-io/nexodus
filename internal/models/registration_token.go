package models

import (
	"github.com/golang-jwt/jwt/v4"
	"time"

	"github.com/google/uuid"
)

// RegistrationToken is used to register devices without an interactive login.
type RegistrationTokenRecord struct {
	Base
	UserID         string
	OrganizationID uuid.UUID
	// BearerToken is the token the client should use to authenticate the device registration request.
	BearerToken string
	Description string
}

func (RegistrationTokenRecord) TableName() string {
	return "registration_tokens"
}

type NexodusClaims struct {
	jwt.RegisteredClaims
	Scope          string    `json:"scope,omitempty"`
	OrganizationID uuid.UUID `json:"org,omitempty"`
	DeviceID       uuid.UUID `json:"device,omitempty"`
}

type AddRegistrationToken struct {
	OrganizationID uuid.UUID `json:"organization_id,omitempty"`
	Description    string    `json:"description,omitempty"`
	// SingleUse only allows the registration token to be used once.
	SingleUse bool `json:"single_use,omitempty"`
	// Expiration is optional, if set the registration token is only valid until the Expiration time.
	Expiration *time.Time `json:"expiration,omitempty"`
}

type RegistrationToken struct {
	Base
	UserID         string     `json:"user_id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	Expiration     *time.Time `json:"expiration,omitempty"`
	BearerToken    string     `json:"bearer_token,omitempty"`
	Description    string     `json:"description,omitempty"`
	DeviceID       *uuid.UUID `json:"device_id,omitempty"`
}
