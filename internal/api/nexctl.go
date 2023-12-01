package api

type PingPeersResponse struct {
	RelayPresent  bool                       `json:"relay-present"`
	RelayRequired bool                       `json:"relay-required"`
	Peers         map[string]KeepaliveStatus `json:"peers"`
}

type KeepaliveStatus struct {
	WgIP        string `json:"wg_ip"`
	IsReachable bool   `json:"is_reachable"`
	Hostname    string `json:"hostname"`
	Latency     string `json:""`
	Method      string `json:"method"`
}
