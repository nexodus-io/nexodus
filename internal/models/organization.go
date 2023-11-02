package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Organization contains Users and VPCs
type Organization struct {
	Base
	OwnerID         uuid.UUID `json:"owner_id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	Name            string    `json:"name" gorm:"uniqueIndex" sql:"index" example:"zone-red"`
	Description     string    `json:"description" example:"Team A"`
	SecurityGroupId uuid.UUID `json:"security_group_id"`

	Users       []*User       `json:"-" gorm:"many2many:user_organizations;"`
	Invitations []*Invitation `json:"-"`
	VPCs        []*VPC        `json:"-"`
}

func (z *Organization) BeforeCreate(tx *gorm.DB) error {
	if z.Users == nil {
		z.Users = make([]*User, 0)
	}
	return z.Base.BeforeCreate(tx)
}

type AddOrganization struct {
	Name            string    `json:"name" example:"zone-red"`
	Description     string    `json:"description" example:"The Red Zone"`
	SecurityGroupId uuid.UUID `json:"security_group_id"`
}
