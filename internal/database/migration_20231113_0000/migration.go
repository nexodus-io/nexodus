package migration_20231113_0000

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type RegKey struct {
	SecurityGroupId *uuid.UUID
}

func New() *gormigrate.Migration {
	migrationId := "20231113-0000"
	return CreateMigrationFromActions(migrationId,
		AddTableColumnsAction(&RegKey{}),
	)
}
