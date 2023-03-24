package bemail

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/mail"
	"strings"

	"github.com/42wim/matterbridge/bridge/config"
	"github.com/DusanKasan/parsemail"
)

func (b *Bemail) Send(msg config.Message) (string, error) {
	switch msg.Event {
	case "email-event":
		go b.HandleEmailEvent(msg)
		return "", nil
	}
	b.Log.Debugf("=> Receiving %#v", msg)
	return b.SendSmtp(msg)
}

type emailEvent struct {
	Base64MailBody string `json:"base64MailBody"`
}

func (b *Bemail) HandleEmailEvent(msg config.Message) {
	if msg.Extra == nil {
		b.Log.Debug("empty email events data")
		return
	}
	events, ok := msg.Extra["email-event"]
	if !ok {
		return
	}
	for _, v := range events {
		bf, err := json.Marshal(v)
		if err != nil {
			b.Log.Debug("failed to marshal email events data")
			continue
		}

		emailEvent := emailEvent{}
		err = json.Unmarshal(bf, &emailEvent)
		if err != nil {
			b.Log.Debug("failed to unmarshal email events data")
			continue

		}

		b64, err := base64.StdEncoding.DecodeString(emailEvent.Base64MailBody)
		if err != nil {
			b.Log.Debug("failed to  decode email events base64MailBody data")
			continue
		}
		reader := bufio.NewReader(bytes.NewReader(b64))
		_, _ = reader.ReadString('\n')

		// read the remaining lines
		emailRaw, err := io.ReadAll(reader)
		if err != nil {
			b.Log.Println(err)
			continue
		}
		strRead := bytes.NewReader(emailRaw)
		emailContent, err := parsemail.Parse(strRead)
		if err != nil {
			b.Log.Errorf("parse email body failed : %s", err)
			return

		}
		// mail.ParseAddress(emailContent.Header.Get("to"))
		sender, err := mail.ParseAddress(emailContent.Header.Get("From"))
		if err != nil {
			b.Log.Errorf("parse email sender address failed : %s", err)
			return
		}

		receiver, err := mail.ParseAddress(emailContent.Header.Get("To"))
		if err != nil {
			b.Log.Errorf("parse email receiver address failed : %s", err)
			return
		}

		if receiver.Address != b.AccountImapAddress {
			b.Log.Debugf("email receiver address is not match : %s", receiver.Address)
			return
		}
		senderName := sender.Name
		if senderName == "" {
			senderName = sender.Address
		}
		msg := config.Message{
			Text:     "",
			Channel:  sender.Address,
			Username: senderName,
			UserID:   sender.Address,
			Avatar:   "",
			Account:  b.Account,
			Event:    "direct_msg",
			Protocol: "email",
			ID:       "",
			ExtraNetworkInfo: config.ExtraNetworkInfo{
				ChannelId:      sender.Address,
				ChannelName:    senderName,
				TargetPlatform: "appservice",
			},
		}
		b.HandleEmailParsed(msg, emailContent)

	}
}
func getStringBetween(s string, start string, end string) string {
	startIndex := strings.Index(s, start)
	if startIndex == -1 {
		return s
	}
	startIndex += len(start)

	endIndex := strings.Index(s[startIndex:], end)
	if endIndex == -1 {
		return s
	}

	return s[startIndex : startIndex+endIndex]
}
