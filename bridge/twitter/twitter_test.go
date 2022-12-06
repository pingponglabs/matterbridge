package btwitter

import (
	"log"
	"testing"
	"time"

	"github.com/42wim/matterbridge/bridge"
	"github.com/42wim/matterbridge/bridge/config"
	"github.com/dghubble/go-twitter/twitter"
)

func TestBtwitter_handleSyncUpdates(t *testing.T) {
	remoteChan := make(chan config.Message)
	twitterBridge := New(&bridge.Config{
		Bridge: &bridge.Bridge{},
		Remote: remoteChan,
	})
	sendDmParams := twitter.DirectMessageEvent{
		CreatedAt: "1668783212804",
		ID:        "1593618415054458886",
		Type:      "message_create",
		Message: &twitter.DirectMessageEventMessage{
			SenderID: "1237519305992003586",
			Target: &twitter.DirectMessageTarget{
				RecipientID: "1592597515597144069",
			},
			Data: &twitter.DirectMessageData{
				Text:     "Hello to just you two, this is a new group conversation. https://t.co/9MSYLVMm02",
				Entities: &twitter.Entities{},
				Attachment: &twitter.DirectMessageDataAttachment{
					Type: "media",
					Media: twitter.MediaEntity{
						URLEntity:         twitter.URLEntity{},
						ID:                1593617104950935555,
						IDStr:             "1593617104950935555",
						MediaURL:          "https://ton.twitter.com/1.1/ton/data/dm/1593618415054458886/1593617104950935555/gwdBx6an.jpg",
						MediaURLHttps:     "https://ton.twitter.com/1.1/ton/data/dm/1593618415054458886/1593617104950935555/gwdBx6an.jpg",
						SourceStatusID:    0,
						SourceStatusIDStr: "",
						Type:              "",
						Sizes:             twitter.MediaSizes{},
						VideoInfo:         twitter.VideoInfo{},
					},
				},
				QuickReply: &twitter.DirectMessageQuickReply{},
				CTAs:       []twitter.DirectMessageCTA{},
			},
		},
	}
	tw := twitterBridge.(*Btwitter)
	tw.handleSyncUpdatesTest(sendDmParams)
	time.Sleep(200 * time.Second)



	msg := <-tw.Remote
	log.Println(msg)

}
