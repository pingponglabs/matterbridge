package bappservice

import (
	"fmt"
	"log"
	"sync"
	"testing"

	"github.com/42wim/matterbridge/bridge"
	matrix "github.com/matterbridge/gomatrix"
	gomatrix "maunium.net/go/mautrix"
)

func TestAppServMatrix_OutcomingMention(t *testing.T) {
	type fields struct {
		mc          *matrix.Client
		apsCli      *gomatrix.Client
		UserID      string
		NicknameMap map[string]NicknameCacheEntry
		RoomMap     map[string]string
		roomsInfo   map[string]ChannelInfo
		rateMutex   sync.RWMutex
		RWMutex     sync.RWMutex
		Config      *bridge.Config
	}
	b := AppServMatrix{
		mc:           &matrix.Client{},
		apsCli:       &gomatrix.Client{},
		UserID:       "",
		NicknameMap:  map[string]NicknameCacheEntry{},
		RoomMap:      map[string]string{},
		channelsInfo: map[string]*ChannelInfo{},
		rateMutex:    sync.RWMutex{},
		RWMutex:      sync.RWMutex{},
		Config:       &bridge.Config{},
	}
	b.Connect()

	b.loadState()
	userMention := "_irc_bridge_wcraftn"
	text := fmt.Sprintf("hi %s:", userMention)
	res, _ := b.outcomingMention("discord", text, text)
	log.Println(res)
}
