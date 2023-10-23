package migration_20231019_0000

import (
	"errors"
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
	"time"
)

// RegistrationToken is used to register devices without an interactive login.
type RegistrationToken struct {
	DeviceId   *uuid.UUID
	Expiration *time.Time
}

type Device struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;"`
	BearerToken string
}

func Migrate() *gormigrate.Migration {
	migrationId := "20231019-0000"
	return CreateMigrationFromActions(migrationId,
		AddTableColumnsAction(&RegistrationToken{}),
		ExecAction(
			`CREATE UNIQUE INDEX IF NOT EXISTS "registration_tokens_bearer_token" ON "registration_tokens" ("bearer_token")`,
			`DROP INDEX registration_tokens_bearer_token`,
		),
		ExecAction(
			`CREATE UNIQUE INDEX IF NOT EXISTS "devices_bearer_token" ON "devices" ("bearer_token")`,
			`DROP INDEX devices_bearer_token`,
		),

		MigrationAction(func(tx *gorm.DB, apply bool) error {
			if apply {
				// Migrate existing device tokens to the shorter format...
				rows, err := tx.Model(&Device{}).Where("substr(bearer_token,1,3) <> 'DT:' OR bearer_token IS NULL").Rows()
				if err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return nil
					}
					return err
				}
				defer rows.Close()

				for rows.Next() {
					var device Device
					// ScanRows is a method of `gorm.DB`, it can be used to scan a row into a struct
					err = tx.ScanRows(rows, &device)
					if err != nil {
						return err
					}

					deviceToken, err := wgtypes.GeneratePrivateKey()
					if err != nil {
						return err
					}
					device.BearerToken = "DT:" + deviceToken.String()
					err = tx.Save(&device).Error
					if err != nil {
						return err
					}
				}
			}
			return nil
		}),
	)
}
