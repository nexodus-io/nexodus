package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/exp/maps"
	"net/http"
	"sync"
)

type InformerAdaptor[T any] interface {
	Kind() string
	Item(value map[string]interface{}) (T, error)
	Revision(item T) int32
	Key(item T) string
}

type Informer[T any] struct {
	adaptor               InformerAdaptor[T]
	watchEventsDataLoader *WatchDataLoader
	watch                 ModelsWatch
	bookmarkChan          chan struct{}
	changed               chan struct{}
	mu                    sync.RWMutex
	data                  map[string]T
	err                   error
	response              *http.Response
}

func NewInformer[T any](adaptor InformerAdaptor[T], gtRevision *int32, request ApiWatchRequest, options map[string]interface{}) *Informer[T] {
	informer := Informer[T]{
		watch: ModelsWatch{
			Kind:       PtrString(adaptor.Kind()),
			GtRevision: gtRevision,
			Options:    options,
		},
		adaptor:      adaptor,
		changed:      make(chan struct{}, 1),
		data:         make(map[string]T),
		bookmarkChan: make(chan struct{}),
	}

	informer.watchEventsDataLoader = getWatchDataLoader(request.ctx)
	if informer.watchEventsDataLoader == nil || !informer.watchEventsDataLoader.Add(&informer.watch, informer.handleWatchEvent) {
		// Fall back to using an exclusive WatchDataLoader if a shared one can't be used.
		informer.watchEventsDataLoader = NewWatchDataLoader(request)
		if !informer.watchEventsDataLoader.Add(&informer.watch, informer.handleWatchEvent) {
			panic("informer.watchEventsDataLoader.Add failed to add handler")
		}
	}
	return &informer
}

func (informer *Informer[T]) Changed() <-chan struct{} {
	return informer.changed
}

var ErrContextCanceled = errors.New("context canceled")

func (informer *Informer[T]) Execute() (map[string]T, *http.Response, error) {

	informer.mu.Lock()
	// after a failure... we need to reset some things... so that we can recover...
	if informer.err != nil {
		informer.err = nil
		informer.bookmarkChan = make(chan struct{})
	}
	bookmarked := informer.watch.GetAtTail()
	bookmarkChan := informer.bookmarkChan
	informer.mu.Unlock()

	informer.watchEventsDataLoader.start()

	// avoid returning a partial data list by, waiting for the bookmark event
	// which signals that all known data items have sent.
	canceled := false
	if !bookmarked {
		select {
		case <-informer.watchEventsDataLoader.request.ctx.Done():
			canceled = true
		case <-bookmarkChan:
		}
	}

	informer.mu.RLock()
	defer informer.mu.RUnlock()
	if canceled {
		return informer.data, informer.response, ErrContextCanceled
	}
	return informer.data, informer.response, informer.err
}

func (informer *Informer[T]) notify() {
	// try to signal...
	select {
	case informer.changed <- struct{}{}:
	default: // so we don't block if a signal is pending.
	}
}

func (informer *Informer[T]) handleWatchEvent(event ModelsWatchEvent, response *http.Response, err error) {
	informer.mu.Lock()
	defer informer.mu.Unlock()

	setError := func(err error) {
		informer.err = err
		if !informer.watch.GetAtTail() {
			close(informer.bookmarkChan)
		}
		informer.notify()
	}

	informer.response = response
	if err != nil {
		setError(err)
		return
	}

	var item T
	switch event.GetType() {
	case "change":
		item, informer.err = informer.adaptor.Item(event.Value)
		if err != nil {
			setError(err)
			return
		}
		revision := informer.adaptor.Revision(item)
		if revision < informer.watch.GetGtRevision() {
			return
		}
		informer.watch.GtRevision = &revision

		data := maps.Clone(informer.data)
		data[informer.adaptor.Key(item)] = item
		informer.data = data

		if informer.watch.GetAtTail() {
			informer.notify()
		}
	case "delete":

		item, err = informer.adaptor.Item(event.Value)
		if err != nil {
			setError(err)
			return
		}
		revision := informer.adaptor.Revision(item)
		if revision < informer.watch.GetGtRevision() {
			return
		}
		informer.watch.GtRevision = &revision

		data := maps.Clone(informer.data)
		delete(data, informer.adaptor.Key(item))
		informer.data = data

		if informer.watch.GetAtTail() {
			informer.notify()
		}
	case "tail":
		if !informer.watch.GetAtTail() {
			informer.notify()
			informer.watch.AtTail = PtrBool(true)
			close(informer.bookmarkChan)
		}
	case "close":
		if err != nil {
			informer.err = err
		}
		if !informer.watch.GetAtTail() {
			informer.watch.AtTail = PtrBool(true)
			close(informer.bookmarkChan)
		}
	case "error":
		item := ModelsBaseError{}
		err = JsonUnmarshal(event.Value, &item)
		if err == nil {
			err = errors.New(item.GetError())
		}
		setError(err)

	default:
		informer.err = fmt.Errorf("unknown event type: %s", event.GetType())
		informer.notify()
	}
}

func JsonUnmarshal(from map[string]interface{}, to interface{}) error {
	b, err := json.Marshal(from)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, to)
}
