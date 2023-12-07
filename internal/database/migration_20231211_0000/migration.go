package migration_20231211_0000

import (
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database/migration_20231031_0000"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type Site struct {
	migration_20231031_0000.Base
	Revision       uint64    `gorm:"type:bigserial;index"`
	OwnerID        uuid.UUID `gorm:"type:uuid;index"`
	VpcID          uuid.UUID `gorm:"type:uuid;index"`
	OrganizationID uuid.UUID `gorm:"type:uuid;index"`
	RegKeyID       uuid.UUID `gorm:"type:uuid;index"`
	BearerToken    string
	Hostname       string `gorm:"index"`
	Os             string
	Name           string
	Platform       string
	PublicKey      string
	LinkSecret     string
}

type VPC struct {
	CaKey          string
	CaCertificates []string `gorm:"type:JSONB; serializer:json"`
	Revision       uint64   `gorm:"type:bigserial;index:"`
}

func init() {
	migrationId := "20231211-0000"
	CreateMigrationFromActions(migrationId,
		CreateTableAction(&Site{}),
		ExecActionIf(`
			CREATE OR REPLACE FUNCTION sites_revision_trigger() RETURNS TRIGGER LANGUAGE plpgsql AS '
			BEGIN
			NEW.revision := nextval(''sites_revision_seq'');
			RETURN NEW;
			END;'
		`, `
			DROP FUNCTION IF EXISTS sites_revision_trigger
		`, NotOnSqlLite),
		ExecActionIf(`
			CREATE OR REPLACE TRIGGER sites_revision_trigger BEFORE INSERT OR UPDATE ON sites
			FOR EACH ROW EXECUTE PROCEDURE sites_revision_trigger();
		`, `
			DROP TRIGGER IF EXISTS sites_revision_trigger ON sites
		`, NotOnSqlLite),
		ExecAction(
			`CREATE UNIQUE INDEX IF NOT EXISTS "idx_sites_public_key" ON "sites" ("public_key")`,
			`DROP INDEX IF EXISTS idx_sites_public_key`,
		),
		ExecAction(
			`CREATE UNIQUE INDEX IF NOT EXISTS "idx_sites_bearer_token" ON "sites" ("bearer_token")`,
			`DROP INDEX IF EXISTS idx_sites_bearer_token`,
		),
		AddTableColumnsAction(&VPC{}),
		ExecActionIf(`
			CREATE OR REPLACE FUNCTION vpcs_revision_trigger() RETURNS TRIGGER LANGUAGE plpgsql AS '
			BEGIN
			NEW.revision := nextval(''vpcs_revision_seq'');
			RETURN NEW;
			END;'
		`, `
			DROP FUNCTION IF EXISTS vpcs_revision_trigger
		`, NotOnSqlLite),
		ExecActionIf(`
			CREATE OR REPLACE TRIGGER vpcs_revision_trigger BEFORE INSERT OR UPDATE ON vpcs
			FOR EACH ROW EXECUTE PROCEDURE vpcs_revision_trigger();
		`, `
			DROP TRIGGER IF EXISTS vpcs_revision_trigger ON vpcs
		`, NotOnSqlLite),
	)
}
