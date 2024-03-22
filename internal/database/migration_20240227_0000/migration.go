package migration_20240227_0000

import (
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database/migration_20231031_0000"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
	"gorm.io/gorm"
	"time"
)

type Organization struct {
	migration_20231031_0000.Base
}

type Site struct {
	migration_20231031_0000.Base
	Online   bool
	OnlineAt *time.Time
}

type ServiceNetwork struct {
	migration_20231031_0000.Base
	OrganizationID uuid.UUID `gorm:"type:uuid"`
	Organization   *Organization
	Description    string
	CaKey          string
	CaCertificates []string `gorm:"type:JSONB; serializer:json"`
	Revision       uint64   `gorm:"type:bigserial;index:"`
}

type VPC struct {
	CaKey          string   `json:"-"`
	CaCertificates []string `json:"ca_certificates,omitempty" gorm:"type:JSONB; serializer:json"`
}

type RegKey struct {
	ServiceNetworkID *uuid.UUID `gorm:"type:uuid"`
	SNOrganizationID *uuid.UUID `gorm:"type:uuid; column:sn_organization_id"`
}

func init() {
	migrationId := "20240227-0000"
	CreateMigrationFromActions(migrationId,
		AddTableColumnAction(&Site{}, "online"),
		AddTableColumnAction(&Site{}, "online_at"),
		AddTableColumnsAction(&RegKey{}),
		CreateTableAction(&ServiceNetwork{}),
		ExecActionIf(`
			CREATE OR REPLACE FUNCTION service_networks_revision_trigger() RETURNS TRIGGER LANGUAGE plpgsql AS '
			BEGIN
			NEW.revision := nextval(''service_networks_revision_seq'');
			RETURN NEW;
			END;'
		`, `
			DROP FUNCTION IF EXISTS service_networks_revision_trigger
		`, NotOnSqlLite),
		ExecActionIf(`
			CREATE OR REPLACE TRIGGER service_networks_revision_trigger BEFORE INSERT OR UPDATE ON service_networks
			FOR EACH ROW EXECUTE PROCEDURE service_networks_revision_trigger();
		`, `
			DROP TRIGGER IF EXISTS service_networks_revision_trigger ON service_networks
		`, NotOnSqlLite),
		func(tx *gorm.DB, apply bool) error {
			if apply {
				return tx.Unscoped().Where("1=1").Delete(&Site{}).Error // Delete all Sites
			}
			return nil
		},
		RenameTableColumnAction(&Site{}, "vpc_id", "service_network_id"),
		DropTableColumnsAction(&VPC{}),
	)
}
