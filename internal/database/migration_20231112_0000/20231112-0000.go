package migration_20231112_0000

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type User struct {
	SecurityGroupId uuid.UUID
}

func New() *gormigrate.Migration {
	migrationId := "20231112-0000"
	return CreateMigrationFromActions(migrationId,
		DropTableColumnAction(&User{}, "security_group_id"),
	)
}
