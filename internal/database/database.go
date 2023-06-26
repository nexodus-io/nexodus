package database

import (
	"context"
	"fmt"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230113_0000"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230126_0000"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230314_0000"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230321_0000"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230327_0000"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230328_0000"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230401_0000"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230403_0000"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230409_0000"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230411_0000"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230412_0000"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230413_0000"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230428_0000"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230509_0000"
	"github.com/nexodus-io/nexodus/internal/database/migration_20230610_0000"
	"github.com/nexodus-io/nexodus/internal/database/migrations"
	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var tracer trace.Tracer

func init() {
	tracer = otel.Tracer("github.com/nexodus-io/nexodus/internal/database")
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
) (*gorm.DB, string, error) {
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
		if err != nil {
			return err
		}
		return nil
	}
	err := backoff.Retry(connectDb, backoff.WithContext(backoff.NewExponentialBackOff(), ctx))
	if err != nil {
		return nil, "", err
	}
	if err := db.Use(otelgorm.NewPlugin()); err != nil {
		return nil, "", err
	}
	return db, dsn, nil
}

func NewTestDatabase() (*gorm.DB, error) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	gormLogger := NewLogger(logger.Sugar())
	config := &gorm.Config{
		Logger: gormLogger,
	}
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), config)
	if err != nil {
		return nil, err
	}
	if err := Migrations().Migrate(context.Background(), db); err != nil {
		return nil, err
	}
	return db, nil
}

// Migrations gormigrate is a wrapper for gorm's migration functions that adds schema versioning and rollback capabilities.
// For help writing migration steps, see the gorm documentation on migrations: https://gorm.io/docs/migration.html
func Migrations() *migrations.Migrations {
	return &migrations.Migrations{
		GormOptions: &gormigrate.Options{
			TableName:      "apiserver_migrations",
			IDColumnName:   "id",
			IDColumnSize:   40,
			UseTransaction: false,
		},
		Migrations: []*gormigrate.Migration{
			migration_20230113_0000.Migrate(),
			migration_20230126_0000.Migrate(),
			migration_20230314_0000.Migrate(),
			migration_20230321_0000.Migrate(),
			migration_20230327_0000.Migrate(),
			migration_20230328_0000.Migrate(),
			migration_20230401_0000.Migrate(),
			migration_20230403_0000.Migrate(),
			migration_20230409_0000.Migrate(),
			migration_20230411_0000.Migrate(),
			migration_20230412_0000.Migrate(),
			migration_20230413_0000.Migrate(),
			migration_20230428_0000.Migrate(),
			migration_20230509_0000.Migrate(),
			migration_20230610_0000.Migrate(),
		},
	}
}
