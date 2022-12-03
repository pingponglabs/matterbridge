package btwitter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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

type group struct {
	Id    string
	Title string
}

// SubTextMessage represents the new content of the message in edit messages.
type SubTextMessage struct {
	MsgType       string `json:"msgtype"`
	Body          string `json:"body"`
	FormattedBody string `json:"formatted_body,omitempty"`
	Format        string `json:"format,omitempty"`
}

// MessageRelation explains how the current message relates to a previous message.
// Notably used for message edits.
type MessageRelation struct {
	EventID string `json:"event_id"`
	Type    string `json:"rel_type"`
}

type EditedMessage struct {
	NewContent SubTextMessage  `json:"m.new_content"`
	RelatedTo  MessageRelation `json:"m.relates_to"`
}

type InReplyToRelationContent struct {
	EventID string `json:"event_id"`
}

type InReplyToRelation struct {
	InReplyTo InReplyToRelationContent `json:"m.in_reply_to"`
}

type ReplyMessage struct {
	RelatedTo InReplyToRelation `json:"m.relates_to"`
}

func New(cfg *bridge.Config) bridge.Bridger {

	b := &Btwitter{Config: cfg}
	b.Groups = make(map[string]string)
	b.GroupsLatestMsg = make(map[string]string)
	b.syncMsg = make(chan twitter.DirectMessageEvent)
	b.UserMap = make(map[string]twitterUser)
	return b
}

func (b *Btwitter) SyncMsg() {
	time.Sleep(10 * time.Second)

	for {
		time.Sleep(30 * time.Second)
		dmEvent, err := b.GetDmEvent(2)
		if err != nil {
			b.Log.Println(err)
			continue
		}
		for _, v := range dmEvent.Events {
			timeStamp, _ := strconv.Atoi(v.CreatedAt)
			if timeStamp > b.LastMsgTimeStamp {
				b.LastMsgTimeStamp = timeStamp
			} else {
				break
			}
			if v.Message.SenderID != b.AccountInfo.ID {
				b.syncMsg <- v
				// send to external network
			}
		}
	}
}
func (b *Btwitter) Connect() error {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
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

	// b.RatelimitStatus()
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
	/*
		dmEvent, err := b.GetDmEvent(1)
		if err != nil {
			log.Println(err)
			return err
		}
		b.parseLatestTimestamp(dmEvent)
	*/
	// go b.SyncMsg()
	// go b.handleSyncUpdates()
	return nil
}
func (b *Btwitter) parseLatestTimestamp(ev *twitter.DirectMessageEvents) {
	for _, v := range ev.Events {
		timeStamp, _ := strconv.Atoi(v.CreatedAt)

		if timeStamp > b.LastMsgTimeStamp {
			b.LastMsgTimeStamp = timeStamp
			return
		}
	}
}
func (b *Btwitter) ProcessDmInfo(ev *twitter.DirectMessageEvents) {

}

