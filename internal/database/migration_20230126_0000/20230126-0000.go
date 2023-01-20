package migration_20230126_0000

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/redhat-et/apex/internal/database/migration_20230113_0000"
	"github.com/redhat-et/apex/internal/database/migrations"
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
	AllowedIPs               pq.StringArray `gorm:"type:text[]"`
	TunnelIP                 string
	ChildPrefix              pq.StringArray `gorm:"type:text[]"`
	Relay                    bool
	OrganizationPrefix       string
	ReflexiveIPv4            string
	EndpointLocalAddressIPv4 string
	SymmetricNat             bool
	Hostname                 string
}

// Organization contains Users and their Devices
type Organization struct {
	Base
	Users       []*User `gorm:"many2many:user_organization;"`
	Devices     []*Device
	Name        string
	Description string
	IpCidr      string
	HubZone     bool
}

// User is the a person who uses Apex
type User struct {
	// Since the ID comes from the IDP, we have no control over the format...
	ID            string `gorm:"primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time `sql:"index" json:"-"`
	Devices       []*Device
	Organizations []*Organization `gorm:"many2many:user_organization"`
	UserName      string
}

func Migrate() *gormigrate.Migration {
	migrationId := "20230126-0000"
	return migrations.CreateMigrationFromActions(migrationId,
		// Peers table removed
		migrations.DropTableAction(&migration_20230113_0000.Peer{}),
		// Field from peers folded into Devices
		migrations.CreateTableAction(&Device{}),
		// Zones => Organizations
		migrations.RenameTableAction(&migration_20230113_0000.Zone{}, &Organization{}),
		migrations.CreateTableAction(&Organization{}),
	)
}
