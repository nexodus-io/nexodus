package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Device is a unique, end-user device.
// Devices belong to one User and may be onboarded into an organization
type Device struct {
	Base
	OwnerID         uuid.UUID      `json:"owner_id"`
	VpcID           uuid.UUID      `json:"vpc_id" example:"694aa002-5d19-495e-980b-3d8fd508ea10"`
	OrganizationID  uuid.UUID      `json:"-"` // Denormalized from the VPC record for performance
	PublicKey       string         `json:"public_key"`
	AllowedIPs      pq.StringArray `json:"allowed_ips" gorm:"type:text[]" swaggertype:"array,string"`
	IPv4TunnelIPs   []TunnelIP     `json:"ipv4_tunnel_ips" gorm:"type:JSONB; serializer:json"`
	IPv6TunnelIPs   []TunnelIP     `json:"ipv6_tunnel_ips" gorm:"type:JSONB; serializer:json"`
	AdvertiseCidrs  pq.StringArray `json:"advertise_cidrs" gorm:"type:text[]" swaggertype:"array,string"`
	Relay           bool           `json:"relay"`
	SymmetricNat    bool           `json:"symmetric_nat"`
	Hostname        string         `json:"hostname"`
	Os              string         `json:"os"`
	Endpoints       []Endpoint     `json:"endpoints" gorm:"type:JSONB; serializer:json"`
	Revision        uint64         `json:"revision" gorm:"type:bigserial;index:"`
	SecurityGroupId uuid.UUID      `json:"security_group_id"`
	Online          bool           `json:"online"`
	OnlineAt        *time.Time     `json:"online_at"`
	RegKeyID        uuid.UUID      `json:"-"`                      // the reg key id that created the device (if it was created with a registration token)
	BearerToken     string         `json:"bearer_token,omitempty"` // the token nexd should use to reconcile device state.
}

// AddDevice is the information needed to add a new Device.
type AddDevice struct {
	VpcID           uuid.UUID  `json:"vpc_id" example:"694aa002-5d19-495e-980b-3d8fd508ea10"`
	PublicKey       string     `json:"public_key"`
	AdvertiseCidrs  []string   `json:"advertise_cidrs" example:"172.16.42.0/24"`
	IPv4TunnelIPs   []TunnelIP `json:"ipv4_tunnel_ips" gorm:"type:JSONB; serializer:json"`
	Relay           bool       `json:"relay"`
	SymmetricNat    bool       `json:"symmetric_nat"`
	Hostname        string     `json:"hostname" example:"myhost"`
	Endpoints       []Endpoint `json:"endpoints" gorm:"type:JSONB; serializer:json"`
	Os              string     `json:"os"`
	SecurityGroupId uuid.UUID  `json:"security_group_id"`
}

// UpdateDevice is the information needed to update a Device.
type UpdateDevice struct {
	VpcID          uuid.UUID  `json:"vpc_id" example:"694aa002-5d19-495e-980b-3d8fd508ea10"`
	AdvertiseCidrs []string   `json:"advertise_cidrs" example:"172.16.42.0/24"`
	SymmetricNat   bool       `json:"symmetric_nat"`
	Hostname       string     `json:"hostname" example:"myhost"`
	Endpoints      []Endpoint `json:"endpoints" gorm:"type:JSONB; serializer:json"`
	Revision       *uint64    `json:"revision"`
}
