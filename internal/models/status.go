package models

import (
	"github.com/google/uuid"
)

type Status struct {
	Base
	UserId      uuid.UUID `json:"user_id"`
	WgIP        string    `json:"wg_ip"`
	IsReachable bool      `json:"is_reachable"`
	Hostname    string    `json:"hostname"`
	Latency     string    `json:"latency"`
	Method      string    `json:"method"`
}

type AddStatus struct {
	WgIP        string `json:"wg_ip"`
	IsReachable bool   `json:"is_reachable"`
	Hostname    string `json:"hostname"`
	Latency     string `json:"latency"`
	Method      string `json:"method"`
}

type UpdateStatus struct {
	WgIP        string `json:"wg_ip"`
	IsReachable bool   `json:"is_reachable"`
	Latency     string `json:""`
	Method      string `json:"method"`
}
