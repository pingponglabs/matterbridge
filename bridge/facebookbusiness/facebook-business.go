package bfacebookbusiness

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/42wim/matterbridge/bridge"
	"github.com/42wim/matterbridge/bridge/config"
	"github.com/42wim/matterbridge/bridge/helper"
)

type BfacebookBusiness struct {
	UserID   string
	Username string
	Token    string
	Accounts []*Account

	SendMessageIdList map[string]bool

	sync.RWMutex
	*bridge.Config
	syncMsg chan MessageData
}

type Account struct {
	pageAccessToken string
	pageID          string
	pageName        string

	intagramBusinessAccount string

	Participents map[string]SenderInfo
}

func New(cfg *bridge.Config) bridge.Bridger {

	b := &BfacebookBusiness{Config: cfg}
	b.syncMsg = make(chan MessageData)
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
	pageList := b.GetStringSlice("PagesNames")
	for _, v := range res.Accounts.Data {
		if sliceContains(pageList, v.Name) {
			b.Accounts = append(b.Accounts, &Account{
				pageAccessToken:         v.AccessToken,
				pageID:                  v.ID,
				pageName:                v.Name,
				intagramBusinessAccount: v.InstagramBusinessAccount.ID,
				Participents:            map[string]SenderInfo{},
			})
		}

	}
	return nil
}

