package datatype

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type StringArray []string

// GormDataType gorm common data type
func (StringArray) GormDataType() string {
	return "string_array"
}

// GormDBDataType gorm db data type
func (StringArray) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "text[]"
	default:
		return "JSON"
	}
}

// Value return json value, implement driver.Valuer interface
func (j StringArray) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j StringArray) GormValue(ctx context.Context, db *gorm.DB) clause.Expr {
	switch db.Dialector.Name() {
	case "postgres":
		return gorm.Expr("?", pq.StringArray(j))
	default:
		data, err := json.Marshal(j)
		if err != nil {
			db.Error = err
		}
		return gorm.Expr("?", string(data))
	}
}

// implements sql.Scanner interface
func (j *StringArray) Scan(value interface{}) error {
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New(fmt.Sprint("Failed to unmarshal string array value:", value))
	}

	if len(bytes) == 0 {
		*j = nil
		return nil
	}
	if bytes[0] == '[' {
		err := json.Unmarshal(bytes, j)
		return err
	}
	if bytes[0] == '{' {
		// Unmarshal as Postgres text array
		var a pq.StringArray
		err := a.Scan(value)
		if err != nil {
			return err
		}
		*j = StringArray(a)
		return nil
	}
	return errors.New(fmt.Sprint("Failed to unmarshal string array value:", value))
}
