package migration_20231113_0000

import (
	"github.com/google/uuid"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type RegKey struct {
	SecurityGroupId *uuid.UUID
}

func init() {
	migrationId := "20231113-0000"
	CreateMigrationFromActions(migrationId,
		AddTableColumnsAction(&RegKey{}),
	)
}
