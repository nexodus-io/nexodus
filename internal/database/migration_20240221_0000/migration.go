package migration_20240221_0000

import (
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/nexodus-io/nexodus/internal/database/datatype"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
	"github.com/nexodus-io/nexodus/internal/models"
	"gorm.io/gorm"
)

type UserOrganization struct {
	Roles datatype.StringArray `json:"roles" swaggertype:"array,string"`
}
type Invitation struct {
	Roles datatype.StringArray `json:"roles" swaggertype:"array,string"`
}

func init() {
	migrationId := "20240221-0000"
	CreateMigrationFromActions(migrationId,
		AddTableColumnsAction(&UserOrganization{}),
		AddTableColumnsAction(&Invitation{}),
		ExecActionIf(
			`CREATE INDEX IF NOT EXISTS "idx_user_organizations_roles" ON "user_organizations" USING GIN ("roles")`,
			`DROP INDEX IF EXISTS idx_user_organizations_roles`,
			NotOnSqlLite,
		),

		// While inspecting the DB I realized a bunch of id columns are not uuids... this should fix that
		ChangeColumnTypeActionIf(`devices`, `owner_id`, `text`, `uuid`, NotOnSqlLite),
		ChangeColumnTypeActionIf(`devices`, `vpc_id`, `text`, `uuid`, NotOnSqlLite),
		ChangeColumnTypeActionIf(`devices`, `organization_id`, `text`, `uuid`, NotOnSqlLite),
		ChangeColumnTypeActionIf(`devices`, `security_group_id`, `text`, `uuid`, NotOnSqlLite),
		ChangeColumnTypeActionIf(`devices`, `reg_key_id`, `text`, `uuid`, NotOnSqlLite),
		ChangeColumnTypeActionIf(`organizations`, `owner_id`, `text`, `uuid`, NotOnSqlLite),
		ChangeColumnTypeActionIf(`reg_keys`, `owner_id`, `text`, `uuid`, NotOnSqlLite),
		ChangeColumnTypeActionIf(`reg_keys`, `vpc_id`, `text`, `uuid`, NotOnSqlLite),
		ChangeColumnTypeActionIf(`reg_keys`, `organization_id`, `text`, `uuid`, NotOnSqlLite),
		ChangeColumnTypeActionIf(`reg_keys`, `device_id`, `text`, `uuid`, NotOnSqlLite),
		ChangeColumnTypeActionIf(`reg_keys`, `security_group_id`, `text`, `uuid`, NotOnSqlLite),
		ChangeColumnTypeActionIf(`security_groups`, `organization_id`, `text`, `uuid`, NotOnSqlLite),
		ChangeColumnTypeActionIf(`security_groups`, `vpc_id`, `text`, `uuid`, NotOnSqlLite),
		ChangeColumnTypeActionIf(`vpcs`, `organization_id`, `text`, `uuid`, NotOnSqlLite),

		func(tx *gorm.DB, apply bool) error {
			if apply && NotOnSqlLite(tx) {

				// this will fill in the role for the user_organizations table
				type UserOrganization struct {
					UserID         uuid.UUID      `json:"user_id" gorm:"type:uuid;primary_key"`
					OrganizationID uuid.UUID      `json:"organization_id" gorm:"type:uuid;primary_key"`
					Roles          pq.StringArray `json:"roles" gorm:"type:text[]" swaggertype:"array,string"`
				}

				result := tx.Model(&models.UserOrganization{}).
					Where("roles IS NULL").
					Update("roles", pq.StringArray{"member"})
				if result.Error != nil {
					return result.Error
				}

				type Organization struct {
					ID      uuid.UUID
					OwnerID uuid.UUID
				}
				rows := []Organization{}

				// make all sure all orgs have a member with the owner role
				sql := `SELECT DISTINCT id, owner_id FROM organizations LEFT JOIN user_organizations ON user_organizations.organization_id = organizations.id WHERE organization_id is NULL`
				result = tx.Raw(sql).FindInBatches(&rows, 100, func(tx *gorm.DB, batch int) error {
					for _, r := range rows {
						result := tx.Create(&UserOrganization{
							UserID:         r.OwnerID,
							OrganizationID: r.ID,
							Roles:          []string{"owner"},
						})
						if result.Error != nil {
							return result.Error
						}
					}
					return nil
				})
				if result.Error != nil {
					return result.Error
				}

				// make sure the owner's memberhip has the owner role
				sql = `select * FROM organizations LEFT JOIN user_organizations ON user_organizations.organization_id = organizations.id WHERE organizations.owner_id = user_organizations.user_id AND roles <> '{owner}'`
				result = tx.Raw(sql).FindInBatches(&rows, 100, func(tx *gorm.DB, batch int) error {
					for _, r := range rows {
						result := tx.Save(&UserOrganization{
							UserID:         r.OwnerID,
							OrganizationID: r.ID,
							Roles:          []string{"owner"},
						})
						if result.Error != nil {
							return result.Error
						}
					}
					return nil
				})
				if result.Error != nil {
					return result.Error
				}

			}
			return nil
		},
	)
}
