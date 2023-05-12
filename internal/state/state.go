package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	atomicFile "github.com/natefinch/atomic"
	"github.com/nexodus-io/nexodus/internal/util"
	"golang.org/x/oauth2"
	"os"
	"path/filepath"
	"sync"
)

type State struct {
	AuthToken        *oauth2.Token    `json:"auth-token,omitempty"`
	PublicKey        string           `json:"public-key"`
	PrivateKey       string           `json:"private-key"`
	ProxyRulesConfig ProxyRulesConfig `json:"proxy-rules-config"`
}

type ProxyRulesConfig struct {
	Egress  []string `json:"egress"`
	Ingress []string `json:"ingress"`
}

type Store interface {
	fmt.Stringer
	Load() error
	Store() error
	State() *State
}

type FileStore struct {
	File  string
	state *State
	mu    sync.RWMutex
}

var _ Store = &FileStore{}

func (fs *FileStore) String() string {
	return fmt.Sprintf("file '%s'", fs.File)
}

func (fs *FileStore) State() *State {
	fs.mu.RLock()
	state := fs.state
	fs.mu.RUnlock()
	return state
}

// Load will read the state from the file
func (fs *FileStore) Load() error {
	state := State{}
	if _, err := os.Stat(fs.File); err != nil {
		fs.mu.Lock()
		fs.state = &state
		fs.mu.Unlock()
		return nil
	}
	f, err := os.Open(fs.File)
	if err != nil {
		return err
	}
	defer util.IgnoreError(f.Close)
	err = json.NewDecoder(f).Decode(&state)
	if err != nil {
		return err
	}

	fs.mu.Lock()
	fs.state = &state
	fs.mu.Unlock()
	return nil
}

// Store saves the state to the file
func (fs *FileStore) Store() error {
	// Create the path to the file if it doesn't exist.
	dir := filepath.Dir(fs.File)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0600); err != nil {
			return err
		}
	}

	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	err := enc.Encode(fs.state)
	if err != nil {
		return err
	}
	return atomicFile.WriteFile(fs.File, buf)
}
