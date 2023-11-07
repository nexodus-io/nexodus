package models

import (
	"github.com/google/uuid"
)

// VPC contains Devices
type VPC struct {
	Base
	OrganizationID  uuid.UUID `json:"organization_id"`
	Description     string    `json:"description"`
	PrivateCidr     bool      `json:"private_cidr"`
	Ipv4Cidr        string    `json:"ipv4_cidr"`
	Ipv6Cidr        string    `json:"ipv6_cidr"`
	SecurityGroupId uuid.UUID `json:"security_group_id"`

	Organization *Organization `json:"-"`
}

type AddVPC struct {
	OrganizationID  uuid.UUID `json:"organization_id"`
	Description     string    `json:"description" example:"The Red Zone"`
	PrivateCidr     bool      `json:"private_cidr"`
	Ipv4Cidr        string    `json:"ipv4_cidr" example:"172.16.42.0/24"`
	Ipv6Cidr        string    `json:"ipv6_cidr" example:"0200::/8"`
	SecurityGroupId uuid.UUID `json:"security_group_id"`
}

type UpdateVPC struct {
	Description     *string   `json:"description" example:"The Red Zone"`
	SecurityGroupId uuid.UUID `json:"security_group_id"`
}
