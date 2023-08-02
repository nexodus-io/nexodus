package cmd

import (
	"fmt"
	"gorm.io/gorm"
)

func ClearIpamDB(ipamDB *gorm.DB) error {
	type Table struct {
		TableName string `json:"table_name"`
	}
	tables := []Table{}
	res := ipamDB.Raw(`SELECT table_name FROM information_schema.tables WHERE table_schema='public'`).Find(&tables)
	if res.Error != nil {
		return res.Error
	}

	for _, t := range tables {
		res = ipamDB.Exec(`DROP table ` + t.TableName)
		if res.Error != nil {
			return res.Error
		}
	}

	fmt.Println("all ipam tables dropped...  now restart the ipam service.")
	return nil
}
