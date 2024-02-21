package models

import "github.com/google/uuid"

// UserOrganization record means the user is a member of the organization
type UserOrganization struct {
	UserID         uuid.UUID `json:"user_id" gorm:"type:uuid;primary_key"`
	OrganizationID uuid.UUID `json:"organization_id" gorm:"type:uuid;primary_key"`
	User           *User     `json:"user,omitempty"`
}
