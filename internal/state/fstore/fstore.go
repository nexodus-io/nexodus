package fstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/natefinch/atomic"
	"github.com/nexodus-io/nexodus/internal/state"
	"github.com/nexodus-io/nexodus/internal/util"
	"os"
	"path/filepath"
	"sync"
)

type store struct {
	File  string
	state *state.State
	mu    sync.RWMutex
}

var _ state.Store = &store{}

func New(file string) state.Store {
	return &store{
		File: file,
	}
}

func (fs *store) String() string {
	return fmt.Sprintf("file '%s'", fs.File)
}

func (fs *store) State() *state.State {
	fs.mu.RLock()
	state := fs.state
	fs.mu.RUnlock()
	return state
}

// Load will read the state from the file
func (fs *store) Load() error {
	state := state.State{}
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
func (fs *store) Store() error {
	// Create the path to the file if it doesn't exist.
	dir := filepath.Dir(fs.File)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}

	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")

	err := enc.Encode(fs.State())
	if err != nil {
		return err
	}
	return atomic.WriteFile(fs.File, buf)
}

func (fs *store) Close() error {
	return nil
}
