package migration_20230327_0000

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database/migrations"
)

// Base contains common columns for all tables.
type Base struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	CreatedAt time.Time  `json:"-"`
	UpdatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `sql:"index" json:"-"`
}

// Invitation is a request for a user to join an organization
type Invitation struct {
	Base
	UserID         string    `json:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Expiry         time.Time `json:"expiry"`
}

type Organization struct {
	OwnerID string `gorm:"owner_id;"`
}

func Migrate() *gormigrate.Migration {
	migrationId := "20230327-0000"
	return migrations.CreateMigrationFromActions(migrationId,
		migrations.CreateTableAction(&Invitation{}),
		migrations.AddTableColumnsAction(&Organization{}),
	)
}
