//go:build !notelegram
// +build !notelegram

package bridgemap

import (
	btwitter "github.com/42wim/matterbridge/bridge/twitter"
)

func init() {
	FullMap["twitter"] = btwitter.New
}
