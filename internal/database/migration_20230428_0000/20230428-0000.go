package migration_20230428_0000

import (
	"encoding/json"
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database/migrations"
	"time"
)

// Base contains common columns for all tables.
type Base struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	CreatedAt time.Time  `json:"-"`
	UpdatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `sql:"index" json:"-"`
}

// Device adds security groups to this table
type Device struct {
	SecurityGroupIds uuid.UUID `json:"security_group_ids,omitempty"`
}

// // Organization adds security groups to this table
type Organization struct {
	SecurityGroupIds uuid.UUID `json:"security_group_ids,omitempty"`
}

// // User adds security groups to this table
type User struct {
	SecurityGroupIds uuid.UUID `json:"security_group_ids,omitempty"`
}

// SecurityGroup represents a security group containing security rules and a group owner
type SecurityGroup struct {
	Base
	GroupName        string          `json:"group_name"`
	GroupDescription string          `json:"group_description"`
	OrganizationID   uuid.UUID       `json:"org_id"`
	InboundRules     json.RawMessage `json:"inbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
	OutboundRules    json.RawMessage `json:"outbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
}

func Migrate() *gormigrate.Migration {
	migrationId := "20230428-0000"
	return migrations.CreateMigrationFromActions(migrationId,
		migrations.CreateTableAction(&SecurityGroup{}),
		migrations.AddTableColumnsAction(&User{}),
		migrations.AddTableColumnsAction(&Device{}),
		migrations.AddTableColumnsAction(&Organization{}),
	)
}
