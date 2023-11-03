package models

type Endpoint struct {
	// How the endpoint was discovered
	Source string `json:"source"`
	// IP address and port of the endpoint.
	Address string `json:"address" example:"10.1.1.1:51820"`
}
