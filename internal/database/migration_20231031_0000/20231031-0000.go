package migration_20231031_0000

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"github.com/lib/pq"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
	"gorm.io/gorm"
	"time"
)

type Base struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index" `
}

type Endpoint struct {
	Source   string
	Address  string
	Distance int
}

type Device struct {
	Base
	UserID                   string    `gorm:"index"`
	OrganizationID           uuid.UUID `gorm:"index"`
	PublicKey                string
	AllowedIPs               pq.StringArray `gorm:"type:text[]" `
	TunnelIP                 string
	TunnelIpV6               string
	ChildPrefix              pq.StringArray `gorm:"type:text[]" `
	Relay                    bool
	Discovery                bool
	OrganizationPrefix       string
	OrganizationPrefixV6     string
	EndpointLocalAddressIPv4 string
	SymmetricNat             bool
	Hostname                 string `gorm:"index"`
	Os                       string
	Endpoints                []Endpoint `gorm:"type:JSONB; serializer:json"`
	Revision                 uint64     `gorm:"type:bigserial;index:"`
	SecurityGroupId          uuid.UUID
	Online                   bool
	OnlineAt                 *time.Time
	// the registration token id that created the device (if it was created with a registration token)
	RegistrationTokenID uuid.UUID
	// the token nexd should use to reconcile device state.
	BearerToken string
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
	UserID         string
	OrganizationID uuid.UUID
	Expiry         time.Time
}

type Organization struct {
	Base
	OwnerID         string  `gorm:"owner_id;index"`
	Users           []*User `gorm:"many2many:user_organizations;"`
	Devices         []*Device
	Name            string
	Description     string
	PrivateCidr     bool
	IpCidr          string
	IpCidrV6        string
	HubZone         bool
	Invitations     []*Invitation
	SecurityGroupId uuid.UUID
}

type RegistrationToken struct {
	Base
	UserID         string    `gorm:"index"`
	OrganizationID uuid.UUID `gorm:"index"`
	// BearerToken is the token the client should use to authenticate the device registration request.
	BearerToken string
	Description string
	DeviceId    *uuid.UUID
	Expiration  *time.Time
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

type User struct {
	// Since the ID comes from the IDP, we have no control over the format...
	ID              string `gorm:"primary_key"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index" `
	Devices         []*Device
	Organizations   []*Organization `gorm:"many2many:user_organizations" `
	UserName        string          `gorm:"index"`
	Invitations     []*Invitation
	SecurityGroupId uuid.UUID
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
			`DROP INDEX idx_devices_bearer_token`,
		),
		ExecAction(
			`CREATE UNIQUE INDEX IF NOT EXISTS "idx_registration_tokens_bearer_token" ON "registration_tokens" ("bearer_token")`,
			`DROP INDEX idx_registration_tokens_bearer_token`,
		),
	)
}
