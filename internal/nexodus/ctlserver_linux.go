//go:build linux

package nexodus

import (
	"context"
	"github.com/nexodus-io/nexodus/internal/util"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"sync"
)

// TODO make this path configurable
const UnixSocketPath = "/run/nexd.sock"

func (ax *Nexodus) CtlServerStart(ctx context.Context, wg *sync.WaitGroup) error {
	ax.CtlServerLinuxStart(ctx, wg)
	return nil
}

func (ax *Nexodus) CtlServerLinuxStart(ctx context.Context, wg *sync.WaitGroup) {
	util.GoWithWaitGroup(wg, func() {
		for {
			// Use a different waitgroup here, because we want to make sure
			// all of the subroutines have exited before we attempt to restart
			// the control server.
			ctlWg := &sync.WaitGroup{}
			err := ax.CtlServerLinuxRun(ctx, ctlWg)
			ctlWg.Done()
			if err == nil {
				// No error means it shut down cleanly because it got a message to stop
				break
			}
			ax.logger.Error("Ctl interface error, restarting: ", err)
		}
	})
}

func (ax *Nexodus) CtlServerLinuxRun(ctx context.Context, ctlWg *sync.WaitGroup) error {
	os.Remove(UnixSocketPath)
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: UnixSocketPath, Net: "unix"})
	if err != nil {
		ax.logger.Error("Error creating unix socket: ", err)
		return err
	}
	defer l.Close()

	ac := new(NexdCtl)
	ac.ax = ax
	err = rpc.Register(ac)
	if err != nil {
		ax.logger.Error("Error on rpc.Register(): ", err)
		return err
	}

	// This routine will exit when the listener is closed intentionally,
	// or some error occurs.
	errChan := make(chan error)
	util.GoWithWaitGroup(ctlWg, func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				errChan <- err
				break
			}
			util.GoWithWaitGroup(ctlWg, func() {
				jsonrpc.ServeConn(conn)
			})
		}
	})

	// Handle new connections until we get notified to stop the CtlServer,
	// or Accept() fails for some reason.
	stopNow := false
	for {
		select {
		case err = <-errChan:
			// Accept() failed, collect the error and stop the CtlServer
			stopNow = true
			ax.logger.Error("Error on Accept(): ", err)
			break
		case <-ctx.Done():
			ax.logger.Info("Stopping CtlServer")
			stopNow = true
			err = nil
		}
		if stopNow {
			break
		}
	}

	return err
}
