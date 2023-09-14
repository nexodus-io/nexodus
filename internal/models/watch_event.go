package models

// Watch is used to describe events you are interested in
type Watch struct {
	Kind       string                 `json:"kind,omitempty"`
	GtRevision uint64                 `json:"gt_revision,omitempty"`
	AtTail     bool                   `json:"at_tail,omitempty"`
	Options    map[string]interface{} `json:"options,omitempty"`
}

// WatchEvent struct for WatchEvent
type WatchEvent struct {
	Kind  string      `json:"kind,omitempty"`
	Type  string      `json:"type"`
	Value interface{} `json:"value,omitempty"`
}
