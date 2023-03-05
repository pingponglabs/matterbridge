package bfacebookbusiness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/42wim/matterbridge/bridge/config"
	fb "github.com/huandu/facebook/v2"
)

type AccountResp struct {
	ID       string   `json:"id,omitempty"`
	Name     string   `json:"name,omitempty"`
	Accounts Accounts `json:"accounts,omitempty"`
}
type Accounts struct {
	Data   []AccountData `json:"data"`
	Paging Paging        `json:"paging"`
}
type AccountData struct {
	AccessToken  string         `json:"access_token"`
	Category     string         `json:"category"`
	CategoryList []CategoryList `json:"category_list"`
	Name         string         `json:"name"`
	ID           string         `json:"id"`
	Tasks        []string       `json:"tasks"`

	InstagramBusinessAccount struct {
		ID string `json:"id"`
	} `json:"instagram_business_account"`
}
type Paging struct {
	Cursors struct {
		Before string `json:"before"`
		After  string `json:"after"`
	} `json:"cursors"`
}

type CategoryList struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type MessagesResp struct {
	Data   []MessageData `json:"data"`
	Paging Paging        `json:"paging"`
}

type MessageData struct {
	ID           string      `json:"id"`
	Senders      SendersData `json:"senders,omitempty"`
	Participants SendersData `json:"participants,omitempty"`

	Messages MessagesInfo `json:"messages,omitempty"`
}
type SendersData struct {
	Data []SenderInfo `json:"data"`
}
type MessagesInfo struct {
	Data   []Message `json:"data"`
	Paging Paging    `json:"paging"`
}
type Message struct {
	Message     string     `json:"message"`
	ID          string     `json:"id"`
	From        SenderInfo `json:"from"`
	CreatedTime string     `json:"created_time"`
}
type SenderInfo struct {
	Name     string `json:"name"`
	Username string `json:"username,omitempty"` // instagram platform
	Email    string `json:"email,omitempty"`
	ID       string `json:"id"`
	Platform string `json:"-"`
}

type SendRecipientJson struct {
	ID string `json:"id,omitempty"`
}

type SendMessageJson struct {
	Text       string      `json:"text,omitempty"`
	Attachment *Attachment `json:"attachment,omitempty"`
}

type SendMediaJson struct {
}

type MessageSendResp struct {
	RecipientID string `json:"recipient_id,omitempty"`
	MessageID   string `json:"message_id,omitempty"`
}

type sendParamsRaw struct {
	link        string
	recipientID SendRecipientJson
	sendMessage SendMessageJson
}

type event struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}
type Entry struct {
	ID         string       `json:"id"`
	Time       int64        `json:"time"`
	Messaging  []Messaging  `json:"messaging"`
	HopContext []HopContext `json:"hop_context"`
}
type HopContext struct {
	AppID    int64  `json:"app_id"`
	Metadata string `json:"metadata"`
}
type Messaging struct {
	Message   EventMessage `json:"message"`
	Timestamp int64        `json:"timestamp,omitempty"`
	Sender    Sender       `json:"sender,omitempty"`
	Recipient Recipient    `json:"recipient"`
}
type EventMessage struct {
	Mid         string       `json:"mid,omitempty"`
	IsEcho      bool         `json:"is_echo,omitempty"`
	Text        string       `json:"text,omitempty"`
	AppID       int64        `json:"app_id,omitempty"`
	Attachments []Attachment `json:"attachments"`
}

type Sender struct {
	ID string `json:"id"`
}
type Recipient struct {
	ID string `json:"id"`
}

func GetAccountInfo(token string) (AccountResp, error) {

	res, err := fb.Get("me", fb.Params{
		"fields":       "accounts{instagram_business_account,access_token,name,id},name,id",
		"access_token": token,
	})
	if err != nil {
		return AccountResp{}, err
	}
	a := AccountResp{}
	b, err := json.Marshal(res)
	if err != nil {
		return AccountResp{}, err
	}
	err = json.Unmarshal(b, &a)
	if err != nil {
		return AccountResp{}, err
	}
	log.Println(res)
	return a, nil
}

func sliceContains(elems []string, v string) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}

func GetMessages(token, pageId, platform, userID string) (MessagesResp, error) {
	if platform == "facebook" {
		platform = ""
	}
	req := fmt.Sprintf("%s/conversations", pageId)
	res, err := fb.Get(req, fb.Params{
		"fields":       "participants,senders,messages{message,from,created_time},id",
		"platform":     platform,
		"user_id":      userID,
		"access_token": token,
	})
	if err != nil {
		return MessagesResp{}, err
	}
	a := MessagesResp{}
	b, err := json.Marshal(res)
	if err != nil {
		return MessagesResp{}, err
	}
	err = json.Unmarshal(b, &a)
	if err != nil {
		return MessagesResp{}, err
	}
	return a, nil
}