type ConversationInfo struct {
	Name           string
	ID             string
	Participents   map[string]SenderInfo
	PageSender     SenderInfo
	CustomerSender SenderInfo
	Platform       string
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
			Platform:       "facebook",
			ChannelsUsers:  conversation.Participants.Data,
		}
		for _, v := range conversation.Participants.Data {
			v.Platform = "facebook"

			if v.Name == "" {
				v.Name = v.Username
				convInfo.Platform = "instagram"
				v.Platform = "instagram"
			}
			convInfo.Participents[v.ID] = v
			if b.MainAccountId(v.ID) {
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
			Text:     "new_users",
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



func (b *BfacebookBusiness) Disconnect() error {
	return nil
}

func (b *BfacebookBusiness) JoinChannel(channel config.ChannelInfo) error {

	return nil

}

func (b *BfacebookBusiness) ParseSenderID(platformID string) (string, string, string) {
	sl := strings.Split(platformID, "__")
	if len(sl) > 2 {
		return sl[0], sl[1], sl[2]
	}
	return platformID, "", ""
}

func (b *BfacebookBusiness) Send(msg config.Message) (string, error) {
	b.Log.Debugf("=> Receiving %#v", msg)
	switch msg.Event {
	case "facebook-event":
		go b.HandleFacebookEvent(msg)
		return "", nil
	}

	var SendErr error
	accountId, recipientID, platform := b.ParseSenderID(msg.ChannelId)

	accountInfo, err := b.GetAccountInfo(accountId)
	if err != nil {
		return "", err
	}

	senderInfo, ok := accountInfo.Participents[recipientID]
	if !ok {

		senderInfo, err = b.RegisterParticipentInfo(accountId, recipientID, platform)
		if err != nil {
			return "", err
		}

	}
	if senderInfo.ID == recipientID {
		sendparams := b.PrepareSendParams(accountInfo, msg, senderInfo)
		for _, param := range sendparams {
			msgID, err := b.SendMessage(accountInfo, param)
			if err != nil {
				log.Println(err)
				SendErr = err
				continue
			}
			b.Lock()
			b.SendMessageIdList[msgID] = true
			b.Unlock()
		}
		return "", SendErr
	}

	return "", SendErr
}
func (b *BfacebookBusiness) HandleFacebookEvent(Msg config.Message) {
	time.Sleep(50 * time.Millisecond)
	if Msg.Extra == nil {
		log.Println(fmt.Errorf("empty facebook events data"))
	}
	events, ok := Msg.Extra["facebook-event"]
	if !ok {
		return
	}
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
		var platform string
		switch event.Object {
		case "page":
			platform = "facebook"
		case "instagram":
			platform = "instagram"
		default:
			return
		}
		for _, entry := range event.Entry {
			if !b.MainAccountId(entry.ID) {
				continue
			}
			accountInfo, err := b.GetAccountInfo(entry.ID)
			if err != nil {
				continue
			}

			for _, msgEvent := range entry.Messaging {

				if b.IsMessageDuplicate(msgEvent.Message.Mid) {
					continue
				}
				senderInfo, err := b.RegisterParticipentInfo(entry.ID, msgEvent.Sender.ID, event.Object)
				if err != nil {
					b.Log.Errorf(err.Error())
					continue
				}

				configMessage := config.Message{
					Text:     "",
					Channel:  accountInfo.pageName + "_" + platform + "_" + senderInfo.Name,
					Username: accountInfo.pageName + "_" + platform + "_" + senderInfo.Name,
					UserID:   entry.ID + "__" + senderInfo.ID + "__" + platform,
					Account:  b.Account,
					Event:    "direct_msg",
					Protocol: "facebookbusiness",
					ID:       entry.ID,
					ExtraNetworkInfo: config.ExtraNetworkInfo{
						ChannelUsersMember: []string{},
						ChannelId:          entry.ID + "__" + senderInfo.ID + "__" + platform,
						ChannelName:        senderInfo.Name,
					},
				}
				if msgEvent.Message.Text != "" {
					configMessage.Text = msgEvent.Message.Text
					b.Remote <- configMessage
				}
				if len(msgEvent.Message.Attachments) > 0 {
					configMessage.Text = ""
					b.HandleMediaEvent(&configMessage, event.Object, msgEvent)
					b.Remote <- configMessage
				}
			}
		}
	}
}

func (b *BfacebookBusiness) RegisterParticipentInfo(accountID, senderId, eventObject string) (SenderInfo, error) {
	accountInfo, err := b.GetAccountInfo(accountID)
	if err != nil {
		return SenderInfo{}, err
	}

	participient, ok := accountInfo.Participents[senderId]
	if ok {
		return participient, nil
	}

	var platform string
	switch eventObject {
	case "page", "facebook":
		platform = "facebook"
	case "instagram":
		platform = "instagram"
	default:
		return SenderInfo{}, fmt.Errorf("unknown platform")
	}
	if b.MainAccountId(senderId) {
		return SenderInfo{}, fmt.Errorf("sender is the main bot account")
	}

	ConversationRes, err := GetMessages(accountInfo.pageAccessToken, accountInfo.pageID, platform, senderId)
	if err != nil {
		return SenderInfo{}, err
	}

	converInfo := b.ParseConversation(ConversationRes)
	for _, conv := range converInfo {

		for k, v := range conv.Participents {
			if !b.MainAccountId(k) {
				b.Lock()
				defer b.Unlock()

				accountInfo.Participents[k] = v
				return v, nil
			}
		}
	}
	return SenderInfo{}, fmt.Errorf("sender %s not found", senderId)
}

func (b *BfacebookBusiness) HandleMediaEvent(rmsg *config.Message, platform string, msgEvent Messaging) {
	rmsg.Extra = make(map[string][]interface{})

	for _, attachement := range msgEvent.Message.Attachments {
		img, err := b.MediaDownload(attachement.Payload.URL)
		if err != nil {
			log.Println(err)
			continue
		}
		var name string
		switch platform {
		case "page":

			sl := strings.Split(attachement.Payload.URL, "/")
			base := strings.Split(sl[len(sl)-1], "?")
			name = base[0]
		case "instagram":
			sl := strings.Split(http.DetectContentType(img[:512]), "/")
			if len(sl) > 1 {
				name = msgEvent.Message.Mid + "." + sl[1]
			} else {
				name = msgEvent.Message.Mid + "." + sl[0]
			}
		}
		helper.HandleDownloadData(b.Log, rmsg, name, "", attachement.Payload.URL, &img, b.General)

	}
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
func (b *BfacebookBusiness) MainAccountId(id string) bool {
	for _, v := range b.Accounts {
		if v.pageID == id || v.intagramBusinessAccount == id {
			return true
		}
	}
	return false
}
func (b *BfacebookBusiness) GetAccountInfo(id string) (*Account, error) {
	for _, v := range b.Accounts {
		if v.pageID == id || v.intagramBusinessAccount == id {
			return v, nil
		}
	}
	return nil, fmt.Errorf("account %s not found", id)
}
