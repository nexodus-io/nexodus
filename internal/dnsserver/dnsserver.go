package dnsserver

import (
	"context"
	"errors"
	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/nexodus-io/nexodus/internal/util"
	"io"
	"log"
	"net"
	"sync"
)

type Server struct {
	instance *caddy.Instance
}

func Start(ctx context.Context, wg *sync.WaitGroup, config string) (*Server, error) {

	// Avoid sending coreDNS output logging...
	caddy.Quiet = true
	dnsserver.Quiet = true
	log.SetOutput(io.Discard)

	// We might want to set some of these caddy global options...
	//caddy.AppName = ""
	//caddy.AppVersion = ""
	//caddy.PidFile = ""
	//caddy.GracefulTimeout = 5 * time.Second

	instance, err := caddy.Start(input(config))
	if err != nil {
		return nil, err
	}

	go func() {
		<-ctx.Done()
		instance.ShutdownCallbacks()
	}()
	util.GoWithWaitGroup(wg, func() {
		// Twiddle your thumbs
		instance.Wait()
	})
	return &Server{
		instance: instance,
	}, nil
}

func (server *Server) Restart(config string) error {
	i, err := server.instance.Restart(input(config))
	if err != nil {
		return err
	}
	server.instance = i
	return nil
}

func (server *Server) Ports() (uppAddress net.Addr, tcpAddress net.Addr, err error) {
	listeners := server.instance.Servers()
	if len(listeners) == 0 {
		err = errors.New("no listeners")
		return
	}
	uppAddress = listeners[0].LocalAddr()
	tcpAddress = listeners[0].Addr()
	return
}

func input(contents string) caddy.CaddyfileInput {
	return caddy.CaddyfileInput{
		ServerTypeName: "dns",
		Filepath:       "Corefile",
		Contents:       []byte(contents),
	}
}
