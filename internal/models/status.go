package models

import(

	"github.com/google/uuid"

)

type Status struct {
	UserId		uuid.UUID	`json:"user_id"`
	WgIP        string 		`json:"wg_ip"`
	IsReachable bool   		`json:"is_reachable"`
	Hostname    string 		`json:"hostname"`
	Latency     string 		`json:"latency"`
	Method      string 		`json:"method"`
}


/*type UpdateStatus struct{
    IsReachable bool   `json:"is_reachable"`
    Latency     string `json:""`
    Method      string `json:"method"`
}*/
