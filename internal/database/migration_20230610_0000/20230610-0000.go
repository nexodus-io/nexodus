package migration_20230610_0000

import (
	"encoding/json"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/gofrs/uuid"
	"github.com/nexodus-io/nexodus/internal/database/migrations"
	"github.com/nexodus-io/nexodus/internal/models"
)

type DeviceMetadataInstance struct {
	models.Base
	DeviceID uuid.UUID       `json:"device_id"`
	Key      string          `json:"key"`
	Value    json.RawMessage `json:"value,omitempty" gorm:"type:JSONB; serializer:json"`
}

func Migrate() *gormigrate.Migration {
	migrationId := "20230610-0000"
	return migrations.CreateMigrationFromActions(migrationId,
		migrations.CreateTableAction(&DeviceMetadataInstance{}),
	)
}
