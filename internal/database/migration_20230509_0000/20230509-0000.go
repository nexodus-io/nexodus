package migration_20230509_0000

import (
	"github.com/go-gormigrate/gormigrate/v2"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type Organization struct {
	PrivateCidr bool `json:"private_cidr"`
}

func Migrate() *gormigrate.Migration {
	migrationId := "20230509-0000"
	return CreateMigrationFromActions(migrationId,
		AddTableColumnsAction(Organization{}),
	)
}
