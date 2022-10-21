package bfacebookbusiness

import (
	"bytes"
	"encoding/json"
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

type BfacebookBusiness struct {
	UserID   string
	Username string
	Token    string
	Accounts []Account

	SendMessageIdList map[string]bool

	Coversations    []ConversationInfo
	GroupsLatestMsg map[string]time.Time
	sync.RWMutex
	*bridge.Config
	syncMsg chan MessageData
}

type Account struct {
	pageAccessToken string
	pageID          string
	pageName        string

	intagramBusinessAccount string
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

// me/accounts?fields=instagram_business_account
func New(cfg *bridge.Config) bridge.Bridger {

	b := &BfacebookBusiness{Config: cfg}
	b.syncMsg = make(chan MessageData)
	b.Coversations = []ConversationInfo{}
	b.GroupsLatestMsg = make(map[string]time.Time)
	b.SendMessageIdList = make(map[string]bool)
	return b
}
func (b *BfacebookBusiness) Connect() error {
	token := b.GetString("Token")
	b.Token = token
	res, err := GetAccountInfo(token)
	if err != nil {
		return err
	}

	b.UserID = res.ID
	b.Username = res.Name
	for _, v := range res.Accounts.Data {
		b.Accounts = append(b.Accounts, Account{
			pageAccessToken:         v.AccessToken,
			pageID:                  v.ID,
			pageName:                v.Name,
			intagramBusinessAccount: v.InstagramBusinessAccount.ID,
		})

	}

	ConversationRes, err := GetMessages(b.Accounts[0].pageAccessToken, b.Accounts[0].pageID, "")
	if err != nil {
		return err
	}
	ConversationResInsta, err := GetMessages(b.Accounts[0].pageAccessToken, b.Accounts[0].pageID, "instagram")
	if err != nil {
		return err
	}

	channelsInfo := b.ParseConversation(ConversationRes)
	channlsInstaInfo := b.ParseConversation(ConversationResInsta)
	channelsInfo = append(channelsInfo, channlsInstaInfo...)

	b.Coversations = channelsInfo
	go b.InitUsers(channelsInfo)
	//go b.SyncMsg()
	//go b.handleSyncUpdates()
	return nil
}

type ConversationInfo struct {
	Name           string
	ID             string
	Participents   map[string]SenderInfo
	PageSender     SenderInfo
	CustomerSender SenderInfo
	ChannelsUsers  []SenderInfo
}

func (b *BfacebookBusiness) ParseConversation(conversation MessagesResp) []ConversationInfo {
	res := []ConversationInfo{}
	for _, conversation := range conversation.Data {
		convInfo := ConversationInfo{
			Name:           "",
			ID:             conversation.ID,
			Participents:   map[string]SenderInfo{},
			PageSender:     SenderInfo{},
			CustomerSender: SenderInfo{},
			ChannelsUsers:  conversation.Participants.Data,
		}
		for _, v := range conversation.Participants.Data {
			if v.Name == "" {
				v.Name = v.Username
			}
			convInfo.Participents[v.ID] = v
			if v.ID == b.Accounts[0].pageID {
				convInfo.PageSender = v
				continue
			}
			convInfo.CustomerSender = v
			convInfo.Name = v.Name

		}
		res = append(res, convInfo)
	}
	return res

}
func (b *BfacebookBusiness) InitUsers(convsInfo []ConversationInfo) {

	time.Sleep(time.Second * 10)
	for _, ch := range convsInfo {
		users := []string{}
		usersId := map[string]string{}

		for _, v := range ch.Participents {
			users = append(users, v.Name)
			usersId[v.Name] = v.ID
		}

		configMessage := config.Message{
			Text:     "test",
			Channel:  ch.Name,
			UserID:   b.UserID,
			Avatar:   "",
			Account:  b.Account,
			Event:    "new_users",
			Protocol: "facebookbusiness",
			ExtraNetworkInfo: config.ExtraNetworkInfo{
				ChannelUsersMember: users,
				ActionCommand:      "new_users",
				ChannelId:          ch.ID,
				ChannelName:        ch.ID,
				ChannelType:        "",
				TargetPlatform:     "appservice",
				UsersMemberId:      usersId,
				Mentions:           map[string]string{},
			},
		}
		b.Remote <- configMessage
	}
}

func (b *BfacebookBusiness) SyncMsg() {
	time.Sleep(10 * time.Second)

	for {
		time.Sleep(10 * time.Second)
		ConversationRes, err := GetMessages(b.Accounts[0].pageAccessToken, b.Accounts[0].pageID, "")
		if err != nil {
			log.Println(err)
			continue
		}
		for _, converstation := range ConversationRes.Data {
			b.syncMsg <- converstation
		}
		time.Sleep(5 * time.Minute)
	}
	//webhook listening on ports ,
}

func (b *BfacebookBusiness) HandleImport() {

}

func (b *BfacebookBusiness) Disconnect() error {
	return nil
}

func (b *BfacebookBusiness) JoinChannel(channel config.ChannelInfo) error {

	return nil

}
func (b *BfacebookBusiness) UploadMedia(image io.Reader) (*goinsta.UploadOptions, error) {

	return nil, nil

}

func (b *BfacebookBusiness) Send(msg config.Message) (string, error) {
	b.Log.Debugf("=> Receiving %#v", msg)
	switch msg.Event {
	case "facebook-event":
		err := b.HandleFacebookEvent(msg)
		return "", err
	}
	for _, conversation := range b.Coversations {
		if conversation.Name == msg.Channel {
			msgID, err := b.SendMessage(conversation.CustomerSender.ID, msg.Text)
			if err != nil {
				return "", err
			}
			b.Lock()
			b.SendMessageIdList[msgID] = true
			b.Unlock()
		}
	}
	return "", nil
}
func (b *BfacebookBusiness) HandleFacebookEvent(Msg config.Message) error {
	if Msg.Extra == nil {
		return fmt.Errorf("empty facebook events data")
	}
	if events, ok := Msg.Extra["facebook-event"]; ok {
		for _, v := range events {
			var event event
			ev, err := json.Marshal(v)
			if err != nil {
				log.Println(err)
				continue
			}
			err = json.Unmarshal(ev, &event)
			if err != nil {
				log.Println(err)
				continue
			}
			var TargetId string
			switch event.Object {
			case "page":
				TargetId = b.Accounts[0].pageID
			case "instagram":
				TargetId = b.Accounts[0].intagramBusinessAccount
			default:
				return nil
			}
			for _, entry := range event.Entry {
				if entry.ID == TargetId {
					for _, v := range entry.Messaging {
						if v.Message.Text == "" || b.IsMessageDuplicate(v.Message.Mid) {
							continue
						}
						for _, cv := range b.Coversations {
							if _, ok := cv.Participents[v.Recipient.ID]; ok {
								if vv, ok := cv.Participents[v.Sender.ID]; ok {
									configMessage := config.Message{
										Text:     v.Message.Text,
										Channel:  cv.Name,
										Username: vv.Name,
										UserID:   vv.ID,
										Avatar:   "",
										Account:  b.Account,
										Protocol: "facebookbusiness",
									}
									go func() {
										time.Sleep(2 * time.Second)
										b.Remote <- configMessage

									}()
								}
							}
						}
					}
				}
			}
		}
	}

	return nil
}
func (b *BfacebookBusiness) handleSyncUpdates() {

	for msgData := range b.syncMsg {
		title := msgData.ID
		var recentMsg time.Time

		b.RLock()
		recentMsg = b.GroupsLatestMsg[title]
		b.RUnlock()
		for _, item := range msgData.Messages.Data {
			parsedCreatedTime := parseCreatedTime(item.CreatedTime)
			if parsedCreatedTime.After(recentMsg) {
				recentMsg = parsedCreatedTime
			}
			if parsedCreatedTime.Before(recentMsg) {
				break
			}
			if item.From.ID == b.UserID {
				continue
			}
			configMessage := config.Message{
				Text:     item.Message,
				Channel:  title,
				Username: item.From.Name,
				UserID:   item.From.ID,
				Avatar:   "",
				Account:  b.Account,
				Protocol: "facebook-business",
			}

			b.Remote <- configMessage
		}
		b.Lock()
		b.GroupsLatestMsg[title] = recentMsg
		b.Unlock()
	}
}

func (b *BfacebookBusiness) handleEdit(rmsg config.Message) bool {

	return true
}

func (b *BfacebookBusiness) handleReply(rmsg config.Message) bool {

	return true
}

func (b *BfacebookBusiness) handleMemberChange() {
	// Update the displayname on join messages, according to https://matrix.org/docs/spec/client_server/r0.6.1#events-on-change-of-profile-information

}

// handleDownloadFile handles file download
func (b *BfacebookBusiness) handleUpload(c *goinsta.Conversation, msg *config.Message) error {

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
func (b *BfacebookBusiness) handleUploadImage(msg *config.Message) (string, error) {

	return "", nil
}

func (b *BfacebookBusiness) IsMessageDuplicate(mid string) bool {
	b.Lock()
	defer b.Unlock()
	if v, ok := b.SendMessageIdList[mid]; ok {
		if v {
			delete(b.SendMessageIdList, mid)
			return true
		}
	}
	return false
}
