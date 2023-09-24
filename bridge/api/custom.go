package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/42wim/matterbridge/bridge/config"
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
    //  TODO: possible change it with select
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
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (b *API) AdjustExtra(msg *config.Message) {
	if msg.Extra == nil {
		return
	}
	res := []interface{}{}
	_, ok := msg.Extra["file"]
	if !ok {
		return
	}
	for _, fi := range msg.Extra["file"] {
		fb, err := json.Marshal(fi)
		if err != nil {
			b.Log.Debugf("extra marshal failed %v", err)
			return
		}
		fi := config.FileInfo{}
		err = json.Unmarshal(fb, &fi)
		if err != nil {
			b.Log.Debugf("extra unmarshal failed %v", err)
			return
		}
		res = append(res, fi)
	}

	msg.Extra["file"] = res
}
