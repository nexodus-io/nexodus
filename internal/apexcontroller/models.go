package apexcontroller

import (
	"context"

	"github.com/bufbuild/connect-go"
	apiv1 "github.com/metal-stack/go-ipam/api/v1"
)

// User is the owner of a device, and a member of one Zone
type User struct {
	ID      string `gorm:"primaryKey"`
	Devices []*Device
	ZoneID  string
}

// Device is a unique, end-user device.
type Device struct {
	ID        string `gorm:"primaryKey"`
	UserID    string
	PublicKey string `gorm:"uniqueIndex"`
}

// Peer is an association between a Device and a Zone.
type Peer struct {
	ID          string `json:"id"`
	DeviceID    string `json:"device-id"`
	ZoneID      string `json:"zone-id"`
	EndpointIP  string `json:"endpoint-ip"`
	AllowedIPs  string `json:"allowed-ips"`
	NodeAddress string `json:"node-address"`
	ChildPrefix string `json:"child-prefix"`
	HubRouter   bool   `json:"hub-router"`
	HubZone     bool   `json:"hub-zone"`
	ZonePrefix  string `json:"zone-prefix"`
}

// Zone is a collection of devices that are connected together.
type Zone struct {
	ID          string `gorm:"primaryKey"`
	Peers       []*Peer
	Name        string
	Description string
	IpCidr      string
	HubZone     bool
}

// NewZone creates a new Zone and allocates the prefix using IPAM
func (ct *Controller) NewZone(id, name, description, cidr string, hubZone bool) (*Zone, error) {
	if _, err := ct.ipam.CreatePrefix(context.Background(), connect.NewRequest(&apiv1.CreatePrefixRequest{Cidr: cidr})); err != nil {
		return nil, err
	}
	zone := &Zone{
		ID:          id,
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
