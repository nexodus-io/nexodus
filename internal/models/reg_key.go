package models

import (
	"github.com/golang-jwt/jwt/v4"
	"time"

	"github.com/google/uuid"
)

// RegKey is used to register devices without an interactive login.
type RegKey struct {
	Base
	OwnerID         uuid.UUID              `json:"owner_id,omitempty"`                          // OwnerID is the ID of the user that created the registration key.
	VpcID           uuid.UUID              `json:"vpc_id,omitempty"`                            // VpcID is the ID of the VPC the device will join.
	OrganizationID  uuid.UUID              `json:"-"`                                           // OrganizationID is denormalized from the VPC record for performance
	BearerToken     string                 `json:"bearer_token,omitempty"`                      // BearerToken is the bearer token the client should use to authenticate the device registration request.
	Description     string                 `json:"description,omitempty"`                       // Description of the registration key.
	DeviceId        *uuid.UUID             `json:"device_id,omitempty"`                         // DeviceId is set if the RegKey was created for single use
	ExpiresAt       *time.Time             `json:"expires_at,omitempty"`                        // ExpiresAt is optional, if set the registration key is only valid until the ExpiresAt time.
	SecurityGroupId *uuid.UUID             `json:"security_group_id"`                           // SecurityGroupId is the ID of the security group to assign to the device.
	Settings        map[string]interface{} `json:"settings" gorm:"type:JSONB; serializer:json"` // Settings contains general settings for the device.
}
type NexodusClaims struct {
	jwt.RegisteredClaims
	Scope    string    `json:"scope,omitempty"`     // Scope is the scope of the token.
	VpcID    uuid.UUID `json:"vpc_id,omitempty"`    // VpcID is the ID of the VPC the device will join.
	DeviceID uuid.UUID `json:"device_id,omitempty"` // DeviceID is the ID of the device.
}

type AddRegKey struct {
	VpcID           uuid.UUID              `json:"vpc_id,omitempty"`      // VpcID is the ID of the VPC the device will join.
	Description     string                 `json:"description,omitempty"` // Description of the registration key.
	SingleUse       bool                   `json:"single_use,omitempty"`  // SingleUse only allows the registration key to be used once.
	ExpiresAt       *time.Time             `json:"expires_at,omitempty"`  // ExpiresAt is optional, if set the registration key is only valid until the ExpiresAt time.
	SecurityGroupId *uuid.UUID             `json:"security_group_id"`     // SecurityGroupId is the ID of the security group to assign to the device.
	Settings        map[string]interface{} `json:"settings"`              // Settings contains general settings for the device.
}

type UpdateRegKey struct {
	Description     *string                `json:"description,omitempty"` // Description of the registration key.
	ExpiresAt       *time.Time             `json:"expires_at,omitempty"`  // ExpiresAt is optional, if set the registration key is only valid until the ExpiresAt time.
	SecurityGroupId *uuid.UUID             `json:"security_group_id"`     // SecurityGroupId is the ID of the security group to assign to the device.
	Settings        map[string]interface{} `json:"settings"`              // Settings contains general settings for the device.
}
