package apexcontroller

import (
	"context"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/google/uuid"
	apiv1 "github.com/metal-stack/go-ipam/api/v1"
	"gorm.io/gorm"
)

// Base contains common columns for all tables.
type Base struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	CreatedAt time.Time  `json:"-"`
	UpdatedAt time.Time  `json:"-"`
	DeletedAt *time.Time `sql:"index" json:"-"`
}

// BeforeCreate populates the ID
func (base *Base) BeforeCreate(tx *gorm.DB) error {
	base.ID = uuid.New()
	return nil
}

// User is the owner of a device, and a member of one Zone
type User struct {
	Base
	Devices    []*Device   `json:"-"`
	DeviceList []uuid.UUID `gorm:"-" json:"devices" example:"4902c991-3dd1-49a6-9f26-d82496c80aff"`
	ZoneID     uuid.UUID   `json:"zone-id" example:"94deb404-c4eb-4097-b59d-76b024ff7867"`
}

// PatchUser is used to update a user
type PatchUser struct {
	ZoneID uuid.UUID `json:"zone-id" example:"3f51dda6-06d2-4724-bb73-f09ad3501bcc"`
}

// Device is a unique, end-user device.
type Device struct {
	Base
	UserID    uuid.UUID `json:"user-id"`
	PublicKey string    `gorm:"uniqueIndex" json:"public-key"`
}

// AddDevice is the information needed to add a new Device.
type AddDevice struct {
	PublicKey string `json:"public-key" example:"rZlVfefGshRxO+r9ethv2pODimKlUeP/bO/S47K3WUk="`
}

// Peer is an association between a Device and a Zone.
type Peer struct {
	Base
	DeviceID    uuid.UUID `json:"device-id" example:"fde38e78-a4af-4f44-8f5a-d84ef1846a85"`
	ZoneID      uuid.UUID `json:"zone-id" example:"2b655c5b-cfdd-4550-b7f0-a36a590fc97a"`
	EndpointIP  string    `json:"endpoint-ip" example:"10.1.1.1"`
	AllowedIPs  string    `json:"allowed-ips" example:"10.1.1.1"`
	NodeAddress string    `json:"node-address" example:"1.2.3.4"`
	ChildPrefix string    `json:"child-prefix" example:"172.16.42.0/24"`
	HubRouter   bool      `json:"hub-router"`
	HubZone     bool      `json:"hub-zone"`
	ZonePrefix  string    `json:"zone-prefix" example:"10.1.1.0/24"`
}

// AddPeer are the fields required to add a new Peer
type AddPeer struct {
	DeviceID    uuid.UUID `json:"device-id" example:"6a6090ad-fa47-4549-a144-02124757ab8f"`
	EndpointIP  string    `json:"endpoint-ip" example:"10.1.1.1"`
	AllowedIPs  string    `json:"allowed-ips" example:"10.1.1.1"`
	NodeAddress string    `json:"node-address" example:"1.2.3.4"`
	ChildPrefix string    `json:"child-prefix" example:"172.16.42.0/24"`
	HubRouter   bool      `json:"hub-router"`
	HubZone     bool      `json:"hub-zone"`
	ZonePrefix  string    `json:"zone-prefix" example:"10.1.1.0/24"`
}

// Zone is a collection of devices that are connected together.
type Zone struct {
	Base
	Peers       []*Peer     `json:"-"`
	PeerList    []uuid.UUID `gorm:"-" json:"peers" example:"4902c991-3dd1-49a6-9f26-d82496c80aff"`
	Name        string      `json:"name" example:"zone-red"`
	Description string      `json:"description" example:"The Red Zone"`
	IpCidr      string      `json:"cidr" example:"172.16.42.0/24"`
	HubZone     bool        `json:"hub-zone"`
}

type AddZone struct {
	Name        string `json:"name" example:"zone-red"`
	Description string `json:"description" example:"The Red Zone"`
	IpCidr      string `json:"cidr" example:"172.16.42.0/24"`
	HubZone     bool   `json:"hub-zone"`
}

// NewZone creates a new Zone and allocates the prefix using IPAM
func (ct *Controller) NewZone(id, name, description, cidr string, hubZone bool) (*Zone, error) {
	if _, err := ct.ipam.CreatePrefix(context.Background(), connect.NewRequest(&apiv1.CreatePrefixRequest{Cidr: cidr})); err != nil {
		return nil, err
	}
	zone := &Zone{
		Peers:       make([]*Peer, 0),
		Name:        name,
		Description: description,
		IpCidr:      cidr,
		HubZone:     hubZone,
	}
	res := ct.db.Create(zone)
	if res.Error != nil {
		return nil, res.Error
	}
	return zone, nil
}

type ApiError struct {
	Error string `json:"error" example:"something bad"`
}
