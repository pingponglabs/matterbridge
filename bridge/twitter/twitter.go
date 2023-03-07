package btwitter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/42wim/matterbridge/bridge"
	"github.com/42wim/matterbridge/bridge/config"
	"github.com/42wim/matterbridge/bridge/helper"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/joho/godotenv"
)

type Btwitter struct {
	client           *http.Client
	twitterClient    *twitter.Client
	UserID           string
	Groups           map[string]string
	GroupsLatestMsg  map[string]string
	AccountInfo      twitterUser
	LastMsgTimeStamp int
	sync.RWMutex
	*bridge.Config
	UserMap map[string]twitterUser
	syncMsg chan twitter.DirectMessageEvent
}
type twitterUser struct {
	ID                   string `json:"id"`
	CreatedTimestamp     string `json:"created_timestamp"`
	Name                 string `json:"name"`
	ScreenName           string `json:"screen_name"`
	Protected            bool   `json:"protected"`
	Verified             bool   `json:"verified"`
	FollowersCount       int    `json:"followers_count"`
	FriendsCount         int    `json:"friends_count"`
	StatusesCount        int    `json:"statuses_count"`
	ProfileImageURL      string `json:"profile_image_url"`
	ProfileImageURLHTTPS string `json:"profile_image_url_https"`
}

func New(cfg *bridge.Config) bridge.Bridger {

	b := &Btwitter{Config: cfg}
	b.Groups = make(map[string]string)
	b.GroupsLatestMsg = make(map[string]string)
	b.syncMsg = make(chan twitter.DirectMessageEvent)
	b.UserMap = make(map[string]twitterUser)
	return b
}

func (b *Btwitter) Connect() error {
	err := godotenv.Load()
	if err != nil {
		b.Log.Fatal("Error loading .env file")
	}
	consumerKey := os.Getenv("TWITTER_KEY")
	consumerSecret := os.Getenv("TWITTER_SECRET")
	accessToken := b.GetString("Oauth1Token")
	accessSecret := b.GetString("Oauth1Secret")

	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)

	// httpClient will automatically authorize http.Request's
	b.client = config.Client(oauth1.NoContext, token)
	b.twitterClient = twitter.NewClient(b.client)

	user, err := b.GetAccountInfo()
	if err != nil {
		return err
	}
	b.UserID = user.IDStr
	b.AccountInfo = twitterUser{
		ID:                   user.IDStr,
		CreatedTimestamp:     "",
		Name:                 user.Name,
		ScreenName:           user.ScreenName,
		Protected:            user.Protected,
		Verified:             user.Verified,
		FollowersCount:       user.FollowersCount,
		FriendsCount:         user.FriendsCount,
		StatusesCount:        user.StatusesCount,
		ProfileImageURL:      user.ProfileImageURL,
		ProfileImageURLHTTPS: user.ProfileImageURLHttps,
	}

	return nil
}

func (b *Btwitter) Disconnect() error {
	return nil
}

func (b *Btwitter) JoinChannel(channel config.ChannelInfo) error {

	return nil

}
func (b *Btwitter) HandleMediaUpload(msg *config.Message) error {
	if msg.Extra == nil {
		return fmt.Errorf("nil extra map")
	}

	for _, f := range msg.Extra["file"] {
		if fi, ok := f.(config.FileInfo); ok {
			content := bytes.NewReader(*fi.Data)
			resp, err := b.MediaUpload(content, fi.Name)
			if err != nil {
				return err
			}
			respSendEvent, err := b.SetDmEvent(msg.Channel, msg.Text, resp.MediaID)
			if err != nil {
				return err
			}
			b.Log.Debugf("respSendEvent: %#v", respSendEvent)
		}
	}
	return nil
}
func (b *Btwitter) SendText(msg *config.Message) error {
	respSendEvent, err := b.SetDmEvent(msg.Channel, msg.Text, 0)
	if err != nil {
		return err
	}
	b.Log.Debugf("SetDmEvent response: %#v", respSendEvent)
	return nil
}

func (b *Btwitter) Send(msg config.Message) (string, error) {
	b.Log.Debugf("=> Receiving %#v", msg)

	switch msg.Event {

	case "twitter-event":
		go b.HandleTwitterEvent(msg)
		return "", nil

	case "direct_msg":
		if msg.Extra != nil {
			if msg.Protocol == "appservice" {
				msg.Text = ""
			}
			err := b.HandleMediaUpload(&msg)
			if err != nil {
				return "", err
			}
		} else {
			err := b.SendText(&msg)
			if err != nil {
				return "", err
			}
		}

	}
	return "", nil

}
func (b *Btwitter) HandleTwitterEvent(Msg config.Message) {
	time.Sleep(50 * time.Millisecond)
	if Msg.Extra == nil {
		b.Log.Debug("empty twitter events data")
		return
	}
	if events, ok := Msg.Extra["twitter-event"]; ok {

		for _, v := range events {

			var event webhookEvent
			bb, err := json.Marshal(v)
			if err != nil {
				b.Log.Errorf("error while marshalling twitter event: %s", err)
				continue
			}
			err = json.Unmarshal(bb, &event)
			if err != nil {
				b.Log.Errorf("error while unmarshalling twitter event: %s", err)
				continue
			}
			b.RegisterParticipentInfo(event.Users)
			for _, entry := range event.DirectMessageEvents {
				if entry.Message.SenderID != b.AccountInfo.ID {
					b.handleDmEvents(*entry)
					return
				}
			}
		}
	}
}

