package signalbus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nexodus-io/nexodus/internal/util"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/lib/pq"
)

var _ SignalBus = &PgSignalBus{} // type check the interface is implemented.

// PgSignalBus implements a signalbus.SignalBus that is clustered using postgresql notify events.
type PgSignalBus struct {
	db         *gorm.DB
	signalBus  SignalBus // typically an in memory signal bus.
	connectDSN string
	logger     *zap.SugaredLogger
}

// NewSignalBusService creates a new PgSignalBus
func NewPgSignalBus(signalBus SignalBus, db *gorm.DB, connectDSN string, logger *zap.SugaredLogger) *PgSignalBus {
	return &PgSignalBus{
		db:         db,
		connectDSN: connectDSN,
		signalBus:  signalBus,
		logger:     logger,
	}
}

// Notify will notify all the subscriptions created across the cluster of the given named signal.
func (pgsb *PgSignalBus) Notify(name string) {
	// instead of send the Notify to the in memory bus, first send it to the DB
	// with the pg_notify function.  The DB will send it back to us and all other processes
	// that are listening for those events.
	if err := pgsb.db.Exec("SELECT pg_notify('signalbus', ?)", name).Error; err != nil {
		pgsb.logger.Info("notify failed:", err.Error())
	}
}

func (pgsb *PgSignalBus) NotifyAll() {
	if err := pgsb.db.Exec("SELECT pg_notify('signalbus', ?)", "*").Error; err != nil {
		pgsb.logger.Info("notify failed:", err.Error())
	}
}

// Subscribe creates a subscription the named signal.
// They are performed on the in memory bus.
func (pgsb *PgSignalBus) Subscribe(name string) *Subscription {
	return pgsb.signalBus.Subscribe(name)
}

// Start starts the background worker that listens for the
// events that are sent from this process and all other processes publishing
// to the signalbus channel.
func (pgsb *PgSignalBus) Start(ctx context.Context, wg *sync.WaitGroup) {
	util.GoWithWaitGroup(wg, func() {

		// use the posgresql db driver specific APIs to listen for events from the DB connection.
		listener := pq.NewListener(pgsb.connectDSN, 10*time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
			if err != nil {
				pgsb.logger.Info("pq listener error", err.Error())
			}

			fmt.Println(ev)
			switch ev {
			case pq.ListenerEventReconnected:
				pgsb.signalBus.NotifyAll()
			}

		})
		defer listener.Close() // clean up connections on return..

		// Listen on the "signalbus" channel.
		err := listener.Listen("signalbus")
		if err != nil {
			pgsb.logger.Errorln("error listening to events:", err.Error())
			return
		}
		for {
			// Now lets pull events sent to the listener
			exit, err := pgsb.waitForNotification(ctx, listener)
			if err != nil {
				pgsb.logger.Errorln("error waiting for event:", err.Error())
				time.Sleep(1 * time.Second)
			}
			if exit {
				return
			}
		}

	})
}

func (pgsb *PgSignalBus) waitForNotification(ctx context.Context, l *pq.Listener) (exit bool, err error) {
	for {
		select {
		case <-ctx.Done():
			// this occurs triggered when PgSignalBus.Stop() is called... let the caller we should exit..
			return true, nil
		case n := <-l.Notify:
			if n == nil {
				// notify channel was closed.. likely due to db connection failure.
				return false, fmt.Errorf("postgres listner channel closed")
			}
			pgsb.logger.Infof("Received data from channel: %s, data: %s", n.Channel, n.Extra)

			// we got the signal name from the DB... lets use the in memory signalBus
			// to notify all the subscribers that registered for events.
			if n.Extra == "*" {
				pgsb.signalBus.NotifyAll()
			} else {
				pgsb.signalBus.Notify(n.Extra)
			}
			return
		case <-time.After(90 * time.Second):
			// in case we have not received an event in a while... lets check to make sure the DB
			// connection is still good... if not exit with error so we can retry...
			pgsb.logger.Debug("Received no events for 90 seconds, checking connection")
			err = l.Ping()
			if err != nil {
				return false, err
			}
		}
	}
}
