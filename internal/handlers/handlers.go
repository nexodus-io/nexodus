package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	TotalCountHeader = "X-Total-Count"
)

type Query struct {
	Sort   string `form:"sort"`
	Filter string `form:"filter"`
	Range  string `form:"range"`
}

func (q *Query) GetSort() (string, error) {
	var parts []string
	if err := json.Unmarshal([]byte(q.Sort), &parts); err != nil {
		return "", err
	}
	if len(parts) != 2 {
		return "", fmt.Errorf("too many parts")
	}
	return strings.Join(parts, " "), nil
}

func (q *Query) GetRange() (int, int, error) {
	var parts []int
	if err := json.Unmarshal([]byte(q.Range), &parts); err != nil {
		return 0, 0, err
	}
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("too many parts")
	}
	start := parts[0]
	end := parts[1]
	pageSize := end - start + 1
	return pageSize, start, nil
}

func (q *Query) GetFilter() (map[string]interface{}, error) {
	var parts map[string]interface{}
	if err := json.Unmarshal([]byte(q.Filter), &parts); err != nil {
		return parts, err
	}
	return parts, nil
}

func FilterAndPaginate(model interface{}, c *gin.Context, orderBy string) func(db *gorm.DB) *gorm.DB {
	var query Query
	if err := c.BindQuery(&query); err != nil {
		return func(db *gorm.DB) *gorm.DB {
			db.Error = err
			return db
		}
	}
	return FilterAndPaginateWithQuery(model, c, query, orderBy)
}

func FilterAndPaginateWithQuery(model interface{}, c *gin.Context, query Query, defaultOrderBy string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {

		if order, err := query.GetSort(); err == nil {
			db = db.Order(order)
		} else if defaultOrderBy != "" {
			db = db.Order(defaultOrderBy)
		}

		if filter, err := query.GetFilter(); err == nil {
			db = db.Where(filter)
		}

		if pageSize, offset, err := query.GetRange(); err == nil {
			var totalCount int64
			countDBSession := db.Session(&gorm.Session{Initialized: true})
			res := countDBSession.Model(model).Count(&totalCount)
			if res.Error != nil {
				return db
			}
			c.Header("Access-Control-Expose-Headers", TotalCountHeader)
			c.Header(TotalCountHeader, strconv.Itoa(int(totalCount)))
			db = db.Offset(offset).Limit(pageSize)
		}
		return db
	}
}
