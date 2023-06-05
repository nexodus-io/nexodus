package models

import (
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Device is a unique, end-user device.
// Devices belong to one User and may be onboarded into an organization
type Device struct {
	Base
	UserID                   string         `json:"user_id"`
	OrganizationID           uuid.UUID      `json:"organization_id"`
	PublicKey                string         `json:"public_key"`
	AllowedIPs               pq.StringArray `json:"allowed_ips" gorm:"type:text[]" swaggertype:"array,string"`
	TunnelIP                 string         `json:"tunnel_ip"`
	TunnelIpV6               string         `json:"tunnel_ip_v6"`
	ChildPrefix              pq.StringArray `json:"child_prefix" gorm:"type:text[]" swaggertype:"array,string"`
	Relay                    bool           `json:"relay"`
	Discovery                bool           `json:"discovery"`
	OrganizationPrefix       string         `json:"organization_prefix"`
	OrganizationPrefixV6     string         `json:"organization_prefix_v6"`
	EndpointLocalAddressIPv4 string         `json:"endpoint_local_address_ip4"`
	SymmetricNat             bool           `json:"symmetric_nat"`
	Hostname                 string         `json:"hostname"`
	Os                       string         `json:"os"`
	Endpoints                []Endpoint     `json:"endpoints" gorm:"type:JSONB; serializer:json"`
	Revision                 uint64         `json:"revision" gorm:"type:bigserial;index:"`
	SecurityGroupId          uuid.UUID      `json:"security_group_id"`
}

// AddDevice is the information needed to add a new Device.
type AddDevice struct {
	UserID                   string     `json:"user_id" example:"694aa002-5d19-495e-980b-3d8fd508ea10"`
	OrganizationID           uuid.UUID  `json:"organization_id" example:"694aa002-5d19-495e-980b-3d8fd508ea10"`
	PublicKey                string     `json:"public_key"`
	TunnelIP                 string     `json:"tunnel_ip" example:"1.2.3.4"`
	TunnelIpV6               string     `json:"tunnel_ip_v6" example:"200::1"`
	ChildPrefix              []string   `json:"child_prefix" example:"172.16.42.0/24"`
	Relay                    bool       `json:"relay"`
	Discovery                bool       `json:"discovery"`
	EndpointLocalAddressIPv4 string     `json:"endpoint_local_address_ip4" example:"1.2.3.4"`
	SymmetricNat             bool       `json:"symmetric_nat"`
	Hostname                 string     `json:"hostname" example:"myhost"`
	Endpoints                []Endpoint `json:"endpoints" gorm:"type:JSONB; serializer:json"`
	Os                       string     `json:"os"`
	SecurityGroupId          uuid.UUID  `json:"security_group_id"`
}

// UpdateDevice is the information needed to update a Device.
type UpdateDevice struct {
	OrganizationID           uuid.UUID  `json:"organization_id" example:"694aa002-5d19-495e-980b-3d8fd508ea10"`
	ChildPrefix              []string   `json:"child_prefix" example:"172.16.42.0/24"`
	EndpointLocalAddressIPv4 string     `json:"endpoint_local_address_ip4" example:"1.2.3.4"`
	SymmetricNat             bool       `json:"symmetric_nat"`
	Hostname                 string     `json:"hostname" example:"myhost"`
	Endpoints                []Endpoint `json:"endpoints" gorm:"type:JSONB; serializer:json"`
	Revision                 *uint64    `json:"revision"`
}
