package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User is the a person who uses Nexodus
type User struct {
	// Since the ID comes from the IDP, we have no control over the format...
	ID               string `gorm:"primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        gorm.DeletedAt  `gorm:"index" json:"-"`
	Devices          []*Device       `json:"-"`
	Organizations    []*Organization `gorm:"many2many:user_organizations" json:"-"`
	UserName         string
	Invitations      []*Invitation `json:"-"`
	SecurityGroupIds uuid.UUID     `json:"security_group_ids"`
}

type UserJSON struct {
	ID       string `json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	UserName string `json:"username" example:"admin"`
}

func (u User) MarshalJSON() ([]byte, error) {
	user := UserJSON{
		ID:       u.ID,
		UserName: u.UserName,
	}
	return json.Marshal(user)
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.Devices == nil {
		u.Devices = make([]*Device, 0)
	}
	if u.Organizations == nil {
		u.Organizations = make([]*Organization, 0)
	}
	if u.Invitations == nil {
		u.Invitations = make([]*Invitation, 0)
	}
	return nil
}
