package migration_20231206_0000

import (
	"errors"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database/migration_20231130_0000"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
	"gorm.io/gorm"
)

type User struct {
	FullName string
	Picture  string
}

type Invitation struct {
	FromID uuid.UUID
	From   migration_20231130_0000.User
}

func init() {
	migrationId := "20231206-0000"
	CreateMigrationFromActions(migrationId,
		func(tx *gorm.DB, apply bool) error {
			return AddTableColumnsAction(&User{})(tx, apply)
		},
		func(tx *gorm.DB, apply bool) error {
			return AddTableColumnsAction(&Invitation{})(tx, apply)
		},
		// AddTableColumnsAction(&Invitation{}),
		func(tx *gorm.DB, apply bool) error {
			if !apply {
				return nil
			}

			// try to add admin user email...
			user := migration_20231130_0000.User{}
			adminIdpId := "01578c9e-8e76-46a4-b2b2-50788cec2ccd"
			if res := tx.First(&user, "idp_id = ?", adminIdpId); res.Error != nil {
				if errors.Is(res.Error, gorm.ErrRecordNotFound) {
					return nil // skip if the admin user does not exist
				}
				return res.Error
			}

			// ignore error if the email already exists
			_ = tx.Create(&migration_20231130_0000.UserIdentity{
				Kind:   "email",
				Value:  "admin@example.com",
				UserID: user.ID,
			})
			return nil
		},
	)
}
