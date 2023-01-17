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

func GetTransactionFunc(db *gorm.DB) (TransactionFunc, error) {

	version := ""

	_ = Silent(db).Raw("SELECT version()").Scan(&version).Error

	if strings.HasPrefix(version, "CockroachDB") {
		return func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...*sql.TxOptions) error {
			var o *sql.TxOptions = nil
			if len(opts) > 0 {
				o = opts[0]
			}
			return crdbgorm.ExecuteTx(ctx, db, o, fn)
		}, nil
	} else {
		return func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...*sql.TxOptions) error {
			var o *sql.TxOptions = nil
			if len(opts) > 0 {
				o = opts[0]
			}
			return db.WithContext(ctx).Transaction(fn, o)
		}, nil
	}
}
