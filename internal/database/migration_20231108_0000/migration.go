package migration_20231108_0000

import (
	"github.com/google/uuid"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type VPC struct {
	SecurityGroupId uuid.UUID
}

func init() {
	migrationId := "20231108-0000"
	CreateMigrationFromActions(migrationId,
		// TODO
		// This is destructive, as it's not copying the default sec group from the org to the VPC.
		// We do not plan to deploy anything prior to this to prod, so it's fine. We should probably
		// just flatten the migrations one more time before we promote to prod.
		DropTableColumnAction(&VPC{}, "security_group_id"),
	)
}
