package database

import (
	"context"
	"database/sql"
	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbgorm"
	"gorm.io/gorm"
	"strings"
)

type TransactionFunc func(
	ctx context.Context, db *gorm.DB, opts *sql.TxOptions, fn func(tx *gorm.DB) error,
) error

func DefaultTransactionFunc(ctx context.Context, db *gorm.DB, opts *sql.TxOptions, fn func(tx *gorm.DB) error) error {
	db = db.WithContext(ctx)
	return db.Transaction(fn, opts)
}

func GetTransactionFunc(db *gorm.DB) (TransactionFunc, error) {

	version := ""
	err := db.Raw("SELECT version()").Scan(&version).Error
	if err != nil {
		// sqlite gives us this error.
		if strings.HasPrefix(err.Error(), "no such function") {
			return DefaultTransactionFunc, nil
		}
		return nil, err
	}

	if strings.HasPrefix(version, "CockroachDB") {
		return crdbgorm.ExecuteTx, nil
	} else {
		return DefaultTransactionFunc, nil
	}
}
