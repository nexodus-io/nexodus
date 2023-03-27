package database

import (
	"context"
	"database/sql"
	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbgorm"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strings"
)

type TransactionFunc func(
	ctx context.Context, fn func(tx *gorm.DB) error, opts ...*sql.TxOptions,
) error

func Silent(db *gorm.DB) *gorm.DB {
	return db.Session(&gorm.Session{
		Logger: db.Logger.LogMode(logger.Silent),
	})
}

type Dialect int

const (
	DialectSqlLite Dialect = iota
	DialectPostgreSQL
	DialectCockroachDB
)

func GetTransactionFunc(db *gorm.DB) (TransactionFunc, Dialect, error) {

	version := ""
	_ = Silent(db).Raw("SELECT version()").Scan(&version).Error

	dialect := DialectSqlLite
	if strings.HasPrefix(version, "PostgreSQL") {
		dialect = DialectPostgreSQL
	} else if strings.HasPrefix(version, "CockroachDB") {
		dialect = DialectCockroachDB
	}

	if dialect == DialectCockroachDB {
		return func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...*sql.TxOptions) error {
			var o *sql.TxOptions = nil
			if len(opts) > 0 {
				o = opts[0]
			}
			return crdbgorm.ExecuteTx(ctx, db, o, fn)
		}, dialect, nil
	} else {
		return func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...*sql.TxOptions) error {
			var o *sql.TxOptions = nil
			if len(opts) > 0 {
				o = opts[0]
			}
			return db.WithContext(ctx).Transaction(fn, o)
		}, dialect, nil
	}
}
