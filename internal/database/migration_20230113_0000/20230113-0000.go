package migration_20230113_0000

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/nexodus-io/nexodus/internal/database/migrations"
)

type Base struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	CreatedAt time.Time  `json:"-"`
	UpdatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `sql:"index" json:"-"`
}

type Peer struct {
	Base
	DeviceID                 uuid.UUID      `json:"device_id" example:"fde38e78-a4af-4f44-8f5a-d84ef1846a85"`
	ZoneID                   uuid.UUID      `json:"zone_id" example:"2b655c5b-cfdd-4550-b7f0-a36a590fc97a"`
	LocalIP                  string         `json:"endpoint_ip" example:"10.1.1.1"`
	AllowedIPs               pq.StringArray `json:"allowed_ips" gorm:"type:text[]"`
	TunnelIP                 string         `json:"node_address" example:"1.2.3.4"`
	ChildPrefix              string         `json:"child_prefix" example:"172.16.42.0/24"`
	HubRouter                bool           `json:"hub_router"`
	HubZone                  bool           `json:"hub_zone"`
	ZonePrefix               string         `json:"zone_prefix" example:"10.1.1.0/24"`
	ReflexiveIPv4            string         `json:"reflexive_ip4"`
	EndpointLocalAddressIPv4 string         `json:"endpoint_local_address_ip4" example:"1.2.3.4"`
	SymmetricNat             bool           `json:"symmetric_nat"`
}

type Zone struct {
	Base
	Peers       []*Peer     `json:"-"`
	PeerList    []uuid.UUID `gorm:"-" json:"peers" example:"4902c991-3dd1-49a6-9f26-d82496c80aff"`
	Name        string      `gorm:"" json:"name" example:"zone-red"`
	Description string      `json:"description" example:"The Red Zone"`
	IpCidr      string      `json:"cidr" example:"172.16.42.0/24"`
	HubZone     bool        `json:"hub_zone"`
}

type Device struct {
	Base
	UserID    string      `json:"user_id" example:"694aa002-5d19-495e-980b-3d8fd508ea10"`
	PublicKey string      `gorm:"" json:"public_key"`
	Peers     []*Peer     `json:"-"`
	PeerList  []uuid.UUID `gorm:"-" json:"peers" example:"97d5214a-8c51-4772-b492-53de034740c5"`
	Hostname  string      `json:"hostname" example:"myhost"`
}

type User struct {
	// Since the ID comes from the IDP, we have no control over the format...
	ID         string      `gorm:"primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	CreatedAt  time.Time   `json:"-"`
	UpdatedAt  time.Time   `json:"-"`
	DeletedAt  *time.Time  `sql:"index" json:"-"`
	Devices    []*Device   `json:"-"`
	DeviceList []uuid.UUID `gorm:"-" json:"devices" example:"4902c991-3dd1-49a6-9f26-d82496c80aff"`
	ZoneID     uuid.UUID   `json:"zone_id" example:"94deb404-c4eb-4097-b59d-76b024ff7867"`
	UserName   string      `json:"username" example:"admin"`
}

// Migrations rules:
//
//  1. IDs are numerical timestamps that must sort ascending.
//     Use YYYYMMDD-HHMM w/ 24 hour time for format
//     Example: August 21 2018 at 2:54pm would be 20180821-1454.
//
//  2. Include models inline with migrations to see the evolution of the object over time.
//     Using our internal type models directly in the first migration would fail in future clean
//     installations.
//
//  3. Migrations must be backwards compatible. There are no new required fields allowed.
//
// 4. Create one function in a separate file that returns your Migration.

func Migrate() *gormigrate.Migration {
	migrationId := "20230113-0000"

	return migrations.CreateMigrationFromActions(migrationId,
		migrations.CreateTableAction(&User{}),
		migrations.CreateTableAction(&Peer{}),
		migrations.CreateTableAction(&Zone{}),
		// manually create the unique indexes for now so that migrations work on cockroach see issue:
		// https://github.com/go-gorm/gorm/issues/5752
		migrations.ExecAction(
			`CREATE UNIQUE INDEX IF NOT EXISTS "idx_zones_name" ON "zones" ("name")`,
			`DROP INDEX zones@idx_zones_name`,
		),
		migrations.CreateTableAction(&Device{}),
		migrations.ExecAction(
			`CREATE UNIQUE INDEX IF NOT EXISTS "idx_devices_public_key" ON "devices" ("public_key")`,
			`DROP INDEX devices@idx_devices_public_key`,
		),
	)
}
