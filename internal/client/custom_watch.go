package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
)

// //////////////////////////////////////////////////////////////////////////////////////////////////
//
// //////////////////////////////////////////////////////////////////////////////////////////////////
type watchEventsDataLoaderKeyType struct{}

var watchEventsDataLoaderKey = watchEventsDataLoaderKeyType{}

func (r ApiWatchRequest) NewSharedInformerContext() context.Context {
	return context.WithValue(r.ctx, watchEventsDataLoaderKey, NewWatchDataLoader(r))
}
func getWatchDataLoader(ctx context.Context) *WatchDataLoader {
	if v, ok := ctx.Value(watchEventsDataLoaderKey).(*WatchDataLoader); ok {
		return v
	}
	return nil
}

type WatchEventHandler = func(event ModelsWatchEvent, response *http.Response, err error)

type WatchDataLoader struct {
	mu            sync.RWMutex
	request       ApiWatchRequest
	watchHandlers map[*ModelsWatch]WatchEventHandler
	stream        *WatchStream
}

func NewWatchDataLoader(r ApiWatchRequest) *WatchDataLoader {
	return &WatchDataLoader{
		watchHandlers: map[*ModelsWatch]WatchEventHandler{},
		request:       r,
	}
}

func (dl *WatchDataLoader) Add(w *ModelsWatch, handler WatchEventHandler) bool {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	if dl.stream != nil {
		return false
	}
	dl.watchHandlers[w] = handler
	return true
}

func (dl *WatchDataLoader) Remove(w *ModelsWatch) bool {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	if dl.stream != nil {
		return false
	}
	if dl.watchHandlers[w] == nil {
		return false
	}
	delete(dl.watchHandlers, w)
	return true
}

func (dl *WatchDataLoader) start() bool {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if dl.stream != nil {
		return false
	}

	// we de-dupe watches here.. keep the watches wanting more data..
	handlers := map[string][]WatchEventHandler{}
	sharedWatches := map[string]*ModelsWatch{}
	for w, h := range dl.watchHandlers {
		kind := w.GetKind()
		handlers[kind] = append(handlers[kind], h)
		prev := sharedWatches[kind]
		if prev == nil || w.GetGtRevision() < prev.GetGtRevision() {
			sharedWatches[kind] = w
		}
	}
	var watches []ModelsWatch
	for _, watch := range sharedWatches {
		watches = append(watches, *watch)
	}
	request := dl.request
	request.watches = &watches

	stream, response, err := request.WatchStream()
	if err != nil {
		for _, hl := range handlers {
			for _, h := range hl {
				h(ModelsWatchEvent{}, response, err)
			}
		}
		return false
	}

	dl.stream = stream
	go dl.run(response, handlers)
	return true

}

func (dl *WatchDataLoader) run(response *http.Response, handlers map[string][]WatchEventHandler) {
	restart := false
	defer func() {
		dl.mu.Lock()
		_ = dl.stream.Close()
		dl.stream = nil
		dl.mu.Unlock()
		if restart {
			dl.start()
		}
	}()

	for i := 0; ; i++ {
		event, err := dl.stream.Receive()
		if err != nil {
			// right now envoy seems to be terminating our long-lived connections after about 10sec,
			// when that happens, we get this error, so try to recover.
			if errors.Is(err, io.ErrUnexpectedEOF) && i >= 0 {
				if Logger != nil {
					Logger.Debug("Event stream connection reset")
				}
				restart = true
				return
			}
		}
		for kind, hl := range handlers {
			for _, h := range hl {
				if event.GetType() == "" || event.GetKind() == kind {
					h(event, response, err)
				}
			}
		}
		if err != nil {
			return
		}
		switch event.GetType() {
		case "close", "error":
			return
		}
	}
}
