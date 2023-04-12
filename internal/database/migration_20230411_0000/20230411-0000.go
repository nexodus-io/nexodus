package migration_20230411_0000

import (
	"github.com/go-gormigrate/gormigrate/v2"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

func Migrate() *gormigrate.Migration {
	migrationId := "20230411-0000"
	return CreateMigrationFromActions(migrationId,
		// Index stuff that we include in where clauses.
		ExecAction(
			`CREATE INDEX organizations_owner_id ON organizations (owner_id)`,
			`DROP INDEX IF EXISTS organizations_owner_id`,
		),
		ExecAction(
			`CREATE INDEX devices_user_id ON devices (user_id)`,
			`DROP INDEX IF EXISTS devices_user_id`,
		),
		ExecAction(
			`CREATE INDEX devices_organization_id ON devices (organization_id)`,
			`DROP INDEX IF EXISTS devices_organization_id`,
		),

		// Create indexes for the fields that we sort on.
		ExecAction(
			`CREATE INDEX users_user_name ON users (user_name)`,
			`DROP INDEX IF EXISTS users_user_name`,
		),
		ExecAction(
			`CREATE INDEX devices_hostname ON devices (hostname)`,
			`DROP INDEX IF EXISTS devices_hostname`,
		),

		// Create indexes to make soft deletes more efficient.
		ExecAction(
			`CREATE INDEX users_deleted_at ON users (deleted_at)`,
			`DROP INDEX IF EXISTS users_deleted_at`,
		),
		ExecAction(
			`CREATE INDEX organizations_deleted_at ON organizations (deleted_at)`,
			`DROP INDEX IF EXISTS organizations_deleted_at`,
		),
		ExecAction(
			`CREATE INDEX devices_deleted_at ON devices (deleted_at)`,
			`DROP INDEX IF EXISTS devices_deleted_at`,
		),
		ExecAction(
			`CREATE INDEX invitations_deleted_at ON invitations (deleted_at)`,
			`DROP INDEX IF EXISTS invitations_deleted_at`,
		),
	)
}
