package migration_20231114_0000

import (
	"fmt"
	"github.com/google/uuid"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
	"gorm.io/gorm"
	"os"
	"time"
)

type Base struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
}

type User struct {
	Base
	IdpID         string
	UserName      string
	Organizations []*Organization `gorm:"many2many:user_organizations"`
}

type Organization struct {
	Base
	OwnerID     uuid.UUID
	Name        string
	Description string

	Users []*User `gorm:"many2many:user_organizations;"`
}

type VPC struct {
	Base
	OrganizationID uuid.UUID `json:"organization_id"`
	Description    string    `json:"description"`
	PrivateCidr    bool      `json:"private_cidr"`
	Ipv4Cidr       string    `json:"ipv4_cidr"`
	Ipv6Cidr       string    `json:"ipv6_cidr"`

	Organization *Organization `json:"-"`
}

type SecurityGroup struct {
	Base
	Description    string         `json:"description"`
	VpcId          uuid.UUID      `json:"vpc_id"`
	OrganizationID uuid.UUID      `json:"-"` // Denormalized from the VPC record for performance
	InboundRules   []SecurityRule `json:"inbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
	OutboundRules  []SecurityRule `json:"outbound_rules,omitempty" gorm:"type:JSONB; serializer:json"`
	Revision       uint64         `json:"revision"  gorm:"type:bigserial;index:"`
}
type SecurityRule struct {
	IpProtocol string   `json:"ip_protocol"`
	FromPort   int64    `json:"from_port"`
	ToPort     int64    `json:"to_port"`
	IpRanges   []string `json:"ip_ranges,omitempty"`
}

type RegKey struct {
	Base
	OwnerID        uuid.UUID  `json:"owner_id,omitempty"`
	VpcID          uuid.UUID  `json:"vpc_id,omitempty"`
	OrganizationID uuid.UUID  `json:"-"`                      // OrganizationID is denormalized from the VPC record for performance
	BearerToken    string     `json:"bearer_token,omitempty"` // BearerToken is the bearer token the client should use to authenticate the device registration request.
	Description    string     `json:"description,omitempty"`
	DeviceId       *uuid.UUID `json:"device_id,omitempty"`  // DeviceId is set if the RegKey was created for single use
	ExpiresAt      *time.Time `json:"expires_at,omitempty"` // ExpiresAt is optional, if set the registration key is only valid until the ExpiresAt time.
}

const (
	defaultIPAMv4Cidr = "100.64.0.0/10"
	defaultIPAMv6Cidr = "200::/64"
)

func init() {
	migrationId := "20231114-0000"
	CreateMigrationFromActions(migrationId,
		func(tx *gorm.DB, apply bool) error {
			if !(apply && os.Getenv("NEXAPI_ENVIRONMENT") == "development") {
				return nil
			}
			return tx.Transaction(func(tx *gorm.DB) error {

				// do we need to create the admin user?
				var count int64
				adminIdpId := "01578c9e-8e76-46a4-b2b2-50788cec2ccd"
				if res := tx.Model(&User{}).Where("idp_id = ?", adminIdpId).Count(&count); res.Error != nil {
					return res.Error
				}
				if count != 0 {
					return nil
				}

				user := &User{
					Base: Base{
						ID: uuid.New(),
					},
					IdpID:    adminIdpId,
					UserName: "admin",
				}
				if res := tx.Create(user); res.Error != nil {
					return res.Error
				}
				if res := tx.Create(&Organization{
					Base: Base{
						ID: user.ID,
					},
					OwnerID:     user.ID,
					Name:        user.UserName,
					Description: fmt.Sprintf("%s's organization", user.UserName),
					Users: []*User{{
						Base: Base{
							ID: user.ID,
						},
					}},
				}); res.Error != nil {
					return res.Error
				}
				if res := tx.Create(&VPC{
					Base: Base{
						ID: user.ID,
					},
					OrganizationID: user.ID,
					Description:    "default vpc",
					PrivateCidr:    false,
					Ipv4Cidr:       defaultIPAMv4Cidr,
					Ipv6Cidr:       defaultIPAMv6Cidr,
				}); res.Error != nil {
					return res.Error
				}

				if res := tx.Create(&SecurityGroup{
					Base: Base{
						ID: user.ID,
					},
					VpcId:          user.ID,
					OrganizationID: user.ID,
					Description:    "default vpc security group",
					InboundRules:   []SecurityRule{},
					OutboundRules:  []SecurityRule{},
				}); res.Error != nil {
					return res.Error
				}
				if res := tx.Create(&RegKey{
					Base: Base{
						ID: user.ID,
					},
					OwnerID:        user.ID,
					VpcID:          user.ID,
					OrganizationID: user.ID,
					Description:    "well known development reg key",
					BearerToken:    "RK:dev-admin-reg-key",
				}); res.Error != nil {
					return res.Error
				}

				return nil
			})
		},
	)
}