func (b *Btwitter) GetParticipentInfo(PartcipentId string) (twitterUser, bool) {
	b.Lock()
	v, ok := b.UserMap[PartcipentId]
	b.Unlock()
	return v, ok
}

func (b *Btwitter) FetchParticipentInfo(PartcipentId string) (twitterUser, error) {
	userId, err := strconv.ParseInt(PartcipentId, 10, 64)
	if err != nil {
		return twitterUser{}, err
	}

	userInfo, err := b.GetUserInfo(int64(userId))
	if err != nil {
		return twitterUser{}, err
	}
	participentInfo := twitterUser{
		ID:                   userInfo.IDStr,
		CreatedTimestamp:     "",
		Name:                 userInfo.Name,
		ScreenName:           userInfo.ScreenName,
		Protected:            userInfo.Protected,
		Verified:             userInfo.Verified,
		FollowersCount:       userInfo.FollowersCount,
		FriendsCount:         userInfo.FriendsCount,
		StatusesCount:        userInfo.StatusesCount,
		ProfileImageURL:      userInfo.ProfileImageURL,
		ProfileImageURLHTTPS: userInfo.ProfileImageURLHttps,
	}
	b.UserMap[PartcipentId] = participentInfo
	return participentInfo, nil
}

func (b *Btwitter) HandleSendAttachment(rmsg *config.Message, senderID, mediaUrl string) error {
	rmsg.Extra = make(map[string][]interface{})

	img, err := b.MediaDownload(mediaUrl)
	if err != nil {
		return err
	}
	var name string
	sl := strings.Split(mediaUrl, "/")
	if len(sl) > 1 {
		name = sl[len(sl)-1]
	}
	helper.HandleDownloadData(b.Log, rmsg, name, "", mediaUrl, &img, b.General)
	return nil
}

func (b *Btwitter) handleSyncUpdatesTest(eventMsg twitter.DirectMessageEvent) {
	var err error

	rmsg := config.Message{
		Text:     eventMsg.Message.Data.Text,
		Channel:  eventMsg.Message.SenderID,
		Username: eventMsg.Message.SenderID,
		UserID:   eventMsg.Message.SenderID,
		Avatar:   "",
		Account:  b.Account,
		Event:    "direct_msg",
		Protocol: "twitter",

		ExtraNetworkInfo: config.ExtraNetworkInfo{
			ChannelUsersMember: []string{},
			ActionCommand:      "",
			ChannelId:          eventMsg.Message.SenderID,
			ChannelName:        eventMsg.Message.SenderID,
			ChannelType:        "direct_msg",
			TargetPlatform:     "appservice",
			UsersMemberId:      map[string]string{eventMsg.Message.SenderID: eventMsg.Message.SenderID},
			Mentions:           map[string]string{},
		},
	}

	if eventMsg.Message != nil {
		if eventMsg.Message.Data != nil {
			if eventMsg.Message.Data.Attachment != nil {
				if eventMsg.Message.Data.Attachment.Type == "media" {
					if eventMsg.Message.Data.Attachment.Media.MediaURL != "" {
						err = b.HandleSendAttachment(&rmsg, eventMsg.Message.SenderID, eventMsg.Message.Data.Attachment.Media.MediaURL)
						if err != nil {
							b.Log.Errorf("error while handling media upload: %s", err)
						}
					}
				}

			}
		}
	}
	b.Remote <- rmsg

}
func (b *Btwitter) RegisterParticipentInfo(participents map[string]twitterUser) {
	b.Lock()
	for k, v := range participents {
		b.UserMap[k] = v
	}
	b.Unlock()

}
func (b *Btwitter) handleDmEvents(eventMsg twitter.DirectMessageEvent) {
	senderInfo, ok := b.GetParticipentInfo(eventMsg.Message.SenderID)
	if !ok {
		return
	}

	text := eventMsg.Message.Data.Text
	if strings.Contains(eventMsg.Message.Data.Text, " https://t.co/") {
		text = strings.Split(eventMsg.Message.Data.Text, " https://t.co/")[0]
	}

	rmsg := config.Message{
		Text:     text,
		Channel:  eventMsg.Message.SenderID,
		Username: senderInfo.ScreenName,
		UserID:   eventMsg.Message.SenderID,
		Avatar:   "",
		Account:  b.Account,
		Event:    "direct_msg",
		Protocol: "twitter",

		ExtraNetworkInfo: config.ExtraNetworkInfo{
			ChannelUsersMember: []string{},
			ActionCommand:      "",
			ChannelId:          senderInfo.ID,
			ChannelName:        senderInfo.ScreenName,
			ChannelType:        "direct_msg",
			TargetPlatform:     "appservice",
			UsersMemberId:      map[string]string{senderInfo.ID: senderInfo.ScreenName},
			Mentions:           map[string]string{},
		},
	}

	if eventMsg.Message != nil {
		if eventMsg.Message.Data != nil {
			if eventMsg.Message.Data.Attachment != nil {
				if eventMsg.Message.Data.Attachment.Type == "media" {
					if eventMsg.Message.Data.Attachment.Media.MediaURL != "" {
						extraRmsg := rmsg
						extraRmsg.Text = ""
						err := b.HandleSendAttachment(&extraRmsg, eventMsg.Message.SenderID, eventMsg.Message.Data.Attachment.Media.MediaURL)
						if err != nil {
							b.Log.Errorf("Error in handleDmEvents: %s", err)
						}
						b.Remote <- extraRmsg
					}
				}

			}
		}

	}
	if rmsg.Text != "" {
		b.Remote <- rmsg
	}

}
