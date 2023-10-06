package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/signalbus"
	"github.com/nexodus-io/nexodus/internal/util"
	"golang.org/x/exp/slices"
	"gorm.io/gorm"
	"net/http"
	"reflect"
	"time"
)

type Watch struct {
	kind       string
	signal     string
	gtRevision uint64
	fetch      fetchmgr.FetchFn
	atTail     bool
}

var deviceCacheSize = 500

func init() {
	size, err := util.GetenvInt("NEXAPI_DEVICE_CACHE_SIZE", "500")
	if err == nil {
		deviceCacheSize = size
	}
}

// WatchEvents lets you watch for resource change events
// @Summary      Watch events occurring in the organization
// @Description  Watches events occurring in the organization
// @Id           WatchEvents
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param		 public_key      query  string          false "connect as the device with the given public key, device will be considered to be online for the duration of this request"
// @Param        Watches         body   []models.Watch  true  "List of events to watch"
// @Param		 organization_id path   string          true  "Organization ID"
// @Success      200  {object}  models.WatchEvent
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/organizations/{organization_id}/events [post]
func (api *API) WatchEvents(c *gin.Context) {

	ctx, span := tracer.Start(c.Request.Context(), "WatchEventsInOrganization")
	defer span.End()

	var query struct {
		PublicKey string `form:"public_key"`
	}

	if err := c.BindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.NewApiError(err))
		return
	}

	var request []models.Watch
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError())
		return
	}

	orgId, err := uuid.Parse(c.Param("organization"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}

	var org models.Organization
	db := api.db.WithContext(ctx)
	db = api.OrganizationIsReadableByCurrentUser(c, db)
	result := db.First(&org, "id = ?", orgId.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			api.sendInternalServerError(c, result.Error)
		}
		return
	}

	tokenClaims, err2 := NxodusClaims(c, api.db.WithContext(ctx))
	if err2 != nil {
		c.JSON(err2.Status, err2.Body)

		return
	}

	var closers []func()
	defer func() {
		for _, closer := range closers {
			closer()
		}
	}()
	var watches []Watch
	for i, r := range request {
		switch r.Kind {

		case "device":

			fetcher := api.fetchManager.Open("org-devices:"+orgId.String(), deviceCacheSize, func(db *gorm.DB, gtRevision uint64) (fetchmgr.ResourceList, error) {
				var items deviceList
				db = db.Unscoped().Limit(100).Order("revision")
				if gtRevision != 0 {
					db = db.Where("revision > ?", gtRevision)
				}
				db = db.Where("organization_id = ?", orgId.String())
				result := db.Find(&items)
				if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
					return nil, result.Error
				}

				for i := range items {
					if hideDeviceBearerToken(items[i], tokenClaims) {
						items[i].BearerToken = ""
					}
				}

				return items, nil
			})
			defer fetcher.Close()

			watches = append(watches, Watch{
				kind:       r.Kind,
				gtRevision: r.GtRevision,
				atTail:     r.AtTail,
				signal:     fmt.Sprintf("/devices/org=%s", orgId.String()),
				fetch:      fetcher.Fetch,
			})

		case "security-group":
			watches = append(watches, Watch{
				kind:       r.Kind,
				gtRevision: r.GtRevision,
				atTail:     r.AtTail,
				signal:     fmt.Sprintf("/security-groups/org=%s", orgId.String()),
				fetch: func(db *gorm.DB, gtRevision uint64) (fetchmgr.ResourceList, error) {
					var items securityGroupList
					db = db.Unscoped().Limit(100).Order("revision")
					if gtRevision != 0 {
						db = db.Where("revision > ?", gtRevision)
					}
					db = db.Where("organization_id = ?", orgId.String())
					result := db.Find(&items)
					if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
						return nil, result.Error
					}
					return items, nil
				},
			})

		case "device-metadata":

			watchOptions := struct {
				Prefixes []string `form:"prefix"`
			}{}

			if r.Options != nil {
				b, err := json.Marshal(r.Options)
				if err != nil {
					c.JSON(http.StatusBadRequest, models.NewApiError(err))
				}
				err = json.Unmarshal(b, &watchOptions)
				if err != nil {
					c.JSON(http.StatusBadRequest, models.NewApiError(err))
				}
			}

			watches = append(watches, Watch{
				kind:       r.Kind,
				gtRevision: r.GtRevision,
				atTail:     r.AtTail,
				signal:     fmt.Sprintf("/metadata/org=%s", orgId.String()),
				fetch: func(db *gorm.DB, gtRevision uint64) (fetchmgr.ResourceList, error) {

					tempDB := db.Model(&models.DeviceMetadata{}).
						Joins("inner join devices on devices.id=device_metadata.device_id").
						Where( // extra wrapping Where needed to group the SQL expressions
							db.Where("devices.organization_id = ?", orgId.String()),
						)

					// Building OR expressions with gorm is tricky...
					if len(watchOptions.Prefixes) > 0 {
						expressions := db
						for i, prefix := range watchOptions.Prefixes {
							if i == 0 {
								expressions = expressions.Where("key LIKE ?", prefix+"%")
							} else {
								expressions = expressions.Or("key LIKE ?", prefix+"%")
							}
						}
						tempDB = tempDB.Where( // extra wrapping Where needed to group the SQL expressions
							expressions,
						)
					}
					db = tempDB

					var items deviceMetadataList
					db = db.Unscoped().Limit(100).Order("device_metadata.revision")

					if gtRevision != 0 {
						db = db.Where("device_metadata.revision > ?", gtRevision)
					}
					results := db.Find(&items)
					if results.Error != nil && !errors.Is(results.Error, gorm.ErrRecordNotFound) {
						return nil, results.Error
					}
					return items, nil
				},
			})

		default:
			c.JSON(http.StatusBadRequest, models.NewInvalidField(fmt.Sprintf("request[%d].kind", i)))
		}

	}

	api.onlineTracker.Connected(api, c, query.PublicKey, func() {
		api.sendMultiWatch(c, ctx, watches)
	})

}

