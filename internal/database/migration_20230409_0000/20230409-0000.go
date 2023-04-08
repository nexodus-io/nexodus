package migration_20230409_0000

import (
	"github.com/go-gormigrate/gormigrate/v2"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
	"github.com/nexodus-io/nexodus/internal/models"
	"gorm.io/gorm"
)

type Device struct {
	Endpoints []models.Endpoint `json:"endpoints" gorm:"type:JSONB; serializer:json"`
}

type DeviceDropCols struct {
	LocalIP       string
	LocalIpV6     string
	ReflexiveIPv4 string
}

func Migrate() *gormigrate.Migration {
	migrationId := "20230409-0000"
	return CreateMigrationFromActions(migrationId,

		// Add the new cols
		AddTableColumnsAction(&Device{}),

		// move data to the new cols
		func(db *gorm.DB, apply bool) error {
			if !apply {
				return nil
			}
			type Device struct {
				models.Base
				LocalIP       string
				LocalIpV6     string
				ReflexiveIPv4 string
				Endpoints     []models.Endpoint `json:"endpoints" gorm:"type:JSONB; serializer:json"`
			}

			rows, err := db.Unscoped().Model(&Device{}).Rows()
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var device Device
				err = db.ScanRows(rows, &device)
				if err != nil {
					return err
				}

				endpoints := []models.Endpoint{}
				if device.LocalIP != "" {
					endpoints = append(endpoints, models.Endpoint{
						Source:   "local",
						Address:  device.LocalIP,
						Distance: 0,
					})
				}
				if device.LocalIpV6 != "" {
					endpoints = append(endpoints, models.Endpoint{
						Source:   "local",
						Address:  device.LocalIpV6,
						Distance: 0,
					})
				}
				if device.ReflexiveIPv4 != "" {
					endpoints = append(endpoints, models.Endpoint{
						Source:   "stun:stun1.l.google.com:19302",
						Address:  device.ReflexiveIPv4,
						Distance: 10,
					})
				}
				device.Endpoints = endpoints
				result := db.Unscoped().Save(&device)
				if result.Error != nil {
					return result.Error
				}
			}
			return nil
		},

		// drop the old cols
		DropTableColumnsAction(&DeviceDropCols{}, "devices"),
	)
}
