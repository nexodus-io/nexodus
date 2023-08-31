package ipc

import (
	"fmt"
	"github.com/natefinch/pie"
	"github.com/nexodus-io/nexodus/internal/state"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"runtime"
	"sync"
)

type ipcStore struct {
	executable string
	state      *state.State
	mu         sync.RWMutex
	client     *rpc.Client
}

var _ state.Store = &ipcStore{}

func New(executable string) (state.Store, error) {
	if runtime.GOOS == "windows" {
		executable = executable + ".exe"
	}
	client, err := pie.StartProviderCodec(jsonrpc.NewClientCodec, os.Stderr, executable)
	if err != nil {
		return nil, fmt.Errorf("error starting store plugin: %w", err)
	}

	return &ipcStore{
		executable: executable,
		client:     client,
	}, nil
}

func (sps *ipcStore) Close() error {
	return sps.client.Close()
}

func (sps *ipcStore) String() string {
	return fmt.Sprintf("command '%s'", sps.executable)
}

func (sps *ipcStore) State() *state.State {
	sps.mu.RLock()
	result := sps.state
	sps.mu.RUnlock()
	return result
}

// Load will read the state RPC based cli command
func (sps *ipcStore) Load() error {
	sps.mu.Lock()
	defer sps.mu.Unlock()
	result := state.State{}
	err := sps.client.Call("Store.Load", true, &result)
	if err != nil {
		return err
	}
	sps.state = &result
	return nil
}

// Store saves the state to the file
func (sps *ipcStore) Store() error {
	sps.mu.Lock()
	defer sps.mu.Unlock()
	result := true
	return sps.client.Call("Store.Store", *sps.state, &result)
}
