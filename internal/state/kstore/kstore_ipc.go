//go:build !kubernetes

package kstore

import (
	"github.com/nexodus-io/nexodus/internal/state"
	"github.com/nexodus-io/nexodus/internal/state/ipc"
	"os"
)

func NewIfInCluster() (state.Store, error) {

	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return nil, nil
	}

	// to access the kstore implementation via IPC.
	s, err := ipc.New("nexd-kstore")
	if err != nil {
		return nil, err
	}

	err = s.Load()
	if err != nil {
		_ = s.Close()
		return nil, err
	}

	return s, nil
}
