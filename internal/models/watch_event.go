package models

// WatchEvent struct for WatchEvent
type WatchEvent struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value,omitempty"`
}
