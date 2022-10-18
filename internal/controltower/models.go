package controltower

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
}

// Zone is a collection of devices that are connected together.
type Zone struct {
	ID          string
	Peers       []Peer
	Name        string
	Description string
	IpCidr      string
}

// NewZone creates a new Zone since we also need to create a database for zone IPAM.
// TODO: Investigate moving the IPAM service out of controltower and access it over grpc.
func (ct *ControlTower) NewZone(id, name, description, cidr string) (*Zone, error) {
	dbName := fmt.Sprintf("ipam_%s", strings.ReplaceAll(id, "-", "_"))
	log.Debugf("creating db %s", dbName)

	result := ct.db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
	if result.Error != nil && !strings.Contains(result.Error.Error(), "already exists") {
		return nil, result.Error
	}
	log.Debugf("created db %s", dbName)

	storage, err := goipam.NewPostgresStorage(
		ct.dbHost,
		"5432",
		"controltower",
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
		Peers:       make([]Peer, 0),
		Name:        name,
		Description: description,
		IpCidr:      cidr,
	}
	res := ct.db.Create(zone)
	if res.Error != nil {
		return nil, res.Error
	}
	return zone, nil
}
