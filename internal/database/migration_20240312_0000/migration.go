package migration_20240312_0000

import (
	"github.com/google/uuid"
	//"github.com/lib/pq"
	//"github.com/nexodus-io/nexodus/internal/database/datatype"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
	//"github.com/nexodus-io/nexodus/internal/models"
	//"gorm.io/gorm"
)

type Status struct {
	UserId      uuid.UUID `gorm:"index"`
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
