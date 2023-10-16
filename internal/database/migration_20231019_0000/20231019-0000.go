package migration_20231019_0000

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
	"time"
)

// RegistrationToken is used to register devices without an interactive login.
type RegistrationToken struct {
	DeviceId   *uuid.UUID
	Expiration *time.Time
}

func Migrate() *gormigrate.Migration {
	migrationId := "20231019-0000"
	return CreateMigrationFromActions(migrationId,
		AddTableColumnsAction(&RegistrationToken{}),
		ExecAction(
			`CREATE UNIQUE INDEX IF NOT EXISTS "registration_tokens_bearer_token" ON "registration_tokens" ("bearer_token")`,
			`DROP INDEX registration_tokens_bearer_token`,
		),
		ExecAction(
			`CREATE UNIQUE INDEX IF NOT EXISTS "devices_bearer_token" ON "devices" ("bearer_token")`,
			`DROP INDEX devices_bearer_token`,
		),
	)
}
