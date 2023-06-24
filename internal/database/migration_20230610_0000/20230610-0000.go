package migration_20230610_0000

import (
	"encoding/json"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type DeviceMetadata struct {
	DeviceID  uuid.UUID       `gorm:"type:uuid;primary_key"`
	Key       string          `gorm:"primary_key"`
	Value     json.RawMessage `gorm:"type:JSONB;serializer:json"`
	Revision  uint64          `gorm:"type:BIGSERIAL;index:"`
	DeletedAt gorm.DeletedAt  `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func Migrate() *gormigrate.Migration {
	migrationId := "20230610-0000"
	return CreateMigrationFromActions(migrationId,
		CreateTableAction(&DeviceMetadata{}),

		ExecActionIf(`
			CREATE OR REPLACE FUNCTION device_metadata_revision_trigger() RETURNS TRIGGER LANGUAGE plpgsql AS '
			BEGIN
			NEW.revision := nextval(''device_metadata_revision_seq'');
			RETURN NEW;
			END;'
		`, `
			DROP FUNCTION IF EXISTS device_metadata_revision_trigger
		`, NotOnSqlLite),
		ExecActionIf(`
			CREATE OR REPLACE TRIGGER device_metadata_revision_trigger BEFORE INSERT OR UPDATE ON device_metadata
			FOR EACH ROW EXECUTE PROCEDURE device_metadata_revision_trigger();
		`, `
			DROP TRIGGER IF EXISTS device_metadata_revision_trigger ON device_metadata
		`, NotOnSqlLite),
	)
}
