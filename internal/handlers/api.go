package handlers

import (
	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/ipam"
	"gorm.io/gorm"
)

type API struct {
	db            *gorm.DB
	ipam          ipam.IPAM
	defaultZoneID uuid.UUID
}

func NewAPI(db *gorm.DB, ipam ipam.IPAM) *API {
	return &API{
		db:            db,
		ipam:          ipam,
		defaultZoneID: uuid.Nil,
	}
}
