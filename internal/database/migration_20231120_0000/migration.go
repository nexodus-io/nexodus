package migration_20231120_0000

import (
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type RegKey struct {
	Settings map[string]interface{} `json:"settings" gorm:"type:JSONB; serializer:json"` // Settings contains general settings for the device.
}

func init() {
	migrationId := "20231120-0000"
	CreateMigrationFromActions(migrationId,
		AddTableColumnsAction(&RegKey{}),
	)
}
