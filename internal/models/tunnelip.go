package models

type TunnelIP struct {
	// IP address and port of the endpoint.
	Address string `json:"address" example:"10.1.1.1:51820"`
	// VPC CIDR this address was allocated from
	CIDR string `json:"cidr" example:"10.0.0.0/24"`
}