func (b *BfacebookBusiness) PrepareSendParams(accountInfo *Account, msg config.Message, participant SenderInfo) []sendParamsRaw {
	sendParams := []sendParamsRaw{}
	recipientParam := SendRecipientJson{ID: participant.ID}

	if msg.Extra == nil {
		sendMessageParam := SendMessageJson{Text: msg.Text}
		sendParams = append(sendParams, sendParamsRaw{
			link:        "/me/messages",
			recipientID: recipientParam,
			sendMessage: sendMessageParam,
		})
		return sendParams
	}
	if msg.Protocol == "appservice" {
		msg.Text = ""
	}
	for _, f := range msg.Extra["file"] {
		if fi, ok := f.(config.FileInfo); ok {
			content := bytes.NewReader(*fi.Data)
			_, typ := GetMediaTypeInfo(fi.Name)

			switch participant.Platform {
			case "facebook":
				resp, err := b.MediaUpload(accountInfo, content, fi.Name, typ)
				if err != nil {
					b.Log.Errorf("Failed to upload media: %v", err)
					continue
				}
				sendMessageParam := SendMessageJson{
					Attachment: &Attachment{
						Type: typ,
						Payload: Payload{
							AttachmentID: resp.AttachmentID,
						},
					},
				}
				sendParams = append(sendParams, sendParamsRaw{
					link:        "/me/messages",
					recipientID: recipientParam,
					sendMessage: sendMessageParam,
				})
			case "instagram":
				sendMessageParam := SendMessageJson{
					Attachment: &Attachment{
						Type: typ,
						Payload: Payload{
							URL: fi.URL,
						},
					},
				}
				sendParams = append(sendParams, sendParamsRaw{
					link:        fmt.Sprintf("%s/messages", accountInfo.pageID),
					recipientID: recipientParam,
					sendMessage: sendMessageParam,
				})
			}

		}
	}
	return sendParams
}

func GetMediaTypeInfo(name string) (string, string) {
	mediaExt := ""
	typ := ""
	if sl := strings.Split(name, "."); len(sl) > 1 {
		mediaExt = sl[len(sl)-1]
	}
	switch mediaExt {
	case "jpg", "png", "jpeg":
		typ = "image"
	}
	return mediaExt, typ
}

func (b *BfacebookBusiness) SendMessage(accountInfo *Account, params sendParamsRaw) (string, error) {

	recpt, err := json.Marshal(params.recipientID)
	if err != nil {
		return "", err
	}
	message, err := json.Marshal(params.sendMessage)
	if err != nil {
		return "", err
	}
	res, err := fb.Post(params.link, fb.Params{
		"recipient": string(recpt), "message": string(message),
		"access_token": accountInfo.pageAccessToken,
	})
	if err != nil {
		return "", err
	}
	if v, ok := res["message_id"]; ok {
		if msgId, ok := v.(string); ok {
			return msgId, nil
		}
	}
	return "", fmt.Errorf("empty facebook send messageId")
}
func (b *BfacebookBusiness) SendInstagramMediaMessage(accountInfo Account, name, recipientID, url string) (string, error) {
	mediaExt := ""
	mediaType := ""
	if sl := strings.Split(name, "."); len(sl) > 1 {
		mediaExt = sl[len(sl)-1]
	}

	switch mediaExt {
	case "jpg", "png", "jpeg":
		mediaType = "image"
	}
	fbUrl := fmt.Sprintf("%s/messages", accountInfo.pageID)
	RecipientParams := &SendRecipientJson{ID: recipientID}
	SendImageParam := &SendMessageJson{
		Attachment: &Attachment{
			Type: mediaType,
			Payload: Payload{
				URL: url,
			},
		},
	}

	recpt, err := json.Marshal(RecipientParams)
	if err != nil {
		return "", err
	}
	message, err := json.Marshal(SendImageParam)
	if err != nil {
		return "", err
	}
	res, err := fb.Post(fbUrl, fb.Params{
		"recipient": string(recpt), "message": string(message),
		"access_token": accountInfo.pageAccessToken,
	})
	if err != nil {
		return "", err
	}
	if v, ok := res["message_id"]; ok {
		if msgId, ok := v.(string); ok {
			return msgId, nil
		}
	}
	return "", fmt.Errorf("empty facebook send messageId")
}

func parseCreatedTime(stime string) (time.Time, error) {
	layout := "2006-01-02T15:04:05+0000"
	t, err := time.Parse(layout, stime)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}
