package models

import (
	"github.com/google/uuid"
)

// SecurityGroup represents a security group containing security rules and a group owner
type SecurityGroup struct {
	Base
	Description    string         `json:"description"`
	OrganizationId uuid.UUID      `json:"organization_id"`
	InboundRules   []SecurityRule `json:"inbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
	OutboundRules  []SecurityRule `json:"outbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
	Revision       uint64         `json:"revision"  gorm:"type:bigserial;index:"`
}

// AddSecurityGroup is the information needed to add a new Security Group.
type AddSecurityGroup struct {
	Description    string         `json:"description" example:"group_description"`
	OrganizationId uuid.UUID      `json:"organization_id"`
	InboundRules   []SecurityRule `json:"inbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
	OutboundRules  []SecurityRule `json:"outbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
}

// UpdateSecurityGroup is the information needed to update an existing Security Group.
type UpdateSecurityGroup struct {
	Description   *string        `json:"description,omitempty"`
	InboundRules  []SecurityRule `json:"inbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
	OutboundRules []SecurityRule `json:"outbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
}

// SecurityRule represents a Security Rule
type SecurityRule struct {
	IpProtocol string   `json:"ip_protocol"`
	FromPort   int64    `json:"from_port"`
	ToPort     int64    `json:"to_port"`
	IpRanges   []string `json:"ip_ranges,omitempty"`
}
