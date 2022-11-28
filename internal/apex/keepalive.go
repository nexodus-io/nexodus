package apex

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/redhat-et/apex/internal/apex/ipsec"
	log "github.com/sirupsen/logrus"
)

func (ax *Apex) Keepalive() {
	var peerEndpoints []string
	if !ax.hubRouter {
		for _, value := range ax.peerCache {
			peerEndpoints = append(peerEndpoints, value.EnpointLocalAddressIPv4)
		}
	}

	_ = probePeers(peerEndpoints)
}

// IpsecHealthCheck for legacy stroke only TODO: (for legacy stroke non-systemd max-thread exceeded issues only, likely removable)
func (ax *Apex) IpsecHealthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = exec.CommandContext(ctx, "ipsec", "status").Run()
	var dealineExceeded bool
	if ctx.Err() == context.DeadlineExceeded {
		dealineExceeded = true
	}
	if dealineExceeded {
		log.Warnf("ipsec status command exceeded deadline, killing and restarting ipsec\n")
		pid, err := getPid("charon")
		if err != nil {
			log.Errorf("%v", err)
		}
		if err = killProc(pid); err != nil {
			log.Errorf("failed to kill the process %s, %v", "charon", err)
		}
		// stop and restart ipsec
		if err := ipsec.StopIpsec(); err != nil {
			log.Errorf("failed to stop the ipsec process%v", err)
		}
		if err := ipsec.StartIpsec(); err != nil {
			log.Errorf("failed to start the ipsec process%v", err)
		}
	} else {
		pid, _ := getPid("charon")
		if pid == "" {
			if err := ipsec.StartIpsec(); err != nil {
				log.Errorf("failed to start the ipsec process%v", err)
			}
		}
	}
}

// getPid return the pid for a process name (for stroke non-systemd issues only, likely remove)
func getPid(p string) (string, error) {
	pid, _ := RunCommand("pidof", p)
	if pid == "" {
		return "", fmt.Errorf("no process %s found", p)
	}
	pid = strings.TrimSuffix(pid, "\n")

	return pid, nil
}

// killProc kill a process with the pid (for stroke non-systemd issues only, likely remove)
func killProc(p string) error {
	_, err := RunCommand("kill", "-9", p)
	if err != nil {
		return fmt.Errorf("failed to kill process: %v", err)
	}
	return nil
}
