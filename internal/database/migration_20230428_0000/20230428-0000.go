package migration_20230428_0000

import (
	"encoding/json"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database/migrations"
	"github.com/nexodus-io/nexodus/internal/models"
)

// Device adds security groups to this table
type Device struct {
	SecurityGroupId uuid.UUID `json:"security_group_id,omitempty"`
}

// Organization adds security groups to this table
type Organization struct {
	SecurityGroupId uuid.UUID `json:"security_group_id,omitempty"`
}

// User adds security groups to this table
type User struct {
	SecurityGroupId uuid.UUID `json:"security_group_id,omitempty"`
}

// SecurityGroup represents a security group containing security rules and a group owner
type SecurityGroup struct {
	models.Base
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
