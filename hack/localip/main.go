package main

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
)

func LocalIPv4Address() net.IP {
	type Candidate struct {
		priority int
		ip       net.IP
	}
	candidates := []Candidate{}

	interfaces, _ := net.Interfaces()
	for _, inter := range interfaces {
		addrs, err := inter.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch ip := addr.(type) {
			case *net.IPNet:
				if ip.IP.IsLoopback() {
					continue
				}
				if ip.IP.DefaultMask() == nil {
					continue
				}
				name := strings.ToLower(inter.Name)
				priority := 100
				if strings.HasPrefix(name, "bridge") {
					priority = 10
				} else if strings.HasPrefix(name, "en") {
					priority = 20
				} else if strings.HasPrefix(name, "eth") {
					priority = 20
				} else if strings.HasPrefix(name, "utun") {
					priority = 200
				}
				candidates = append(candidates, Candidate{
					priority: priority,
					ip:       ip.IP,
				})
			}
		}
	}

	if len(candidates) > 0 {
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].priority < candidates[j].priority
		})
		return candidates[0].ip
	}
	return nil
}

func main() {
	ip := LocalIPv4Address()
	if ip == nil {
		fmt.Fprintln(os.Stderr, "ip not found")
		os.Exit(1)
	}
	fmt.Println(ip.String())
}
