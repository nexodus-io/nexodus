package migration_20231002_0000

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	. "github.com/nexodus-io/nexodus/internal/database/migrations"
)

type Base struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// RegistrationToken is used to register devices without an interactive login.
type RegistrationTokenRecord struct {
	Base
	UserID         string    `json:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	// BearerToken is the token the client should use to authenticate the device registration request.
	BearerToken string `json:"bearer_token"`
	Description string `json:"description,omitempty"`
}

func (RegistrationTokenRecord) TableName() string {
	return "registration_tokens"
}

type Device struct {
	// the registration token id that created the device (if it was created with a registration token)
	RegistrationTokenID uuid.UUID
	// the token nexd should use to reconcile device state.
	BearerToken string `json:"bearer_token,omitempty"`
}

func Migrate() *gormigrate.Migration {
	migrationId := "20231002-0000"
	return CreateMigrationFromActions(migrationId,
		CreateTableAction(&RegistrationTokenRecord{}),
		AddTableColumnsAction(&Device{}),
	)
}
