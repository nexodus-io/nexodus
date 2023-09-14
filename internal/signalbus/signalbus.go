// The signalbus package provides a simple way to issue notifications that named events have occurred.
package signalbus

import (
	"sync"
)

type SignalBus interface {
	// Notify will notify all the subscriptions created for the given named signal.
	Notify(name string)
	// NotifyAll will notify all the subscriptions
	NotifyAll()
	// Subscribe creates a subscription the named signal
	Subscribe(name string) *Subscription
}

var _ SignalBus = &signalBus{} // type check the interface is implemented.

type signalBus struct {
	sync.RWMutex
	signals map[string][]*Subscription
}

// NewSignalBus creates a new signalBus
func NewSignalBus() SignalBus {
	return &signalBus{
		signals: make(map[string][]*Subscription),
	}
}

// Notify will notify all the subscriptions created for the given named signal.
func (sb *signalBus) Notify(name string) {
	var result []*Subscription
	sb.RLock()
	result = sb.signals[name]
	sb.RUnlock()

	for _, sub := range result {
		select {
		case sub.c <- struct{}{}:
		default:
		}
	}
}
func (sb *signalBus) NotifyAll() {
	var result []*Subscription
	sb.RLock()
	for _, s := range sb.signals {
		result = append(result, s...)
	}
	sb.RUnlock()

	for _, sub := range result {
		select {
		case sub.c <- struct{}{}:
		default:
		}
	}
}

// Subscribe creates a subscription the named signal
func (sb *signalBus) Subscribe(name string) *Subscription {
	sub := &Subscription{
		sb:   sb,
		name: name,
		c:    make(chan struct{}, 1),
	}

	sb.Lock()
	subs := sb.signals[name]
	sb.signals[name] = append(subs, sub)
	sb.Unlock()
	return sub
}

func (sb *signalBus) close(sub *Subscription) {
	sb.Lock()
	subs := sb.signals[sub.name]
	for i, s := range subs {
		if s == sub {
			// replace it with the last item..
			lastIdx := len(subs) - 1
			if lastIdx != 0 {
				subs[i] = subs[lastIdx]
				// then shrink the slice...
				subs = subs[:lastIdx]
				sb.signals[sub.name] = subs
			} else {
				delete(sb.signals, sub.name)
			}
		}
	}
	sb.Unlock()
}

type Subscription struct {
	sb        *signalBus
	name      string
	closeOnce sync.Once
	c         chan struct{}
}

// Signal returns a channel that receives a true message when the subscription is notified.
//
// Signal is provided for use in select statements:
//
//	func WatchTheKey(sb *signalBus) error {
//	    sub := sb.Subscribe("the-key")
//	    defer sub.Close()
//	 	for {
//	 		select {
//	 		case <-sub.Signal():
//				// this waits for the signal to occur..
//	 			fmt.Print("the-key was signaled.")
//	 		}
//	 	}
//	}
func (sub *Subscription) Signal() <-chan struct{} {
	return sub.c
}

// IsSignaled checks to see if the subscription has been notified.
func (sub *Subscription) IsSignaled() bool {
	select {
	case <-sub.c:
		return true
	default:
		return false
	}
}

// Close is used to close out the subscription.
func (sub *Subscription) Close() {
	sub.closeOnce.Do(func() {
		sub.sb.close(sub)
	})
}
