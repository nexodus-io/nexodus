package main

import (
	"context"
	"github.com/nexodus-io/nexodus/internal/stun"
	zap2 "go.uber.org/zap"
	"net"
	"os"
	"os/signal"
	"syscall"
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
	logger, err := zap2.NewDevelopment()
	if err != nil {
		panic(err)
	}

	address := ":3478"
	if len(os.Args) > 1 {
		address = os.Args[1]
	}

	server, err := stun.ListenAndStart(address, logger)
	if err != nil {
		panic(err)
	}
	defer server.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	defer cancel()

	<-ctx.Done()
}
