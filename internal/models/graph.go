package models

import (


	"github.com/google/uuid"
	
)

type KeepaliveStatus struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
	WgIP        string
	IsReachable bool
	Hostname    string
	Latency     string
	Method      string
}