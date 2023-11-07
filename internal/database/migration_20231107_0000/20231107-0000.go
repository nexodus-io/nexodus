package migration_20231107_0000

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type Organization struct {
}

type VPC struct {
	SecurityGroupId uuid.UUID
}

type SecurityGroup struct {
}

func Migrate20231107() *gormigrate.Migration {
	migrationId := "20231107-0000"
	return CreateMigrationFromActions(migrationId,
		RenameTableColumnAction(&SecurityGroup{}, "organization_id", "vpc_id"),
		// TODO
		// This is destructive, as it's not copying the default sec group from the org to the VPC.
		// We do not plan to deploy anything prior to this to prod, so it's fine. We should probably
		// just flatten the migrations one more time before we promote to prod.
		DropTableColumnAction(&Organization{}, "security_group_id"),
		AddTableColumnAction(&VPC{}, "security_group_id"),
	)
}
