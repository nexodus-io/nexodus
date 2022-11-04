package database

import (
	"fmt"

	"github.com/redhat-et/apex/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewDatabase(
	host string,
	user string,
	password string,
	dbname string,
	port int,
	sslmode string,
) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		host, user, password, dbname, port, sslmode)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := Migrate(db); err != nil {
		return nil, err
	}
	return db, nil
}

func Migrate(db *gorm.DB) error {
	// Migrate the schema
	if err := db.AutoMigrate(&models.User{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&models.Zone{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&models.Peer{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&models.Device{}); err != nil {
		return err
	}
	return nil
}
