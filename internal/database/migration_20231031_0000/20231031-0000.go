package migration_20231031_0000

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"github.com/lib/pq"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
	"gorm.io/gorm"
)

type Base struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index" `
}

type User struct {
	Base
	Organizations   []*Organization `gorm:"many2many:user_organizations" `
	UserName        string          `gorm:"index"`
	IdpID           string
	Invitations     []*Invitation
	SecurityGroupId uuid.UUID
}
type Organization struct {
	Base
	OwnerID         uuid.UUID `gorm:"index"`
	Name            string
	Description     string
	SecurityGroupId uuid.UUID

	Users       []*User `gorm:"many2many:user_organizations;"`
	Invitations []*Invitation
}

type VPC struct {
	Base
	OrganizationID uuid.UUID `gorm:"index"`
	Description    string
	PrivateCidr    bool
	Ipv4Cidr       string
	Ipv6Cidr       string
}

type Device struct {
	Base
	OwnerID                  uuid.UUID `gorm:"index"`
	VpcID                    uuid.UUID `gorm:"index"`
	OrganizationID           uuid.UUID `gorm:"index"`
	PublicKey                string
	AllowedIPs               pq.StringArray `gorm:"type:text[]" `
	TunnelIPsV4              []TunnelIP     `gorm:"type:JSONB; serializer:json"`
	TunnelIPsV6              []TunnelIP     `gorm:"type:JSONB; serializer:json"`
	AdvertiseCidrs           pq.StringArray `gorm:"type:text[]" `
	Relay                    bool
	EndpointLocalAddressIPv4 string
	SymmetricNat             bool
	Hostname                 string `gorm:"index"`
	Os                       string
	Endpoints                []Endpoint `gorm:"type:JSONB; serializer:json"`
	Revision                 uint64     `gorm:"type:bigserial;index:"`
	SecurityGroupId          uuid.UUID
	Online                   bool
	OnlineAt                 *time.Time
	RegistrationTokenID      uuid.UUID
	BearerToken              string `gorm:"index"`
}

type TunnelIP struct {
	Address string
	CIDR    string
}

type Endpoint struct {
	Source   string
	Address  string
	Distance int
}

