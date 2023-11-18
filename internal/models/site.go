package models

import (
	"github.com/google/uuid"
)

// Site is a unique, end-user Site.
// Sites belong to one User and may be onboarded into an organization
type Site struct {
	Base
	Revision       uint64    `json:"revision" gorm:"type:bigserial;index:"`
	OwnerID        uuid.UUID `json:"owner_id"`
	VpcID          uuid.UUID `json:"vpc_id" example:"694aa002-5d19-495e-980b-3d8fd508ea10"`
	OrganizationID uuid.UUID `json:"-"`                      // Denormalized from the VPC record for performance
	RegKeyID       uuid.UUID `json:"-"`                      // the reg key id that created the Site (if it was created with a registration token)
	BearerToken    string    `json:"bearer_token,omitempty"` // the token nexd should use to reconcile Site state.
	Hostname       string    `json:"hostname" example:"myhost"`
	Os             string    `json:"os"`
	Name           string    `json:"name"`
	Platform       string    `json:"platform"`
	PublicKey      string    `json:"public_key"`
}

// AddSite is the information needed to add a new Site.
type AddSite struct {
	VpcID     uuid.UUID `json:"vpc_id" example:"694aa002-5d19-495e-980b-3d8fd508ea10"`
	Name      string    `json:"name"`
	Platform  string    `json:"platform"`
	PublicKey string    `json:"public_key"`
}

// UpdateSite is the information needed to update a Site.
type UpdateSite struct {
	Hostname string  `json:"hostname" example:"myhost"`
	Revision *uint64 `json:"revision"`
}
