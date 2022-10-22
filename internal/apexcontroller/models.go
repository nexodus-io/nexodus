package apexcontroller

import (
	"context"
	"fmt"
	"strings"

	goipam "github.com/metal-stack/go-ipam"
	log "github.com/sirupsen/logrus"
)

// Device is a unique end-user device.
type Device struct {
	ID    string
	Peers []Peer
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
	ID          string
	Peers       []*Peer
	Name        string
	Description string
	IpCidr      string
	HubZone     bool
}

// NewZone creates a new Zone since we also need to create a database for zone IPAM.
// TODO: Investigate moving the IPAM service out of controller and access it over grpc.
func (ct *Controller) NewZone(id, name, description, cidr string, hubZone bool) (*Zone, error) {
	dbName := fmt.Sprintf("ipam_%s", strings.ReplaceAll(id, "-", "_"))
	log.Debugf("Creating db %s", dbName)

	result := ct.db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
	if result.Error != nil && !strings.Contains(result.Error.Error(), "already exists") {
		return nil, result.Error
	}
	log.Debugf("Created db %s", dbName)

	storage, err := goipam.NewPostgresStorage(
		ct.dbHost,
		"5432",
		"controller",
		ct.dbPass,
		dbName,
		goipam.SSLModeDisable,
	)
	if err != nil {
		return nil, err
	}
	ct.ipam[id] = goipam.NewWithStorage(storage)

	if _, err = ct.ipam[id].NewPrefix(context.Background(), cidr); err != nil {
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
