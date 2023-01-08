//go:build !email
// +build !email

package bridgemap

import (
	bemail "github.com/42wim/matterbridge/bridge/email"
)

func init() {
	FullMap["email"] = bemail.New
}
