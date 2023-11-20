package migration_20231106_0000

import (
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type SecurityGroup struct {
	GroupName string
}

type Invitation struct {
}

type RegKey struct {
}

func init() {
	migrationId := "20231106-0000"
	CreateMigrationFromActions(migrationId,
		RenameTableColumnAction(&Invitation{}, "expiry", "expires_at"),
		RenameTableColumnAction(&RegKey{}, "expiration", "expires_at"),
		RenameTableColumnAction(&SecurityGroup{}, "group_description", "description"),
		DropTableColumnAction(&SecurityGroup{}, "group_name"),
	)
}
