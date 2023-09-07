package migration_20230907_0000

import (
	"github.com/go-gormigrate/gormigrate/v2"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type SecurityGroup struct {
	Revision uint64 `gorm:"type:BIGSERIAL;index:"`
}

func Migrate() *gormigrate.Migration {
	migrationId := "20230907-0000"
	return CreateMigrationFromActions(migrationId,
		AddTableColumnsAction(&SecurityGroup{}),

		ExecActionIf(`
			CREATE OR REPLACE FUNCTION security_groups_revision_trigger() RETURNS TRIGGER LANGUAGE plpgsql AS '
			BEGIN
			NEW.revision := nextval(''security_groups_revision_seq'');
			RETURN NEW;
			END;'
		`, `
			DROP FUNCTION IF EXISTS security_groups_revision_trigger
		`, NotOnSqlLite),
		ExecActionIf(`
			CREATE OR REPLACE TRIGGER security_groups_revision_trigger BEFORE INSERT OR UPDATE ON security_groups
			FOR EACH ROW EXECUTE PROCEDURE security_groups_revision_trigger();
		`, `
			DROP TRIGGER IF EXISTS security_groups_revision_trigger ON security_groups
		`, NotOnSqlLite),
	)
}
