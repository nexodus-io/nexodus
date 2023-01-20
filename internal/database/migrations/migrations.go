package migrations

import (
	"context"
	"fmt"
	"runtime"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

var tracer trace.Tracer

func init() {
	tracer = otel.Tracer("github.com/redhat-et/apex/internal/database")
}

type Migrations struct {
	Migrations  []*gormigrate.Migration
	GormOptions *gormigrate.Options
}

func (m *Migrations) Migrate(ctx context.Context, db *gorm.DB) error {
	_, span := tracer.Start(ctx, "Migrate")
	defer span.End()
	db = db.Debug()
	return gormigrate.New(db, m.GormOptions, m.Migrations).Migrate()
}

// Migrating to a specific migration will not seed the database, seeds are up to date with the latest
// schema based on the most recent migration
// This should be for testing purposes mainly
func (m *Migrations) MigrateTo(ctx context.Context, db *gorm.DB, migrationID string) error {
	_, span := tracer.Start(ctx, "MigrateTo")
	defer span.End()

	gm := gormigrate.New(db, m.GormOptions, m.Migrations)
	return gm.MigrateTo(migrationID)
}

func (m *Migrations) RollbackLast(ctx context.Context, db *gorm.DB) error {
	_, span := tracer.Start(ctx, "RollbackLast")
	defer span.End()

	gm := gormigrate.New(db, m.GormOptions, m.Migrations)
	if err := gm.RollbackLast(); err != nil {
		return err
	}
	return m.deleteMigrationTableIfEmpty(db)
}

func (m *Migrations) RollbackTo(ctx context.Context, db *gorm.DB, migrationID string) error {
	_, span := tracer.Start(ctx, "RollbackTo")
	defer span.End()

	gm := gormigrate.New(db, m.GormOptions, m.Migrations)
	return gm.RollbackTo(migrationID)
}

// RollbackAll rolls back all migrations..
func (m *Migrations) RollbackAll(ctx context.Context, db *gorm.DB) error {
	_, span := tracer.Start(ctx, "RollbackAll")
	defer span.End()

	gm := gormigrate.New(db, m.GormOptions, m.Migrations)
	type Result struct {
		ID string
	}
	sql := fmt.Sprintf("SELECT %s AS id FROM %s", m.GormOptions.IDColumnName, m.GormOptions.TableName)
	for {
		var result Result
		err := db.Raw(sql).Scan(&result).Error
		if err != nil || result.ID == "" {
			break
		}
		if err := gm.RollbackLast(); err != nil {
			return err
		}
	}
	return m.deleteMigrationTableIfEmpty(db)
}

func (m *Migrations) deleteMigrationTableIfEmpty(db *gorm.DB) error {
	if !db.Migrator().HasTable(m.GormOptions.TableName) {
		return nil
	}
	result, err := m.CountMigrationsApplied(db)
	if err != nil {
		return err
	}
	if result == 0 {
		if err := db.Migrator().DropTable(m.GormOptions.TableName); err != nil {
			return fmt.Errorf("could not drop migration table: %w", err)
		}
	}
	return nil
}

func (m *Migrations) CountMigrationsApplied(db *gorm.DB) (int, error) {
	if !db.Migrator().HasTable(m.GormOptions.TableName) {
		return 0, nil
	}
	sql := fmt.Sprintf("SELECT count(%s) AS id FROM %s", m.GormOptions.IDColumnName, m.GormOptions.TableName)
	var count int
	if err := db.Raw(sql).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

type MigrationAction func(tx *gorm.DB, apply bool) error

func CreateTableAction(table interface{}) MigrationAction {
	caller := ""
	if _, file, no, ok := runtime.Caller(1); ok {
		caller = fmt.Sprintf("[ %s:%d ]", file, no)
	}
	return func(tx *gorm.DB, apply bool) error {
		if apply {
			err := tx.AutoMigrate(table)
			if err != nil {
				return errors.Wrap(err, caller)
			}
		} else {
			err := tx.Migrator().DropTable(table)
			if err != nil {
				return errors.Wrap(err, caller)
			}
		}
		return nil
	}
}

func DropTableAction(table interface{}) MigrationAction {
	caller := ""
	if _, file, no, ok := runtime.Caller(1); ok {
		caller = fmt.Sprintf("[ %s:%d ]", file, no)
	}
	return func(tx *gorm.DB, apply bool) error {
		if apply {
			err := tx.Migrator().DropTable(table)
			if err != nil {
				return errors.Wrap(err, caller)
			}
		} else {
			err := tx.AutoMigrate(table)
			if err != nil {
				return errors.Wrap(err, caller)
			}
		}
		return nil
	}
}

func AddTableColumnsAction(table interface{}) MigrationAction {
	caller := ""
	if _, file, no, ok := runtime.Caller(1); ok {
		caller = fmt.Sprintf("[ %s:%d ]", file, no)
	}
	return func(tx *gorm.DB, apply bool) error {
		if apply {
			stmt := &gorm.Statement{DB: tx}
			if err := stmt.Parse(table); err != nil {
				return errors.Wrap(err, caller)
			}
			for _, field := range stmt.Schema.FieldsByDBName {
				if err := tx.Migrator().AddColumn(table, field.DBName); err != nil {
					return errors.Wrap(err, caller)
				}
			}
		} else {
			stmt := &gorm.Statement{DB: tx}
			if err := stmt.Parse(table); err != nil {
				return errors.Wrap(err, caller)
			}
			for _, field := range stmt.Schema.FieldsByDBName {
				if err := tx.Migrator().DropColumn(table, field.DBName); err != nil {
					return errors.Wrap(err, caller)
				}
			}
		}
		return nil

	}
}

func AddTableColumnAction(table interface{}, columnName string) MigrationAction {
	caller := ""
	if _, file, no, ok := runtime.Caller(1); ok {
		caller = fmt.Sprintf("[ %s:%d ]", file, no)
	}
	return func(tx *gorm.DB, apply bool) error {
		if apply {
			if err := tx.Migrator().AddColumn(table, columnName); err != nil {
				return errors.Wrap(err, caller)
			}
		} else {
			if err := tx.Migrator().DropColumn(table, columnName); err != nil {
				return errors.Wrap(err, caller)
			}
		}
		return nil

	}
}

func DropTableColumnsAction(table interface{}, tableName ...string) MigrationAction {
	caller := ""
	if _, file, no, ok := runtime.Caller(1); ok {
		caller = fmt.Sprintf("[ %s:%d ]", file, no)
	}
	return func(tx *gorm.DB, apply bool) error {
		if apply {
			stmt := &gorm.Statement{DB: tx}
			if err := stmt.Parse(table); err != nil {
				return errors.Wrap(err, caller)
			}
			if len(tableName) > 0 {
				stmt.Schema.Table = tableName[0]
			}
			for _, field := range stmt.Schema.FieldsByDBName {
				if err := tx.Migrator().DropColumn(table, field.DBName); err != nil {
					return errors.Wrap(err, caller)
				}
			}
		} else {
			stmt := &gorm.Statement{DB: tx}
			if err := stmt.Parse(table); err != nil {
				return errors.Wrap(err, caller)
			}
			if len(tableName) > 0 {
				stmt.Schema.Table = tableName[0]
			}
			for _, field := range stmt.Schema.FieldsByDBName {
				if err := tx.Migrator().AddColumn(table, field.DBName); err != nil {
					return errors.Wrap(err, caller)
				}
			}
		}
		return nil

	}
}

func DropTableColumnAction(table interface{}, columnName string) MigrationAction {
	caller := ""
	if _, file, no, ok := runtime.Caller(1); ok {
		caller = fmt.Sprintf("[ %s:%d ]", file, no)
	}
	return func(tx *gorm.DB, apply bool) error {
		if apply {
			if err := tx.Migrator().DropColumn(table, columnName); err != nil {
				return errors.Wrap(err, caller)
			}
		} else {
			if err := tx.Migrator().AddColumn(table, columnName); err != nil {
				return errors.Wrap(err, caller)
			}
		}
		return nil

	}
}

func RenameTableColumnAction(table interface{}, oldFieldName string, newFieldName string) MigrationAction {
	caller := ""
	if _, file, no, ok := runtime.Caller(1); ok {
		caller = fmt.Sprintf("[ %s:%d ]", file, no)
	}
	return func(tx *gorm.DB, apply bool) error {
		if apply {
			if err := tx.Migrator().RenameColumn(table, oldFieldName, newFieldName); err != nil {
				return errors.Wrap(err, caller)
			}
		} else {
			if err := tx.Migrator().RenameColumn(table, newFieldName, oldFieldName); err != nil {
				return errors.Wrap(err, caller)
			}
		}
		return nil

	}
}

func RenameTableAction(from interface{}, to interface{}) MigrationAction {
	caller := ""
	if _, file, no, ok := runtime.Caller(1); ok {
		caller = fmt.Sprintf("[ %s:%d ]", file, no)
	}
	return func(tx *gorm.DB, apply bool) error {
		if apply {
			if err := tx.Migrator().RenameTable(from, to); err != nil {
				return errors.Wrap(err, caller)
			}
		} else {
			if err := tx.Migrator().RenameTable(to, from); err != nil {
				return errors.Wrap(err, caller)
			}
		}
		return nil

	}
}

func ExecAction(applySql string, unapplySql string) MigrationAction {
	caller := ""
	if _, file, no, ok := runtime.Caller(1); ok {
		caller = fmt.Sprintf("[ %s:%d ]", file, no)
	}

	return func(tx *gorm.DB, apply bool) error {
		if apply {
			if applySql != "" {
				err := tx.Exec(applySql).Error
				if err != nil {
					return errors.Wrap(err, caller)
				}
			}
		} else {
			if unapplySql != "" {
				err := tx.Exec(unapplySql).Error
				if err != nil {
					return errors.Wrap(err, caller)
				}
			}
		}
		return nil

	}
}

func FuncAction(applyFunc func(*gorm.DB) error, unapplyFunc func(*gorm.DB) error) MigrationAction {
	caller := ""
	if _, file, no, ok := runtime.Caller(1); ok {
		caller = fmt.Sprintf("[ %s:%d ]", file, no)
	}

	return func(tx *gorm.DB, apply bool) error {
		if apply {
			err := applyFunc(tx)
			if err != nil {
				return errors.Wrap(err, caller)
			}
		} else {
			err := unapplyFunc(tx)
			if err != nil {
				return errors.Wrap(err, caller)
			}
		}
		return nil

	}
}

func CreateMigrationFromActions(id string, actions ...MigrationAction) *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: id,
		Migrate: func(tx *gorm.DB) error {
			tx = tx.Debug()
			for _, action := range actions {
				err := action(tx, true)
				if err != nil {
					return err
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			tx = tx.Debug()
			for i := len(actions) - 1; i >= 0; i-- {
				err := actions[i](tx, false)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
}
