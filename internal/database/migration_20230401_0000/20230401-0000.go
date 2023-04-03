package migration_20230401_0000

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/nexodus-io/nexodus/internal/database/migrations"
)

// Base contains common columns for all tables.
type Base struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	CreatedAt time.Time  `json:"-"`
	UpdatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `sql:"index" json:"-"`
}

// Device is a unique, end-user device.
// Devices belong to one User and may be onboarded into an organization
type Device struct {
	Base
	UserID                   string
	OrganizationID           uuid.UUID
	PublicKey                string
	LocalIP                  string
	LocalIpV6                string
	AllowedIPs               pq.StringArray `gorm:"type:text[]"`
	TunnelIP                 string
	TunnelIpV6               string
	ChildPrefix              pq.StringArray `gorm:"type:text[]"`
	Relay                    bool
	Discovery                bool
	OrganizationPrefix       string
	OrganizationPrefixV6     string
	ReflexiveIPv4            string
	EndpointLocalAddressIPv4 string
	SymmetricNat             bool
	Hostname                 string
}

// Organization contains Users and their Devices
type Organization struct {
	Base
	Users       []*User `gorm:"many2many:user_organizations;"`
	Devices     []*Device
	Name        string
	Description string
	IpCidr      string
	IpCidrV6    string
	HubZone     bool
}

// User is a person who uses Nexodus
type User struct {
	// Since the ID comes from the IDP, we have no control over the format...
	ID            string `gorm:"primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time `sql:"index" json:"-"`
	Devices       []*Device
	Organizations []*Organization `gorm:"many2many:user_organizations"`
	UserName      string
}

func Migrate() *gormigrate.Migration {
	migrationId := "20230401-0000"
	return migrations.CreateMigrationFromActions(migrationId,
		// Add IPv6 fields to the Device and Organization tables
		migrations.AddTableColumnAction(&Device{}, "organization_prefix_v6"),
		migrations.AddTableColumnAction(&Device{}, "local_ip_v6"),
		migrations.AddTableColumnAction(&Device{}, "tunnel_ip_v6"),
		migrations.AddTableColumnAction(&Organization{}, "ip_cidr_v6"),
	)
}
