package models

import (
	"encoding/json"
	"github.com/google/uuid"
)

// DeviceMetadataInstance represents a key-value pair of device metadata in the database
type DeviceMetadataInstance struct {
	DeviceID uuid.UUID       `json:"device_id" gorm:"type:uuid;primary_key"`
	Key      string          `json:"key"       gorm:"primary_key"`
	Value    json.RawMessage `json:"value"     gorm:"type:JSONB; serializer:json"`
	Revision uint64          `json:"revision"  gorm:"type:bigserial;index:"`
}

// DeviceMetadata represents all device metadata in the API
type DeviceMetadata struct {
	DeviceID string                         `json:"device_id"`
	Metadata map[string]DeviceMetadataValue `json:"metadata"`
}

// DeviceMetadataValue represents a device metadata value for a specific key in the API
type DeviceMetadataValue struct {
	Value    interface{} `json:"value"`
	Revision uint64      `json:"revision"`
}
