//go:build linux || darwin || windows

package nexodus

import (
	"context"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"sync"
	"time"

	"github.com/nexodus-io/nexodus/internal/api"

	"github.com/nexodus-io/nexodus/internal/util"
)

func (nx *Nexodus) CtlServerStart(ctx context.Context, wg *sync.WaitGroup) error {
	return nx.CtlServerUnixStart(ctx, wg)
}

func (nx *Nexodus) createListener() (*net.UnixListener, error) {
	socketPath := api.UnixSocketPath
	_ = os.Remove(socketPath)
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		nx.logger.Error("Error creating unix socket: ", err)
		return nil, err
	}
	return l, nil
}

func (nx *Nexodus) CtlServerUnixStart(ctx context.Context, wg *sync.WaitGroup) error {
	l, err := nx.createListener()
	if err != nil {
		return err
	}

	util.GoWithWaitGroup(wg, func() {
		for {
			if ctx.Err() != nil {
				break
			}
			l, err = nx.createListener()
			if err != nil {
				time.Sleep(time.Second * 5)
				continue
			}
			// Use a different waitgroup here, because we want to make sure
			// all of the subroutines have exited before we attempt to restart
			// the control server.
			ctlWg := &sync.WaitGroup{}
			err := nx.CtlServerUnixRun(ctx, ctlWg, l)
			l.Close()
			ctlWg.Wait()
			if err == nil {
				// No error means it shut down cleanly because it got a message to stop
				break
			}
			nx.logger.Error("Ctl interface error, restarting: ", err)
			time.Sleep(time.Second * 5)
		}
	})

	return nil
}

func (nx *Nexodus) CtlServerUnixRun(ctx context.Context, ctlWg *sync.WaitGroup, l *net.UnixListener) error {
	ac := new(NexdCtl)
	ac.nx = nx
	err := rpc.Register(ac)
	if err != nil {
		nx.logger.Error("Error on rpc.Register(): ", err)
		return err
	}

	// This routine will exit when the listener is closed intentionally,
	// or some error occurs.
	errChan := make(chan error)
	util.GoWithWaitGroup(ctlWg, func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				// Don't return an error if the context was canceled
				if ctx.Err() == nil {
					errChan <- err
				}
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
			nx.logger.Error("Error on Accept(): ", err)
			break
		case <-ctx.Done():
			nx.logger.Info("Stopping CtlServer")
			stopNow = true
			err = nil
		}
		if stopNow {
			break
		}
	}

	return err
}
