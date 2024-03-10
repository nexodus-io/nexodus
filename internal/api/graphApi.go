package api



type ConnectivityStatus struct {
	IP          string `json:"ip"`
	Reachable   bool   `json:"reachable"`
	Hostname    string `json:"hostname"`
	Latency     string    `json:"latency"`
	ConnectionMethod string `json:"connection_method"`
}


func getRemapConnectivityResults(results map[string]KeepaliveStatus) []ConnectivityStatus {
	var remappedResults []ConnectivityStatus
	for _, status := range results {
		remappedResults = append(remappedResults, ConnectivityStatus{
			IP:          status.WgIP,
			Reachable:   status.IsReachable,
			Hostname:    status.Hostname,
			Latency:     status.Latency,
			ConnectionMethod: status.Method,
		})
	}
	return remappedResults
}