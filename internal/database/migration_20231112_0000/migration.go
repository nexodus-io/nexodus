package migration_20231112_0000

import (
	"github.com/google/uuid"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type User struct {
	SecurityGroupId uuid.UUID
}

func init() {
	migrationId := "20231112-0000"
	CreateMigrationFromActions(migrationId,
		DropTableColumnAction(&User{}, "security_group_id"),
	)
}
