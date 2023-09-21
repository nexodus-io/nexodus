package migration_20230921_0000

import (
	"github.com/go-gormigrate/gormigrate/v2"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
	"time"
)

type Device struct {
	Online   bool
	OnlineAt *time.Time
}

func Migrate() *gormigrate.Migration {
	migrationId := "20230921-0000"
	return CreateMigrationFromActions(migrationId,
		AddTableColumnsAction(&Device{}),
	)
}
