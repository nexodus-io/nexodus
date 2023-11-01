package public

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/util"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

var Logger *zap.SugaredLogger = nil

func (r ApiWatchEventsRequest) WatchEventsStream() (*WatchEventsStream, *http.Response, error) {
	return r.ApiService.WatchEventsStream(r)
}

func (a *VPCApiService) WatchEventsStream(r ApiWatchEventsRequest) (*WatchEventsStream, *http.Response, error) {
	// this is a stream friendly version of WatchEventsExecute
	var (
		localVarHTTPMethod  = http.MethodPost
		localVarPostBody    interface{}
		formFiles           []formFile
		localVarReturnValue *WatchEventsStream // different from WatchEventsExecute
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "VPCApiService.WatchEvents")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/api/vpc/{id}/events"
	localVarPath = strings.Replace(localVarPath, "{"+"id"+"}", url.PathEscape(parameterValueToString(r.id, "id")), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	if r.watches == nil {
		return localVarReturnValue, nil, reportError("watches is required and must be specified")
	}

	if r.publicKey != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "public_key", r.publicKey, "")
	}
	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"application/json"}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	// body params
	localVarPostBody = r.watches
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {

		// different from WatchEventsExecute: start
		localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
		_ = localVarHTTPResponse.Body.Close()
		localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
		if err != nil {
			return nil, localVarHTTPResponse, err
		}
		// different from WatchEventsExecute: end

		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
		if localVarHTTPResponse.StatusCode == 400 {
			var v ModelsBaseError
			err = a.client.decode(&v, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
			if err != nil {
				newErr.error = err.Error()
				return localVarReturnValue, localVarHTTPResponse, newErr
			}
			newErr.error = formatErrorMessage(localVarHTTPResponse.Status, &v)
			newErr.model = v
			return localVarReturnValue, localVarHTTPResponse, newErr
		}
		if localVarHTTPResponse.StatusCode == 401 {
			var v ModelsBaseError
			err = a.client.decode(&v, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
			if err != nil {
				newErr.error = err.Error()
				return localVarReturnValue, localVarHTTPResponse, newErr
			}
			newErr.error = formatErrorMessage(localVarHTTPResponse.Status, &v)
			newErr.model = v
			return localVarReturnValue, localVarHTTPResponse, newErr
		}
		if localVarHTTPResponse.StatusCode == 429 {
			var v ModelsBaseError
			err = a.client.decode(&v, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
			if err != nil {
				newErr.error = err.Error()
				return localVarReturnValue, localVarHTTPResponse, newErr
			}
			newErr.error = formatErrorMessage(localVarHTTPResponse.Status, &v)
			newErr.model = v
			return localVarReturnValue, localVarHTTPResponse, newErr
		}
		if localVarHTTPResponse.StatusCode == 500 {
			var v ModelsBaseError
			err = a.client.decode(&v, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
			if err != nil {
				newErr.error = err.Error()
				return localVarReturnValue, localVarHTTPResponse, newErr
			}
			newErr.error = formatErrorMessage(localVarHTTPResponse.Status, &v)
			newErr.model = v
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	// different from WatchEventsExecute: start
	return &WatchEventsStream{
		close:   localVarHTTPResponse.Body.Close,
		decoder: json.NewDecoder(localVarHTTPResponse.Body),
	}, localVarHTTPResponse, nil
	// different from WatchEventsExecute: end

}

type WatchEventsStream struct {
	decoder *json.Decoder
	close   func() error
}

func (ds *WatchEventsStream) Receive() (ModelsWatchEvent, error) {
	event := ModelsWatchEvent{}
	err := ds.decoder.Decode(&event)
	return event, err
}

func (ds *WatchEventsStream) Close() error {
	return ds.close()
}

// //////////////////////////////////////////////////////////////////////////////////////////////////
//
// //////////////////////////////////////////////////////////////////////////////////////////////////
type watchEventsDataLoaderKeyType struct{}

var watchEventsDataLoaderKey = watchEventsDataLoaderKeyType{}

func (r ApiWatchEventsRequest) NewSharedInformerContext() context.Context {
	return context.WithValue(r.ctx, watchEventsDataLoaderKey, NewWatchEventsDataLoader(r))
}
func getWatchEventsDataLoader(ctx context.Context) *WatchEventsDataLoader {
	if v, ok := ctx.Value(watchEventsDataLoaderKey).(*WatchEventsDataLoader); ok {
		return v
	}
	return nil
}

type WatchEventHandler = func(event ModelsWatchEvent, response *http.Response, err error)

type WatchEventsDataLoader struct {
	mu            sync.RWMutex
	request       ApiWatchEventsRequest
	watchHandlers map[*ModelsWatch]WatchEventHandler
	stream        *WatchEventsStream
}

func NewWatchEventsDataLoader(r ApiWatchEventsRequest) *WatchEventsDataLoader {
	return &WatchEventsDataLoader{
		watchHandlers: map[*ModelsWatch]WatchEventHandler{},
		request:       r,
	}
}

func (dl *WatchEventsDataLoader) Add(w *ModelsWatch, handler WatchEventHandler) bool {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	if dl.stream != nil {
		return false
	}
	dl.watchHandlers[w] = handler
	return true
}

func (dl *WatchEventsDataLoader) Remove(w *ModelsWatch) bool {
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

func (dl *WatchEventsDataLoader) start() bool {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if dl.stream != nil {
		return false
	}

	// we de-dupe watches here.. keep the watches wanting more data..
	handlers := map[string][]WatchEventHandler{}
	sharedWatches := map[string]*ModelsWatch{}
	for w, h := range dl.watchHandlers {
		handlers[w.Kind] = append(handlers[w.Kind], h)
		prev := sharedWatches[w.Kind]
		if prev == nil || w.GtRevision < prev.GtRevision {
			sharedWatches[w.Kind] = w
		}
	}
	var watches []ModelsWatch
	for _, watch := range sharedWatches {
		watches = append(watches, *watch)
	}
	request := dl.request
	request.watches = &watches

	stream, response, err := request.WatchEventsStream()
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

func (dl *WatchEventsDataLoader) run(response *http.Response, handlers map[string][]WatchEventHandler) {
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
				if event.Type == "" || event.Kind == kind {
					h(event, response, err)
				}
			}
		}
		if err != nil {
			return
		}
		switch event.Type {
		case "close", "error":
			return
		}
	}
}

type InformerAdaptor[T any] interface {
	Kind() string
	Item(value map[string]interface{}) (T, error)
	Revision(item T) int32
	Key(item T) string
}

type Informer[T any] struct {
	adaptor               InformerAdaptor[T]
	watchEventsDataLoader *WatchEventsDataLoader
	watch                 ModelsWatch
	bookmarkChan          chan struct{}
	changed               chan struct{}
	mu                    sync.RWMutex
	data                  map[string]T
	err                   error
	response              *http.Response
}

func NewInformer[T any](adaptor InformerAdaptor[T], gtRevision *int32, request ApiWatchEventsRequest) *Informer[T] {
	informer := Informer[T]{
		watch: ModelsWatch{
			Kind: adaptor.Kind(),
		},
		adaptor:      adaptor,
		changed:      make(chan struct{}, 1),
		data:         make(map[string]T),
		bookmarkChan: make(chan struct{}),
	}
	if gtRevision != nil {
		informer.watch.GtRevision = *gtRevision
	}

	informer.watchEventsDataLoader = getWatchEventsDataLoader(request.ctx)
	if informer.watchEventsDataLoader == nil || !informer.watchEventsDataLoader.Add(&informer.watch, informer.handleWatchEvent) {
		// Fall back to using an exclusive WatchEventsDataLoader if a shared one can't be used.
		informer.watchEventsDataLoader = NewWatchEventsDataLoader(request)
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
	bookmarked := informer.watch.AtTail
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
		if !informer.watch.AtTail {
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
	switch event.Type {
	case "change":
		item, informer.err = informer.adaptor.Item(event.Value)
		if err != nil {
			setError(err)
			return
		}
		revision := informer.adaptor.Revision(item)
		if revision < informer.watch.GtRevision {
			return
		}
		informer.watch.GtRevision = revision

		data := maps.Clone(informer.data)
		data[informer.adaptor.Key(item)] = item
		informer.data = data

		if informer.watch.AtTail {
			informer.notify()
		}
	case "delete":

		item, err = informer.adaptor.Item(event.Value)
		if err != nil {
			setError(err)
			return
		}
		revision := informer.adaptor.Revision(item)
		if revision < informer.watch.GtRevision {
			return
		}
		informer.watch.GtRevision = revision

		data := maps.Clone(informer.data)
		delete(data, informer.adaptor.Key(item))
		informer.data = data

		if informer.watch.AtTail {
			informer.notify()
		}
	case "tail":
		if !informer.watch.AtTail {
			informer.notify()
			informer.watch.AtTail = true
			close(informer.bookmarkChan)
		}
	case "close":
		if err != nil {
			informer.err = err
		}
		if !informer.watch.AtTail {
			informer.watch.AtTail = true
			close(informer.bookmarkChan)
		}
	case "error":
		item := ModelsBaseError{}
		err = util.JsonUnmarshal(event.Value, &item)
		if err == nil {
			err = errors.New(item.Error)
		}
		setError(err)

	default:
		informer.err = fmt.Errorf("unknown event type: %s", event.Type)
		informer.notify()
	}
}
