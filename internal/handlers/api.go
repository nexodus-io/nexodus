package handlers

import (
	"context"

	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/ipam"
	"gorm.io/gorm"
)

type API struct {
	db            *gorm.DB
	ipam          ipam.IPAM
	defaultZoneID uuid.UUID
}

func NewAPI(ctx context.Context, db *gorm.DB, ipam ipam.IPAM) (*API, error) {
	api := &API{
		db:            db,
		ipam:          ipam,
		defaultZoneID: uuid.Nil,
	}
	if err := api.CreateDefaultZoneIfNotExists(ctx); err != nil {
		return nil, err
	}
	return api, nil
}
