package migration_20240312_0000

import (

	. "github.com/nexodus-io/nexodus/internal/database/migrations"
	"github.com/google/uuid"


)

type Status struct {
	UserId		uuid.UUID	`gorm:"index"`
	WgIP        string
	IsReachable bool
	Hostname    string
	Latency     string
	Method      string
}

func init() {
	migrationId := "20240312_0000"

	CreateMigrationFromActions(migrationId,
		ExecAction(`DROP TABLE IF EXISTS status`, ""),
		CreateTableAction(&Status{}),
	)
}
