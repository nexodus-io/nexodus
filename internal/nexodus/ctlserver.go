package nexodus

import (
	"fmt"
)

type NexdCtl struct {
	ax *Nexodus
}

func (ac *NexdCtl) Status(_ string, result *string) error {
	var statusStr string
	switch ac.ax.status {
	case NexdStatusStarting:
		statusStr = "Starting"
	case NexdStatusAuth:
		statusStr = "WaitingForAuth"
	case NexdStatusRunning:
		statusStr = "Running"
	default:
		statusStr = "Unknown"
	}
	res := fmt.Sprintf("Status: %s\n", statusStr)
	if len(ac.ax.statusMsg) > 0 {
		res += ac.ax.statusMsg
	}
	*result = res
	return nil
}

func (ac *NexdCtl) Version(_ string, result *string) error {
	*result = ac.ax.version
	return nil
}
