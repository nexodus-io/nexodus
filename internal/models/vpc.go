package models

import (
	"github.com/google/uuid"
)

// VPC contains Devices
type VPC struct {
	Base
	OrganizationID uuid.UUID `json:"organization_id"`
	Description    string    `json:"description"`
	PrivateCidr    bool      `json:"private_cidr"`
	IpCidr         string    `json:"cidr"`
	IpCidrV6       string    `json:"cidr_v6"`
	HubZone        bool      `json:"hub_zone"`

	Organization *Organization `json:"-"`
}

type AddVPC struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	Description    string    `json:"description" example:"The Red Zone"`
	PrivateCidr    bool      `json:"private_cidr"`
	IpCidr         string    `json:"cidr" example:"172.16.42.0/24"`
	IpCidrV6       string    `json:"cidr_v6" example:"0200::/8"`
	HubZone        bool      `json:"hub_zone"`
}
