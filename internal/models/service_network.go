package models

import (
	"github.com/google/uuid"
)

// ServiceNetwork contains interconnected Sites
type ServiceNetwork struct {
	Base
	OrganizationID uuid.UUID     `json:"organization_id" gorm:"type:uuid"`
	Organization   *Organization `json:"-"`
	Description    string        `json:"description"`
	CaKey          string        `json:"-"`
	CaCertificates []string      `json:"ca_certificates,omitempty" gorm:"type:JSONB; serializer:json"`
	Revision       uint64        `json:"revision" gorm:"type:bigserial;index:"`
}

type AddServiceNetwork struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	Description    string    `json:"description" example:"The Red Zone"`
}

type UpdateServiceNetwork struct {
	Description *string `json:"description" example:"The Red Zone"`
}
