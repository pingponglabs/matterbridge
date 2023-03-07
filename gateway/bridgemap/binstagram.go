//go:build !instagram
// +build !instagram

package bridgemap

import (
	binstagram "github.com/42wim/matterbridge/bridge/instagram"
)

func init() {
	FullMap["instagram"] = binstagram.New
}
