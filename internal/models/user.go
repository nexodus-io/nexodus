package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User is the owner of a device, and a member of one Zone
type User struct {
	// Since the ID comes from the IDP, we have no control over the format...
	ID         string      `gorm:"primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	CreatedAt  time.Time   `json:"-"`
	UpdatedAt  time.Time   `json:"-"`
	DeletedAt  *time.Time  `sql:"index" json:"-"`
	Devices    []*Device   `json:"-"`
	DeviceList []uuid.UUID `gorm:"-" json:"devices" example:"4902c991-3dd1-49a6-9f26-d82496c80aff"`
	ZoneID     uuid.UUID   `json:"zone_id" example:"94deb404-c4eb-4097-b59d-76b024ff7867"`
	UserName   string      `json:"username" example:"admin"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.Devices == nil {
		u.Devices = make([]*Device, 0)
	}
	if u.DeviceList == nil {
		u.DeviceList = make([]uuid.UUID, 0)
	}
	return nil
}

// PatchUser is used to update a user
type PatchUser struct {
	ZoneID uuid.UUID `json:"zone_id" example:"3f51dda6-06d2-4724-bb73-f09ad3501bcc"`
}
