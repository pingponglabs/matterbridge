package binstagram

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/42wim/matterbridge/bridge"
	"github.com/42wim/matterbridge/bridge/config"
	"github.com/42wim/matterbridge/bridge/helper"
	"github.com/Davincible/goinsta"
)

type Binstagram struct {
	instaClient     *goinsta.Instagram
	UserID          string
	Groups          map[string]string
	GroupsLatestMsg map[string]string
	InstaAccount    account
	sync.RWMutex
	*bridge.Config
	syncMsg chan *goinsta.Conversation
}

type account struct {
	Id       string
	Username string
	Email    string
	FullName string
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

	b := &Binstagram{Config: cfg}
	b.syncMsg = make(chan *goinsta.Conversation)
	b.Groups = make(map[string]string)
	b.GroupsLatestMsg = make(map[string]string)
	return b
}

// params
//UserName ,Password ,TwoFactorCode , ImportProfilePath
func (b *Binstagram) LogWithProfilefiLe(path string) error {

	insta, err := goinsta.Import(path)
	if err != nil {
		return err
	}
	err = insta.OpenApp()
	if err != nil {
		return err
	}
	b.instaClient = insta
	return nil

}
func (b *Binstagram) LogWithUsername() error {
	username := b.GetString("UserName")
	password := b.GetString("Password")
	var insta *goinsta.Instagram

	if username == "" || password == "" {
		return fmt.Errorf("invalid username or password")
	}
	b.Log.Infof("Connecting %s account", b.GetString("UserName"))

	insta = goinsta.New(username, password)
	err := insta.Login()
	if err == goinsta.Err2FARequired {
		code := b.GetString("TwoFactorCode")
		err2FA := insta.TwoFactorInfo.Login2FA(code)
		if err2FA != nil {
			return err2FA
		}
	} else if err != nil {
		return err
	}

	b.instaClient = insta
	return nil
}
func (b *Binstagram) SyncMsg() {
	time.Sleep(10 * time.Second)

	for {
		time.Sleep(5 * time.Second)
		err := b.instaClient.Inbox.Sync()
		if err != nil {
			log.Println(err)
			continue
		}
		for _, group := range b.instaClient.Inbox.Conversations {
			b.syncMsg <- group
		}
	}
}
func (b *Binstagram) Connect() error {
	var err error
	importProfilePath := b.GetString("ImportProfilePath")

	if importProfilePath == "" {
		return fmt.Errorf("instagram import path not defined")
	}

	err = b.LogWithProfilefiLe(importProfilePath)
	if err != nil {
		log.Println(err)
		err = b.LogWithUsername()
		if err != nil {
			log.Println(err)
			return err
		}
	}

	// make this func
	b.InstaAccount = account{
		Id:       fmt.Sprintf("%v", b.instaClient.Account.ID),
		Username: b.instaClient.Account.Username,
		Email:    b.instaClient.Account.Email,
		FullName: b.instaClient.Account.FullName,
	}
	b.instaClient.Export(importProfilePath)
	go b.SendInfo()
	go b.SyncMsg()
	go b.handleSyncUpdates()
	b.Log.Info("Connection succeeded")

	return nil
}

func (b *Binstagram) SendInfo() {
	time.Sleep(time.Second * 10)
	for _, group := range b.instaClient.Inbox.Conversations {

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
			Text:     "new_users",
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
	}
}
func (b *Binstagram) HandleImport() {

}

func (b *Binstagram) Disconnect() error {
	return nil
}

func (b *Binstagram) JoinChannel(channel config.ChannelInfo) error {

	return nil

}
func (b *Binstagram) UploadMedia(image io.Reader) (*goinsta.UploadOptions, error) {

	return b.instaClient.UploadImage(
		&goinsta.UploadOptions{
			File: image,
		},
	)

}

func (b *Binstagram) Send(msg config.Message) (string, error) {
	b.Log.Debugf("=> Receiving %#v", msg)
	for _, group := range b.instaClient.Inbox.Conversations {
		if group.Title == msg.Channel {
			if msg.Extra != nil {
				err := b.handleUpload(group, &msg)
				return "", err
			}
			err := group.Send(msg.Text)
			if err != nil {
				return "", err
			}

			return "", nil
		}
	}
	return "", nil
}

func (b *Binstagram) handleSyncUpdates() {

	for group := range b.syncMsg {
		title := group.Title
		usersMapId := map[int64]string{}
		for _, user := range group.Users {
			usersMapId[user.ID] = user.FullName
		}
		b.RLock()
		latestMsId := b.GroupsLatestMsg[title]
		b.RUnlock()
		for _, item := range group.Items {
			if item.ID == latestMsId {
				break
			}
			if item.UserID == b.instaClient.Account.ID {
				continue
			}
			configMessage := config.Message{
				Text:     item.Text,
				Channel:  group.Title,
				Username: usersMapId[item.UserID],
				UserID:   b.InstaAccount.Id,
				Avatar:   "",
				Account:  b.Account,
				Protocol: "instagram",
			}
			if item.Type == "media" {
				configMessage.Text = item.Media.Images.GetBest()
			}
			b.Remote <- configMessage
		}
		b.Lock()
		b.GroupsLatestMsg[title] = group.LastPermanentItem.ID
		b.Unlock()
	}
}

func (b *Binstagram) handleEdit(rmsg config.Message) bool {

	return true
}

func (b *Binstagram) handleReply(rmsg config.Message) bool {

	return true
}

func (b *Binstagram) handleMemberChange() {
	// Update the displayname on join messages, according to https://matrix.org/docs/spec/client_server/r0.6.1#events-on-change-of-profile-information

}

// handleDownloadFile handles file download
func (b *Binstagram) handleUpload(c *goinsta.Conversation, msg *config.Message) error {

	if msg.Extra == nil {
		return fmt.Errorf("nil extra map")
	}
	for _, rmsg := range helper.HandleExtra(msg, b.General) {
		b.Log.Println(rmsg.Text)

	}
	for _, f := range msg.Extra["file"] {
		if fi, ok := f.(config.FileInfo); ok {
			content := bytes.NewReader(*fi.Data)
			u, err := b.UploadMedia(content)
			if err != nil {
				b.Log.Println(err)
			}
			err = c.ConfigureDMImagePost(u.GetuploadId(), u.GetuploadName())
			if err != nil {
				b.Log.Println(err)
			}
		}
	}
	return nil
}

// handleUploadFiles handles native upload of files.
func (b *Binstagram) handleUploadImage(msg *config.Message) (string, error) {

	return "", nil
}
