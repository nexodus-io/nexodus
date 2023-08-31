//go:build kubernetes

package main

import (
	"github.com/natefinch/pie"
	"github.com/nexodus-io/nexodus/internal/state/ipc"
	"github.com/nexodus-io/nexodus/internal/state/kstore"
	"log"
	"net/rpc/jsonrpc"
)

func main() {
	log.SetPrefix("[nexd-kstore log] ")
	provider := pie.NewProvider()
	server := ipc.NewServer(kstore.New())
	if err := provider.RegisterName("Store", server); err != nil {
		log.Fatalf("failed to register plugin: %s", err)
	}
	provider.ServeCodec(jsonrpc.NewServerCodec)
}
