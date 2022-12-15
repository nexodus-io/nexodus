package handlers

import (
	"context"

	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/ipam"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var tracer trace.Tracer

func init() {
	tracer = otel.Tracer("github.com/redhat-et/apex/internal/handlers")
}

type API struct {
	logger        *zap.SugaredLogger
	db            *gorm.DB
	ipam          ipam.IPAM
	defaultZoneID uuid.UUID
}

func NewAPI(parent context.Context, logger *zap.SugaredLogger, db *gorm.DB, ipam ipam.IPAM) (*API, error) {
	ctx, span := tracer.Start(parent, "NewAPI")
	defer span.End()
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
