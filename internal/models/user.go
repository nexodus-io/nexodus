package models

import (
	"gorm.io/gorm"
)

// User is the a person who uses Nexodus
type User struct {
	Base
	IdpID         string          `json:"-"` // Comes from the IDP
	Organizations []*Organization `gorm:"many2many:user_organizations" json:"-"`
	FullName      string          `json:"full_name"`
	Picture       string          `json:"picture"`
	UserName      string          `json:"username"`
	Invitations   []*Invitation   `json:"-"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.Organizations == nil {
		u.Organizations = make([]*Organization, 0)
	}
	if u.Invitations == nil {
		u.Invitations = make([]*Invitation, 0)
	}
	return nil
}
