package migration_20230412_0000

import (
	"github.com/go-gormigrate/gormigrate/v2"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type Device struct {
	Revision uint64 `gorm:"type:bigserial;index:"`
}

func Migrate() *gormigrate.Migration {
	migrationId := "20230412-0000"
	return CreateMigrationFromActions(migrationId,
		AddTableColumnsAction(&Device{}),
		ExecActionIf(`
			CREATE OR REPLACE FUNCTION devices_revision_trigger() RETURNS TRIGGER LANGUAGE plpgsql AS '
			BEGIN
			NEW.revision := nextval(''devices_revision_seq'');
			RETURN NEW;
			END;'
		`, `
			DROP FUNCTION IF EXISTS devices_revision_trigger
		`, NotOnSqlLite),
		ExecActionIf(`
			CREATE OR REPLACE TRIGGER devices_revision_trigger BEFORE INSERT OR UPDATE ON devices
			FOR EACH ROW EXECUTE PROCEDURE devices_revision_trigger();
		`, `
			DROP TRIGGER IF EXISTS devices_revision_trigger ON devices
		`, NotOnSqlLite),
	)
}
