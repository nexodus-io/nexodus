package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/signalbus"
	"net/http"
	"time"
)

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

// waitForCancelOrTimeoutOrNotification returns true if the context has been canceled or false after the timeout or sub signal
func waitForCancelOrTimeoutOrNotification(ctx context.Context, timeout time.Duration, sub *signalbus.Subscription) bool {
	tc, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	select {
	case <-tc.Done():
		return false
	case <-sub.Signal():
		return false
	case <-ctx.Done():
		return true
	}
}