type DeviceMetadata struct {
	DeviceID  uuid.UUID      `gorm:"type:uuid;primary_key"`
	Key       string         `gorm:"primary_key"`
	Value     interface{}    `gorm:"type:JSONB; serializer:json"`
	Revision  uint64         `gorm:"type:bigserial;index:"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Invitation struct {
	Base
	UserID         uuid.UUID `gorm:"index"`
	OrganizationID uuid.UUID `gorm:"index"`
	Expiry         time.Time
}

type RegistrationToken struct {
	Base
	OwnerID        uuid.UUID `gorm:"index"`
	VpcID          uuid.UUID `gorm:"index"`
	OrganizationID uuid.UUID `gorm:"index"` // OrganizationID is denormalized from the VPC record for performance
	BearerToken    string    `gorm:"index"`
	Description    string
	DeviceId       *uuid.UUID
	Expiration     *time.Time
}

type SecurityGroup struct {
	Base
	GroupName        string
	GroupDescription string
	OrganizationId   uuid.UUID      `gorm:"index"`
	InboundRules     []SecurityRule `gorm:"type:JSONB; serializer:json"`
	OutboundRules    []SecurityRule `gorm:"type:JSONB; serializer:json"`
	Revision         uint64         `gorm:"type:bigserial;index:"`
}

type SecurityRule struct {
	IpProtocol string
	FromPort   int64
	ToPort     int64
	IpRanges   []string
}

func Migrate() *gormigrate.Migration {
	migrationId := "20231031-0000"
	return CreateMigrationFromActions(migrationId,

		ExecAction(`DROP TABLE IF EXISTS registration_tokens`, ""),
		ExecAction(`DROP TABLE IF EXISTS device_metadata`, ""),
		ExecAction(`DROP TABLE IF EXISTS security_groups`, ""),
		ExecAction(`DROP TABLE IF EXISTS devices`, ""),
		ExecAction(`DROP TABLE IF EXISTS invitations`, ""),
		ExecAction(`DROP TABLE IF EXISTS user_organizations`, ""),
		ExecAction(`DROP TABLE IF EXISTS organizations`, ""),
		ExecAction(`DROP TABLE IF EXISTS users`, ""),

		CreateTableAction(&User{}),
		CreateTableAction(&Organization{}),
		CreateTableAction(&Invitation{}),
		CreateTableAction(&VPC{}),
		CreateTableAction(&Device{}),
		ExecActionIf(`
			CREATE OR REPLACE FUNCTION devices_revision_trigger() RETURNS TRIGGER LANGUAGE plpgsql AS '
			BEGIN
			NEW.revision := nextval(''devices_revision_seq'');
			RETURN NEW;
			END;'
		`, `
			DROP FUNCTION IF EXISTS devices_revision_trigger
		`, NotOnSqlLite),
		ExecActionIf(`
			CREATE OR REPLACE TRIGGER devices_revision_trigger BEFORE INSERT OR UPDATE ON devices
			FOR EACH ROW EXECUTE PROCEDURE devices_revision_trigger();
		`, `
			DROP TRIGGER IF EXISTS devices_revision_trigger ON devices
		`, NotOnSqlLite),

		CreateTableAction(&SecurityGroup{}),
		ExecActionIf(`
			CREATE OR REPLACE FUNCTION security_groups_revision_trigger() RETURNS TRIGGER LANGUAGE plpgsql AS '
			BEGIN
			NEW.revision := nextval(''security_groups_revision_seq'');
			RETURN NEW;
			END;'
		`, `
			DROP FUNCTION IF EXISTS security_groups_revision_trigger
		`, NotOnSqlLite),
		ExecActionIf(`
			CREATE OR REPLACE TRIGGER security_groups_revision_trigger BEFORE INSERT OR UPDATE ON security_groups
			FOR EACH ROW EXECUTE PROCEDURE security_groups_revision_trigger();
		`, `
			DROP TRIGGER IF EXISTS security_groups_revision_trigger ON security_groups
		`, NotOnSqlLite),

		CreateTableAction(&DeviceMetadata{}),
		ExecActionIf(`
			CREATE OR REPLACE FUNCTION device_metadata_revision_trigger() RETURNS TRIGGER LANGUAGE plpgsql AS '
			BEGIN
			NEW.revision := nextval(''device_metadata_revision_seq'');
			RETURN NEW;
			END;'
		`, `
			DROP FUNCTION IF EXISTS device_metadata_revision_trigger
		`, NotOnSqlLite),
		ExecActionIf(`
			CREATE OR REPLACE TRIGGER device_metadata_revision_trigger BEFORE INSERT OR UPDATE ON device_metadata
			FOR EACH ROW EXECUTE PROCEDURE device_metadata_revision_trigger();
		`, `
			DROP TRIGGER IF EXISTS device_metadata_revision_trigger ON device_metadata
		`, NotOnSqlLite),

		CreateTableAction(&RegistrationToken{}),

		// manually create the unique indexes for now so that migrations work on cockroach see issue:
		// https://github.com/go-gorm/gorm/issues/5752
		ExecAction(
			`CREATE UNIQUE INDEX IF NOT EXISTS "idx_organizations_name" ON "organizations" ("name")`,
			`DROP INDEX IF EXISTS idx_organizations_name`,
		),
		ExecAction(
			`CREATE UNIQUE INDEX IF NOT EXISTS "idx_devices_public_key" ON "devices" ("public_key")`,
			`DROP INDEX IF EXISTS idx_devices_public_key`,
		),
		ExecAction(
			`CREATE UNIQUE INDEX IF NOT EXISTS "idx_devices_bearer_token" ON "devices" ("bearer_token")`,
			`DROP INDEX IF EXISTS idx_devices_bearer_token`,
		),
		ExecAction(
			`CREATE UNIQUE INDEX IF NOT EXISTS "idx_registration_tokens_bearer_token" ON "registration_tokens" ("bearer_token")`,
			`DROP INDEX IF EXISTS idx_registration_tokens_bearer_token`,
		),
		ExecAction(
			`CREATE UNIQUE INDEX IF NOT EXISTS "idx_users_idp_id" ON "users" ("idp_id")`,
			`DROP INDEX IF EXISTS idx_users_idp_id`,
		),
	)
}
