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
	ID            string `gorm:"primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time `sql:"index" json:"-"`
	Devices       []*Device
	Organizations []*Organization `gorm:"many2many:user_organizations"`
	UserName      string
	Invitations   []*Invitation
}

type UserJSON struct {
	ID            string        `json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	Devices       []uuid.UUID   `json:"devices" example:"4902c991-3dd1-49a6-9f26-d82496c80aff"`
	Organizations []uuid.UUID   `json:"organizations" example:"94deb404-c4eb-4097-b59d-76b024ff7867"`
	UserName      string        `json:"username" example:"admin"`
	Invitations   []*Invitation `json:"invitations"`
}

func (u User) MarshalJSON() ([]byte, error) {
	user := UserJSON{
		ID:            u.ID,
		Devices:       make([]uuid.UUID, 0),
		Organizations: make([]uuid.UUID, 0),
		UserName:      u.UserName,
		Invitations:   make([]*Invitation, 0),
	}
	for _, device := range u.Devices {
		user.Devices = append(user.Devices, device.ID)
	}
	for _, org := range u.Organizations {
		user.Organizations = append(user.Organizations, org.ID)
	}
	user.Invitations = append(user.Invitations, u.Invitations...)
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
