package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Zone is a collection of devices that are connected together.
type Zone struct {
	Base
	Peers       []*Peer     `json:"-"`
	PeerList    []uuid.UUID `gorm:"-" json:"peers" example:"4902c991-3dd1-49a6-9f26-d82496c80aff"`
	Name        string      `gorm:"uniqueIndex" json:"name" example:"zone-red"`
	Description string      `json:"description" example:"The Red Zone"`
	IpCidr      string      `json:"cidr" example:"172.16.42.0/24"`
	HubZone     bool        `json:"hub_zone"`
}

func (z *Zone) BeforeCreate(tx *gorm.DB) error {
	if z.Peers == nil {
		z.Peers = make([]*Peer, 0)
	}
	if z.PeerList == nil {
		z.PeerList = make([]uuid.UUID, 0)
	}
	return z.Base.BeforeCreate(tx)
}

type AddZone struct {
	Name        string `json:"name" example:"zone-red"`
	Description string `json:"description" example:"The Red Zone"`
	IpCidr      string `json:"cidr" example:"172.16.42.0/24"`
	HubZone     bool   `json:"hub_zone"`
}
