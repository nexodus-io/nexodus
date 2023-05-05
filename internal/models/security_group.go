package models

import (
	"github.com/google/uuid"
)

// SecurityGroup represents a security group containing security rules and a group owner
type SecurityGroup struct {
	Base
	GroupName        string    `json:"group_name"`
	GroupDescription string    `json:"group_description"`
	OrganizationId   uuid.UUID `json:"org_id"`
	InboundRules     string    `json:"inbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
	OutboundRules    string    `json:"outbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
}

// AddSecurityGroup is the information needed to add a new Security Group.
type AddSecurityGroup struct {
	GroupName        string    `json:"group_name" example:"group_name"`
	GroupDescription string    `json:"group_description" example:"group_description"`
	OrganizationId   uuid.UUID `json:"org_id"`
	InboundRules     string    `json:"inbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
	OutboundRules    string    `json:"outbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
}

// UpdateSecurityGroup is the information needed to update an existing Security Group.
type UpdateSecurityGroup struct {
	GroupName        string `json:"group_name,omitempty"`
	GroupDescription string `json:"group_description,omitempty"`
	InboundRules     string `json:"inbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
	OutboundRules    string `json:"outbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
}

// SecurityRuleJson represents a Security Rule
type SecurityRuleJson struct {
	IpProtocol string   `json:"ip_protocol"`
	FromPort   int64    `json:"from_port"`
	ToPort     int64    `json:"to_port"`
	IpRanges   []string `json:"ip_ranges,omitempty"`
}
