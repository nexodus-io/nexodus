package database

import (
	"context"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var tracer trace.Tracer

func init() {
	tracer = otel.Tracer("github.com/redhat-et/apex/internal/database")
}

func NewDatabase(
	parent context.Context,
	logger *zap.SugaredLogger,
	host string,
	user string,
	password string,
	dbname string,
	port string,
	sslmode string,
) (*gorm.DB, error) {
	ctx, span := tracer.Start(parent, "NewDatabase")
	defer span.End()
	gormLogger := NewLogger(logger)
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		host, user, password, dbname, port, sslmode)
	var db *gorm.DB
	connectDb := func() error {
		var err error
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: gormLogger,
		})
		db = db.Debug()
		if err != nil {
			return err
		}
		return nil
	}
	err := backoff.Retry(connectDb, backoff.WithContext(backoff.NewExponentialBackOff(), ctx))
	if err != nil {
		return nil, err
	}
	if err := db.Use(otelgorm.NewPlugin()); err != nil {
		return nil, err
	}
	return db, nil
}
