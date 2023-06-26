package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

// DeviceMetadata represents a key-value pair of device metadata in the database
type DeviceMetadata struct {
	DeviceID  uuid.UUID      `json:"device_id" gorm:"type:uuid;primary_key"`
	Key       string         `json:"key"       gorm:"primary_key"`
	Value     interface{}    `json:"value"     gorm:"type:JSONB; serializer:json"`
	Revision  uint64         `json:"revision"  gorm:"type:bigserial;index:"`
	DeletedAt gorm.DeletedAt `json:"-"         gorm:"index"`
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
}
