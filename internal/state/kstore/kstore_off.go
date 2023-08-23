//go:build !kubernetes

package kstore

import (
	"github.com/nexodus-io/nexodus/internal/state"
)

func NewIfInCluster() (state.Store, error) {
	return nil, nil
}
