package migration_20230328_0000

import (
	"github.com/go-gormigrate/gormigrate/v2"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
	"gorm.io/gorm"
)

func Migrate() *gormigrate.Migration {
	migrationId := "20230328-0000"
	return CreateMigrationFromActions(migrationId,

		// assign an owner to the organizations..
		ExecActionIf(
			`
				UPDATE organizations
				SET owner_id=subquery.user_id
				FROM (SELECT user_id, organization_id FROM  user_organizations) AS subquery
				WHERE organizations.id=subquery.organization_id AND organizations.owner_id='';
				`,
			``, func(db *gorm.DB) bool {
				// this SQL does not work well against Sqlite.
				version := ""
				return db.Raw("SELECT sqlite_version()").Scan(&version).Error != nil
			}),
	)
}
