package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
)

// gormigrate is a wrapper for gorm's migration functions that adds schema versioning and rollback capabilities.
// For help writing migration steps, see the gorm documentation on migrations: https://gorm.io/docs/migration.html
func New() *Migrations {
	return &Migrations{
		GormOptions: &gormigrate.Options{
			TableName:      "apiserver_migrations",
			IDColumnName:   "id",
			IDColumnSize:   40,
			UseTransaction: false,
		},
		Migrations: []*gormigrate.Migration{
			addApexTables(),
		},
	}
}
