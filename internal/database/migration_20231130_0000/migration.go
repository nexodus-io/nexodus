package migration_20231130_0000

import (
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database/migration_20231031_0000"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
	"gorm.io/gorm"
)

type User struct {
	migration_20231031_0000.Base
	IdpID    string `json:"-"`
	UserName string `json:"username"`
}

type UserIdentity struct {
	Kind   string    `gorm:"primary_key"`
	Value  string    `gorm:"primary_key"`
	UserID uuid.UUID `gorm:"type:uuid;index"`
	User   *User     `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Invitation struct {
	Email string `gorm:"index"`
}

func init() {
	migrationId := "20231130-0000"
	CreateMigrationFromActions(migrationId,
		AddTableColumnAction(&Invitation{}, "email"),
		ExecAction(
			`CREATE INDEX IF NOT EXISTS "idx_invitations_user_id" ON "invitations" ("user_id")`,
			`DROP INDEX IF EXISTS idx_invitations_user_id`,
		),
		CreateTableAction(&UserIdentity{}),
		func(tx *gorm.DB, apply bool) error {

			rows, err := tx.Unscoped().Model(&User{}).Rows()
			if err != nil {
				return err
			}
			defer rows.Close()

			for rows.Next() {
				var user User
				err = tx.ScanRows(rows, &user)
				if err != nil {
					return err
				}

				// create keycloak:id identity for the user..
				uid := UserIdentity{
					Kind:   "keycloak:id",
					Value:  user.IdpID,
					UserID: user.ID,
				}
				err = tx.Create(&uid).Error
				if err != nil {
					return err
				}
			}
			return nil
		},
	)
}
