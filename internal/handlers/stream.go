package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/internal/models"
	"gorm.io/gorm"
)

type WatchableList interface {
	Len() int
	Item(i int) (any, uint64, gorm.DeletedAt)
}

func (api *API) sendListOrWatch(c *gin.Context, ctx context.Context, signal string, revisionCol string, scopes []func(*gorm.DB) *gorm.DB, getList func(db *gorm.DB) (WatchableList, error)) {

	gtRevision := uint64(0)
	if v := c.Query("gt_revision"); v != "" {
		gtRevision, _ = strconv.ParseUint(v, 10, 0)
	}
	scopes = append(scopes,
		func(db *gorm.DB) *gorm.DB {
			if gtRevision == 0 {
				return db
			}
			return db.Where(revisionCol+" > ?", gtRevision)
		},
	)

	if v := c.Query("watch"); v == "true" {

		// fmt.Sprintf("/devices/org=%s", k.String())
		sub := api.signalBus.Subscribe(signal)
		defer sub.Close()

		idx := 1
		var list WatchableList
		bookmarkSent := false
		var err error

		scopes = append(scopes,
			func(db *gorm.DB) *gorm.DB {
				return db.Unscoped()
			},
		)

		c.Header("Content-Type", "application/json;stream=watch")
		c.Status(http.StatusOK)
		stream(c, func() models.WatchEvent {
			// This function blocks until there is an event to return...
			for {
				if err != nil {
					return models.WatchEvent{
						Type:  "error",
						Value: err.Error(),
					}
				}
				if list != nil && idx < list.Len() {

					result, revision, deletedAt := list.Item(idx)
					gtRevision = revision
					idx += 1

					if deletedAt.Valid {
						return models.WatchEvent{
							Type:  "delete",
							Value: result,
						}
					} else {
						return models.WatchEvent{
							Type:  "change",
							Value: result,
						}
					}
				} else {

					// get the next list...
					db := api.db.WithContext(ctx)

					for _, scope := range scopes {
						db = scope(db)
					}
					list, err = getList(db)
					if err != nil {
						return models.WatchEvent{
							Type:  "error",
							Value: err.Error(),
						}
					}
					idx = 0

					// did we run out of items to send?
					if list.Len() == 0 {

						// bookmark idea taken from: https://kubernetes.io/docs/reference/using-api/api-concepts/#watch-bookmarks
						if !bookmarkSent {
							bookmarkSent = true
							return models.WatchEvent{
								Type: "bookmark",
							}
						}

						// Wait for some items to come into the list
						if waitForCancelTimeoutOrNotification(ctx, 30*time.Second, sub.Signal()) == -2 {
							// ctx was canceled... likely due to the http connection being closed by
							// the client.  Signal the event stream is done.
							return models.WatchEvent{
								Type: "close",
							}
						}
					}
				}
			}
		})

	} else {
		api.sendList(c, ctx, getList, scopes)
	}
}

func (api *API) sendList(c *gin.Context, ctx context.Context, getList func(db *gorm.DB) (WatchableList, error), scopes []func(*gorm.DB) *gorm.DB) {
	db := api.db.WithContext(ctx)
	for _, scope := range scopes {
		db = scope(db)
	}
	items, err := getList(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		return
	}

	// For pagination
	c.Header("Access-Control-Expose-Headers", TotalCountHeader)
	c.Header(TotalCountHeader, strconv.Itoa(items.Len()))
	c.JSON(http.StatusOK, items)
}

func stream(c *gin.Context, nextEvent func() models.WatchEvent) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(fmt.Errorf("streaming unsupported")))
		return
	}
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

// waitForCancelTimeoutOrNotification returns -2 if ctx is closed, -1 on timeout, otherwise the index of the channel that
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
