package nexodus

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
)

type InMemResolver struct {
	mu    sync.RWMutex
	hosts map[string][]netip.Addr
}

func NewInMemResolver() *InMemResolver {
	return &InMemResolver{
		hosts: map[string][]netip.Addr{},
	}
}
func (r *InMemResolver) LookupIP(ctx context.Context, host string) ([]netip.Addr, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r, found := r.hosts[host]; found {
		return r, nil
	}
	return nil, fmt.Errorf("not found")
}

func (r *InMemResolver) Set(host string, addrs []netip.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hosts[host] = addrs
}

func (r *InMemResolver) Delete(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.hosts, host)
}
