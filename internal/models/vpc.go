package models

import (
	"github.com/google/uuid"
)

// VPC contains Devices
type VPC struct {
	Base
	OrganizationID uuid.UUID     `json:"organization_id"`
	Description    string        `json:"description"`
	PrivateCidr    bool          `json:"private_cidr"`
	Ipv4Cidr       string        `json:"ipv4_cidr"`
	Ipv6Cidr       string        `json:"ipv6_cidr"`
	Organization   *Organization `json:"-"`
	Revision       uint64        `json:"revision" gorm:"type:bigserial;index:"`
}

type AddVPC struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	Description    string    `json:"description" example:"The Red Zone"`
	PrivateCidr    bool      `json:"private_cidr"`
	Ipv4Cidr       string    `json:"ipv4_cidr" example:"172.16.42.0/24"`
	Ipv6Cidr       string    `json:"ipv6_cidr" example:"0200::/8"`
}

type UpdateVPC struct {
	Description *string `json:"description" example:"The Red Zone"`
}
