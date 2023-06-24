package migration_20230610_0000

import (
	"encoding/json"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type DeviceMetadataInstance struct {
	DeviceID  uuid.UUID       `json:"device_id" gorm:"type:uuid;primary_key"`
	Key       string          `json:"key"       gorm:"primary_key"`
	Value     json.RawMessage `json:"value"     gorm:"type:JSONB; serializer:json"`
	Revision  uint64          `json:"revision"  gorm:"type:bigserial;index:"`
	DeletedAt gorm.DeletedAt  `json:"-"         gorm:"index"`
	CreatedAt time.Time       `json:"-"`
	UpdatedAt time.Time       `json:"-"`
}

func Migrate() *gormigrate.Migration {
	migrationId := "20230610-0000"
	return CreateMigrationFromActions(migrationId,
		CreateTableAction(&DeviceMetadataInstance{}),

		ExecActionIf(`
			CREATE OR REPLACE FUNCTION device_metadata_instances_revision_trigger() RETURNS TRIGGER LANGUAGE plpgsql AS '
			BEGIN
			NEW.revision := nextval(''device_metadata_instances_revision_seq'');
			RETURN NEW;
			END;'
		`, `
			DROP FUNCTION IF EXISTS device_metadata_instances_revision_trigger
		`, NotOnSqlLite),
		ExecActionIf(`
			CREATE OR REPLACE TRIGGER device_metadata_instances_revision_trigger BEFORE INSERT OR UPDATE ON device_metadata_instances
			FOR EACH ROW EXECUTE PROCEDURE device_metadata_instances_revision_trigger();
		`, `
			DROP TRIGGER IF EXISTS device_metadata_instances_revision_trigger ON device_metadata_instances
		`, NotOnSqlLite),
	)
}
