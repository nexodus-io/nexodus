/*
Nexodus API

This is the Nexodus API Server.

API version: 1.0
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package public

// ModelsAddSite struct for ModelsAddSite
type ModelsAddSite struct {
	Hostname  string `json:"hostname,omitempty"`
	Os        string `json:"os,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
	VpcId     string `json:"vpc_id,omitempty"`
}
