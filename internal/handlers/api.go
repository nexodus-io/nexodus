package handlers

import (
	"context"

	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/ipam"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type API struct {
	logger        *zap.SugaredLogger
	db            *gorm.DB
	ipam          ipam.IPAM
	defaultZoneID uuid.UUID
}

func NewAPI(ctx context.Context, logger *zap.SugaredLogger, db *gorm.DB, ipam ipam.IPAM) (*API, error) {
	api := &API{
		logger:        logger,
		db:            db,
		ipam:          ipam,
		defaultZoneID: uuid.Nil,
	}
	if err := api.CreateDefaultZoneIfNotExists(ctx); err != nil {
		return nil, err
	}
	return api, nil
}
