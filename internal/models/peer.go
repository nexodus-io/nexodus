package models

import (
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Peer is an association between a Device and a Zone.
type Peer struct {
	Base
	DeviceID                uuid.UUID      `json:"device_id" example:"fde38e78-a4af-4f44-8f5a-d84ef1846a85"`
	ZoneID                  uuid.UUID      `json:"zone_id" example:"2b655c5b-cfdd-4550-b7f0-a36a590fc97a"`
	EndpointIP              string         `json:"endpoint_ip" example:"10.1.1.1"`
	AllowedIPs              pq.StringArray `json:"allowed_ips" gorm:"type:text[]"`
	NodeAddress             string         `json:"node_address" example:"1.2.3.4"`
	ChildPrefix             string         `json:"child_prefix" example:"172.16.42.0/24"`
	HubRouter               bool           `json:"hub_router"`
	HubZone                 bool           `json:"hub_zone"`
	ZonePrefix              string         `json:"zone_prefix" example:"10.1.1.0/24"`
	ReflexiveIPv4           string         `json:"reflexive_ip4"`
	EnpointLocalAddressIPv4 string         `json:"endpoint_local_address_ip4" example:"1.2.3.4"`
	SymmetricNat            bool           `json:"symmetric_nat"`
}

// AddPeer are the fields required to add a new Peer
type AddPeer struct {
	DeviceID                uuid.UUID      `json:"device_id" example:"6a6090ad-fa47-4549-a144-02124757ab8f"`
	EndpointIP              string         `json:"endpoint_ip" example:"10.1.1.1"`
	AllowedIPs              pq.StringArray `json:"allowed_ips" gorm:"type:text[]"`
	NodeAddress             string         `json:"node_address" example:"1.2.3.4"`
	ChildPrefix             string         `json:"child_prefix" example:"172.16.42.0/24"`
	HubRouter               bool           `json:"hub_router"`
	HubZone                 bool           `json:"hub_zone"`
	ZonePrefix              string         `json:"zone_prefix" example:"10.1.1.0/24"`
	ReflexiveIPv4           string         `json:"reflexive_ip4"`
	EnpointLocalAddressIPv4 string         `json:"endpoint_local_address_ip4" example:"1.2.3.4"`
	SymmetricNat            bool           `json:"symmetric_nat"`
}
