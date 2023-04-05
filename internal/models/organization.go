package models

import (
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Organization contains Users and their Devices
type Organization struct {
	Base
	OwnerID     string    `json:"owner_id" gorm:"owner_id;"`
	Users       []*User   `json:"-" gorm:"many2many:user_organizations;"`
	Devices     []*Device `json:"-"`
	Name        string    `json:"name" gorm:"uniqueIndex" sql:"index"`
	Description string    `json:"description"`
	IpCidr      string    `json:"cidr"`
	IpCidrV6    string    `json:"cidr_v6"`
	HubZone     bool      `json:"hub_zone"`
	Invitations []*Invitation
}

// Organization contains Users and their Devices
type OrganizationJSON struct {
	ID          uuid.UUID `json:"id"`
	OwnerID     string    `json:"owner_id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	Name        string    `json:"name" example:"zone-red"`
	Description string    `json:"description" example:"The Red Zone"`
	IpCidr      string    `json:"cidr" example:"172.16.42.0/24"`
	IpCidrV6    string    `json:"cidr_v6" example:"200::/8"`
	HubZone     bool      `json:"hub_zone"`
}

func (o Organization) MarshalJSON() ([]byte, error) {
	org := OrganizationJSON{
		ID:          o.ID,
		OwnerID:     o.OwnerID,
		Name:        o.Name,
		Description: o.Description,
		IpCidr:      o.IpCidr,
		IpCidrV6:    o.IpCidrV6,
		HubZone:     o.HubZone,
	}
	return json.Marshal(org)
}

func (z *Organization) BeforeCreate(tx *gorm.DB) error {
	if z.Devices == nil {
		z.Devices = make([]*Device, 0)
	}
	if z.Users == nil {
		z.Users = make([]*User, 0)
	}
	return z.Base.BeforeCreate(tx)
}

type AddOrganization struct {
	Name        string `json:"name" example:"zone-red"`
	Description string `json:"description" example:"The Red Zone"`
	IpCidr      string `json:"cidr" example:"172.16.42.0/24"`
	IpCidrV6    string `json:"cidr_v6" example:"0200::/8"`
	HubZone     bool   `json:"hub_zone"`
}
