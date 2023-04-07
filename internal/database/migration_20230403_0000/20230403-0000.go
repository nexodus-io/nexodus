package migration_20230403_0000

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/nexodus-io/nexodus/internal/database/migrations"
)

type Device struct {
	OS string
}

func Migrate() *gormigrate.Migration {
	migrationId := "20230403-0000"
	return migrations.CreateMigrationFromActions(migrationId,
		// Add OS field to the Device and Organization tables
		migrations.AddTableColumnAction(&Device{}, "os"),
	)
}