func (b *Btwitter) Reserve() {
	// Twitter client
	client := twitter.NewClient(b.client)

	// Home Timeline
	tweets, resp, err := client.DirectMessages.EventsList(&twitter.DirectMessageEventsListParams{
		Cursor: "",
		Count:  0,
	})
	_, _, _ = tweets, resp, err

	// Send a Tweet
	newtweet, resp, err := client.DirectMessages.EventsNew(&twitter.DirectMessageEventsNewParams{
		Event: &twitter.DirectMessageEvent{
			CreatedAt: "",
			ID:        "",
			Type:      "",
			Message: &twitter.DirectMessageEventMessage{
				SenderID: "",
				Target: &twitter.DirectMessageTarget{
					RecipientID: "",
				},
				Data: &twitter.DirectMessageData{
					Text: "",
					Entities: &twitter.Entities{
						Hashtags:     []twitter.HashtagEntity{},
						Media:        []twitter.MediaEntity{},
						Urls:         []twitter.URLEntity{},
						UserMentions: []twitter.MentionEntity{},
						Symbols:      []twitter.SymbolEntity{},
						Polls:        []twitter.PollEntity{},
					},
					Attachment: &twitter.DirectMessageDataAttachment{},
					QuickReply: &twitter.DirectMessageQuickReply{},
					CTAs:       []twitter.DirectMessageCTA{},
				},
			},
		},
	})
	_, _, _ = newtweet, resp, err

	// Status Show
	/*
		tweet, resp, err := client.Show(585613041028431872, nil)

		// Search Tweets
		search, resp, err := client.Search.Tweets(&twitter.SearchTweetParams{
			Query: "gopher",
		})

		// User Show
		user, resp, err := client.Users.Show(&twitterUserShowParams{
			ScreenName: "dghubble",
		})

		// Followers
		followers, resp, err := client.Followers.List(&twitter.FollowerListParams{})
	*/
}
func (b *Btwitter) SendInfo() {
	time.Sleep(time.Second * 10)
	for _, group := range b.Groups {
		log.Println(group)
		/*
			userMemberId := map[string]string{}
			users := []string{}
			for _, user := range group.Users {
				userMemberId[user.Username] = user.FullName

				users = append(users, user.FullName)
			}
			b.Lock()
			b.GroupsLatestMsg[group.Title] = group.LastPermanentItem.ID
			b.Unlock()
			configMessage := config.Message{
				Text:     "test",
				Channel:  group.Title,
				Username: b.InstaAccount.Username,
				UserID:   b.InstaAccount.Id,
				Avatar:   "",
				Account:  b.Account,
				Event:    "new_users",
				Protocol: "instagram",
				ExtraNetworkInfo: config.ExtraNetworkInfo{
					ChannelUsersMember: users,
					ActionCommand:      "new_users",
					ChannelId:          group.ID,
					ChannelName:        group.Title,
					ChannelType:        group.ThreadType,
					TargetPlatform:     "appservice",
					UsersMemberId:      userMemberId,
					Mentions:           map[string]string{},
				},
			}
			b.Remote <- configMessage
		*/
	}

}
func (b *Btwitter) HandleImport() {

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
			b.Log.Debug(respSendEvent)
		}
	}
	return nil
}
func (b *Btwitter) SendText(msg *config.Message) error {
	respSendEvent, err := b.SetDmEvent(msg.Channel, msg.Text, 0)
	if err != nil {
		return err
	}
	b.Log.Debug(respSendEvent)
	return nil
}

func (b *Btwitter) Send(msg config.Message) (string, error) {
	b.Log.Debugf("=> Receiving %#v", msg)

	switch msg.Event {

	case "twitter-event":
		// TODO gorotune
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
		log.Println(fmt.Errorf("empty twitter events data"))
	}
	if events, ok := Msg.Extra["twitter-event"]; ok {

		for _, v := range events {

			var event webhookEvent
			bb, err := json.Marshal(v)
			if err != nil {
				log.Println(err)
				continue
			}
			err = json.Unmarshal(bb, &event)
			if err != nil {
				log.Println(err)
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

func (b *Btwitter) handleSyncUpdates() {
	var err error
	for eventMsg := range b.syncMsg {
		senderInfo, ok := b.GetParticipentInfo(eventMsg.Message.SenderID)
		if !ok {
			senderInfo, err = b.FetchParticipentInfo(eventMsg.Message.SenderID)
			if err != nil {
				b.Log.Println(err)
				continue
			}
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
							err = b.HandleSendAttachment(&extraRmsg, eventMsg.Message.SenderID, eventMsg.Message.Data.Attachment.Media.MediaURL)
							if err != nil {
								b.Log.Println(err)
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

func (b *Btwitter) handleEdit(rmsg config.Message) bool {

	return true
}

func (b *Btwitter) handleReply(rmsg config.Message) bool {

	return true
}

func (b *Btwitter) handleMemberChange() {
	// Update the displayname on join messages, according to https://matrix.org/docs/spec/client_server/r0.6.1#events-on-change-of-profile-information

}

// handleDownloadFile handles file download

// handleUploadFiles handles native upload of files.
func (b *Btwitter) handleUploadImage(msg *config.Message) (string, error) {

	return "", nil
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
							b.Log.Println(err)
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
		b.Log.Println("TODO")
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
							b.Log.Println(err)
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
