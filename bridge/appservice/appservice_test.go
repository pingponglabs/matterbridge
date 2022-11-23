package bappservice

import (
	"encoding/json"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/42wim/matterbridge/bridge"
	"github.com/42wim/matterbridge/bridge/config"
	matrix "github.com/matterbridge/gomatrix"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	gomatrix "maunium.net/go/mautrix"
)

func TestAppServMatrix_Connect(t *testing.T) {
	fakeconf := FakeConfig{
		strKey:        map[string]string{},
		boolKey:       map[string]bool{},
		accountPrefix: "fakeuser",
	}
	fakeconf.Fill()

	b := AppServMatrix{
		mc:           &matrix.Client{},
		apsCli:       &gomatrix.Client{},
		UserID:       "",
		NicknameMap:  map[string]NicknameCacheEntry{},
		RoomMap:      map[string]string{},
		channelsInfo: map[string]*ChannelInfo{},
		rateMutex:    sync.RWMutex{},
		RWMutex:      sync.RWMutex{},
		Config: &bridge.Config{
			Bridge: &bridge.Bridge{
				Bridger:  nil,
				Name:     "",
				Account:  "fakeuser",
				Protocol: "",
				Channels: map[string]config.ChannelInfo{},
				Joined:   map[string]bool{},
				Config:   &fakeconf,
				General:  &config.Protocol{},
				Log:      logrus.NewEntry(logrus.New()),
			},
			Remote: make(chan config.Message),
		},
	}
	b.Connect()
	channelMember := []string{"Spr0cket"}
	b.loadState()

	msg := config.Message{
		Text:      "hi good morning",
		Channel:   "#ubuntu",
		Username:  "exterUser",
		UserID:    "123456",
		Avatar:    "",
		Account:   "exterUserAccount",
		Event:     "join_leave",
		Protocol:  "irc",
		Timestamp: time.Time{},
		ID:        "147852",
		Extra:     map[string][]interface{}{},
		ExtraNetworkInfo: config.ExtraNetworkInfo{
			ChannelUsersMember: channelMember,
			ActionCommand:      "quit",
			ChannelId:          "",
		},
	}

	b.Send(msg)
	b.saveState()
	time.Sleep(4 * time.Minute)
}
func TestAppServMatrix_Event(t *testing.T) {
	evBody := `{
	"events":[
	   {
		  "content":{
			 "displayname":"_irc_bridge_ex",
			 "membership":"invite"
		  },
		  "event_id":"$X4P4qoJV9ol9xl36C-IBMAw2EsAUziVLpp7nO9hUr-g",
		  "origin_server_ts":1656724456554,
		  "room_id":"!Je5iMpaE2o95pw6u:localhost",
		  "sender":"@sampletest:localhost",
		  "state_key":"@_irc_bridge_ex:localhost",
		  "type":"m.room.member",
		  "unsigned":{
			 "age":43,
			 "invite_room_state":[
				{
				   "content":{
					  "displayname":"_irc_bridge_ex",
					  "membership":"invite"
				   },
				   "sender":"@sampletest:localhost",
				   "state_key":"@_irc_bridge_ex:localhost",
				   "type":"m.room.member"
				},
				{
				   "content":{
					  "creator":"@sampletest:localhost",
					  "room_version":"6"
				   },
				   "sender":"@sampletest:localhost",
				   "state_key":"",
				   "type":"m.room.create"
				},
				{
				   "content":{
					  "join_rule":"public"
				   },
				   "sender":"@sampletest:localhost",
				   "state_key":"",
				   "type":"m.room.join_rules"
				},
				{
				   "content":{
					  "name":"kkko"
				   },
				   "sender":"@sampletest:localhost",
				   "state_key":"",
				   "type":"m.room.name"
				},
				{
				   "content":{
					  "alias":"#kko:localhost"
				   },
				   "sender":"@sampletest:localhost",
				   "state_key":"",
				   "type":"m.room.canonical_alias"
				},
				{
				   "content":{
					  "displayname":"_irc_bridge_ex",
					  "membership":"invite"
				   },
				   "sender":"@sampletest:localhost",
				   "state_key":"@_irc_bridge_ex:localhost",
				   "type":"m.room.member"
				}
			 ]
		  }
	   }
	]
 }`
	type AppEvents struct {
		Events []*matrix.Event `json:"events,omitempty"`
	}
	var apsEvents AppEvents

	err := json.Unmarshal([]byte(evBody), &apsEvents)
	log.Println(string(evBody))
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(apsEvents)
}

var _ config.Config = (*FakeConfig)(nil)

type FakeConfig struct {
	strKey        map[string]string
	boolKey       map[string]bool
	accountPrefix string // emulate the toml bridge config
}

func (c *FakeConfig) Fill() {

	c.strKey[c.accountPrefix+"."+"MxId"] = "@_irc_bot:localhost"
	c.strKey[c.accountPrefix+"."+"HsToken"] = "CqtpqUyM"
	c.strKey[c.accountPrefix+"."+"Token"] = "30c05ae90a248a4188e620216fa72e349803310ec83e2a77b34fe90be6081f46"
	c.strKey[c.accountPrefix+"."+"Server"] = "http://localhost:8008"
	c.strKey[c.accountPrefix+"."+"StorePath"] = "/tmp/room-info-discord.json"
	c.strKey[c.accountPrefix+"."+"MainUser"] = "@ibra10:localhost"
	c.strKey[c.accountPrefix+"."+"Port"] = "1234"
	c.boolKey[c.accountPrefix+"."+"UseUserName"] = true
	c.strKey[c.accountPrefix+"."+"RemoteNickFormat"] = "{NICK}"
	c.boolKey[c.accountPrefix+"."+"ShowJoinPart"] = true
	c.strKey[c.accountPrefix+"."+"ApsPrefix"] = "_irc_bridge_"
	c.strKey[c.accountPrefix+"."+"UserSuffix"] = "bd"
	c.strKey[c.accountPrefix+"."+"AvatarUrl"] = "/home/ibrahim/Downloads/Whatsapp-icon.png"
}
func (c *FakeConfig) Viper() *viper.Viper {
	return nil
}
func (c *FakeConfig) BridgeValues() *config.BridgeValues {
	return nil
}
func (c *FakeConfig) IsKeySet(key string) bool {
	return false
}
func (c *FakeConfig) GetBool(key string) (bool, bool) {
	v, ok := c.boolKey[key]
	return v, ok
}
func (c *FakeConfig) GetInt(key string) (int, bool) {
	return 0, false
}
func (c *FakeConfig) GetString(key string) (string, bool) {
	v, ok := c.strKey[key]
	return v, ok

}

func (c *FakeConfig) GetStringSlice(key string) ([]string, bool) {
	return nil, false
}

func (c *FakeConfig) GetStringSlice2D(key string) ([][]string, bool) {
	return nil, false
}