func (api *API) sendMultiWatch(c *gin.Context, ctx context.Context, watches []Watch) {
	type watchState struct {
		Watch
		sub    *signalbus.Subscription
		idx    int
		list   fetchmgr.ResourceList
		atTail bool
		err    error
		parked bool
	}

	var states []*watchState
	defer func() {
		for _, w := range states {
			if w.sub != nil {
				w.sub.Close()
			}
		}
	}()

	for _, w := range watches {
		state := &watchState{
			Watch: w,
		}

		// fmt.Sprintf("/devices/org=%s", k.String())
		state.sub = api.signalBus.Subscribe(w.signal)

		state.idx = 1
		state.atTail = w.atTail

		states = append(states, state)
	}

	c.Header("Content-Type", "application/json;stream=watch")
	c.Status(http.StatusOK)
	api.stream(c, func() models.WatchEvent {
		// This function blocks until there is an event to return...
		for {
			parkedCounter := 0
			for i, state := range states {
				if state.parked {

					// while servicing other watches, we might get a signal and have to unpark a watch
					select {
					case <-state.sub.Signal():
						state.parked = false
					case <-ctx.Done():
						return models.WatchEvent{
							Type: "close",
						}
					default: // we don't want this select to block
						parkedCounter += 1
					}

				} else if state.list != nil && state.idx < state.list.Len() {

					result, revision, deletedAt := state.list.Item(state.idx)
					state.gtRevision = revision
					state.idx += 1

					if deletedAt.Valid {
						return models.WatchEvent{
							Kind:  state.kind,
							Type:  "delete",
							Value: result,
						}
					}

					return models.WatchEvent{
						Kind:  state.kind,
						Type:  "change",
						Value: result,
					}

				} else {

					// get the next list...
					state.err = api.transaction(ctx, func(db *gorm.DB) error {
						var err error
						state.list, err = state.fetch(db, state.gtRevision)
						return err
					})
					if state.err != nil {
						state.sub.Close()
						states = slices.Delete(states, i, 1)
						return models.WatchEvent{
							Type:  "error",
							Value: models.NewApiError(state.err),
						}
					}
					state.idx = 0

					// did we run out of items to send?
					if state.list.Len() == 0 {
						// bookmark idea taken from: https://kubernetes.io/docs/reference/using-api/api-concepts/#watch-bookmarks
						if !state.atTail {
							state.atTail = true
							return models.WatchEvent{
								Kind: state.kind,
								Type: "tail",
							}
						}
						state.parked = true
						parkedCounter += 1
					}
				}
			}

			// are all the watches waiting for a notification?
			if parkedCounter == len(states) {
				var channels []<-chan struct{}
				for _, state := range states {
					channels = append(channels, state.sub.Signal())
				}
				// Wait for some items to come into any of the lists.
				notified := waitForCancelTimeoutOrNotification(ctx, 30*time.Second, channels...)
				if notified == -2 {
					// ctx was canceled... likely due to the http connection being closed by
					// the client.  Signal the event stream is done.
					return models.WatchEvent{
						Type: "close",
					}
				}
				if notified >= 0 {
					states[notified].parked = false
				}
			}
		}
	})
}

func (api *API) stream(c *gin.Context, nextEvent func() models.WatchEvent) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		api.sendInternalServerError(c, fmt.Errorf("streaming unsupported"))
		return
	}
	c.Writer.WriteHeader(200)
	flusher.Flush()
	for {
		result := nextEvent()
		if result.Type == "close" {
			return
		}
		_ = json.NewEncoder(c.Writer).Encode(result)
		_, _ = c.Writer.Write([]byte("\n"))
		flusher.Flush() // sends the result to the client (forces Transfer-Encoding: chunked)
		if result.Type == "error" {
			return
		}
	}
}

// WaitForCancelTimeoutOrNotification returns -2 if ctx is closed, -1 on timeout, otherwise the index of the channel that
// was notified.
func waitForCancelTimeoutOrNotification(ctx context.Context, timeout time.Duration, channels ...<-chan struct{}) int {
	tc, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cases := []reflect.SelectCase{
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())},
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(tc.Done())},
	}
	for _, ch := range channels {
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)})
	}
	chosen, _, _ := reflect.Select(cases)
	return chosen - 2
}
