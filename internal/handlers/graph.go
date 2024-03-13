package handlers

import (
	"context"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/api"
	
	"gorm.io/gorm"
)

func InsertKeepaliveStatuses(db *gorm.DB, statuses []models.KeepaliveStatus) error {
	// Using CreateInBatches for efficiency when dealing with multiple records
	return db.WithContext(context.Background()).CreateInBatches(statuses, 100).Error // Batch size of 100
}
func StoreConnectivityResults(db *gorm.DB, peerResults map[string]api.KeepaliveStatus) error {
	var statuses []models.KeepaliveStatus
	for _, result := range peerResults {
		statuses = append(statuses, models.KeepaliveStatus{
			WgIP:        result.WgIP,
			IsReachable: result.IsReachable,
			Hostname:    result.Hostname,
			Latency:     result.Latency,
			Method:      result.Method,
		})
	}

	// Insert the mapped results into the database
	return InsertKeepaliveStatuses(db, statuses)
}


	
