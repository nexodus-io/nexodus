package models

import (
	"gorm.io/gorm"
)

// Organization contains Users and VPCs
type Organization struct {
	Base
	Name        string `json:"name" gorm:"uniqueIndex" sql:"index" example:"zone-red"`
	Description string `json:"description" example:"Team A"`

	Users       []*User       `json:"-" gorm:"many2many:user_organizations;"`
	Invitations []*Invitation `json:"-"`
}

func (z *Organization) BeforeCreate(tx *gorm.DB) error {
	if z.Users == nil {
		z.Users = make([]*User, 0)
	}
	return z.Base.BeforeCreate(tx)
}

type AddOrganization struct {
	Name        string `json:"name" example:"zone-red"`
	Description string `json:"description" example:"The Red Zone"`
}
