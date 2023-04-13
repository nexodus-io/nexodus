package migration_20230413_0000

import (
	"github.com/go-gormigrate/gormigrate/v2"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

func Migrate() *gormigrate.Migration {
	migrationId := "20230413-0000"
	// clear the DB...
	return CreateMigrationFromActions(migrationId,
		ExecAction(` DELETE FROM invitations WHERE 1=1;`, ``),
		ExecAction(` DELETE FROM devices WHERE 1=1;`, ``),
		ExecAction(` DELETE FROM user_organizations WHERE 1=1;`, ``),
		ExecAction(` DELETE FROM organizations WHERE 1=1;`, ``),
		ExecAction(` DELETE FROM users WHERE 1=1;`, ``),
	)
}
