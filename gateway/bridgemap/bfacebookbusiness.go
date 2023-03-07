//go:build !facebookbusiness
// +build !facebookbusiness

package bridgemap

import (
	bfacebookbusiness "github.com/42wim/matterbridge/bridge/facebookbusiness"
)

func init() {
	FullMap["facebookbusiness"] = bfacebookbusiness.New
}
