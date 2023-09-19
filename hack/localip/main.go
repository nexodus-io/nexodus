package main

import (
	"fmt"
	"net"
	"os"
)

func LocalIPv4Address() net.IP {
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
				return ip.IP
			}
		}
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
