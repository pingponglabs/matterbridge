package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

var pollingLock sync.Mutex
var reqLocked bool

func (b *API) handleCustomStream(c echo.Context) error {
	//lock this method to prevent multiple clients from connecting

	if reqLocked {
		return c.String(http.StatusTooManyRequests, "request locked")
	}

	pollingLock.Lock()
	reqLocked = true
	defer func() {
		reqLocked = false
	}()
	defer pollingLock.Unlock()

	c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	for {
		// TODO: this causes issues, messages should be broadcasted to all connected clients
		msg := b.Messages.Dequeue()
		if msg != nil {
			c.Response().WriteHeader(http.StatusOK)
			if err := json.NewEncoder(c.Response()).Encode(msg); err != nil {
				return err
			}
			c.Response().Flush()
			return nil
		}
		if c.Request().Context().Err() != nil {
			b.Log.Debugf("custom-stream : possible client disconnected ,  context error: %v", c.Request().Context().Err())
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}
