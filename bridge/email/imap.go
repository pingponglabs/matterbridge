package bemail

import (
	"io"
	"log"
	"mime"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/42wim/matterbridge/bridge"
	"github.com/42wim/matterbridge/bridge/config"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-message/mail"

	"github.com/emersion/go-imap/client"
)

type Bemail struct {
	sync.RWMutex
	*bridge.Config
	ProcessedEmailMsgID []string
	LastFetchTimeStamp  time.Time
	*client.Client
	Inboxes []*imap.MailboxInfo
}

func New(cfg *bridge.Config) bridge.Bridger {
	b := &Bemail{Config: cfg}
	return b
}

func (b *Bemail) HandleIncomingEmail(emailMsg *imap.Message) {
	for _, from := range emailMsg.Envelope.From {
		username := from.PersonalName
		if username == "" {
			username = from.MailboxName
		}
		msg := config.Message{
			Text:     "",
			Channel:  from.MailboxName + "@" + from.HostName,
			Username: from.PersonalName,
			UserID:   from.MailboxName + "@" + from.HostName,
			Avatar:   "",
			Account:  b.Account,
			Event:    "direct_msg",
			Protocol: "email",
			ID:       "",
			ExtraNetworkInfo: config.ExtraNetworkInfo{
				ChannelId:      from.MailboxName + "@" + from.HostName,
				ChannelName:    username,
				TargetPlatform: "appservice",
			},
		}
		b.HandleEmailContent(msg, emailMsg.Body)
	}
}
func (b *Bemail) HandleEmailContent(msg config.Message, body map[*imap.BodySectionName]imap.Literal) {
	for section, bodyPart := range body {
		b.Log.Info(section.BodyPartName.Fields)
		rmail, err := mail.CreateReader(bodyPart)
		if err != nil {
			b.Log.Errorf("mail CreateReader func : %s", err)
			continue
		}
		// only 50 part is allowed , precaution of endless loop
		for i := 0; i < 50; i++ {
			p, err := rmail.NextPart()
			if err != nil {
				b.Log.Errorf("next Part func : %s", err)
				break
			}
			b.HandleEmailPart(msg, p)
		}
	}
}
func (b *Bemail) HandleEmailPart(msg config.Message, part *mail.Part) {
	mediatype, mediaParams, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
	if err != nil {
		b.Log.Infof("parse email header failed : %s", err)
		return
	}
	contentDisposition := strings.Split(part.Header.Get("Content-Disposition"), ";")[0]

	switch mediatype {
	case "multipart/mixed":
	case "text/plain":
		b.HandleEmailText(msg, part.Body)
	case "image/jpeg", "image/png":
		if contentDisposition != "attachment" {
			break
		}
		fileName := mediaParams["name"]
		b.HandleIncomingAttach(msg, part.Body, mediatype, fileName)
	default:
		// TODO if there is other type to handle
	}
}
func (b *Bemail) HandleEmailText(rmsg config.Message, r io.Reader) {
	content, err := io.ReadAll(r)
	if err != nil {
		b.Log.Println(err)
		return
	}
	rmsg.Text = strings.Split(string(content), "\r\n")[0]
	b.Remote <- rmsg
}

func (b *Bemail) HandleIncomingAttach(rmsg config.Message, r io.Reader, ctyp, fileName string) {
	// add ext to file name if there is none , there is chance that it can change the original file name
	extSL := strings.Split(ctyp, "/")
	ext := extSL[len(extSL)-1]
	if filepath.Ext(fileName) == "" {
		fileName += "." + ext
	}
	content, err := io.ReadAll(r)
	if err != nil {
		b.Log.Errorf(err.Error())
		return
	}
	rmsg.Extra = make(map[string][]interface{})

	rmsg.Extra["file"] = append(rmsg.Extra["file"], config.FileInfo{
		Name: fileName,
		Data: &content,
		Size: int64(len(content)),
	})
	b.Remote <- rmsg
}

func (b *Bemail) IsProcessed(msgID string) bool {
	for _, v := range b.ProcessedEmailMsgID {
		if v == msgID {
			return true
		}
	}
	return false
}

func (b *Bemail) Connect() error {
	b.Log.Println("Connecting to the IMAP server...")

	// Connect to  iamp server
	c, err := client.DialTLS(b.GetString("IMAP"), nil)
	if err != nil {
		return err
	}
	b.Log.Println("Connected to IMAP server")

	// Login
	if err := c.Login(b.GetString("username"), b.GetString("password")); err != nil {
		return err
	}

	// TODO add logout
	b.Log.Println("Logged in")

	// List mailboxes
	mailboxes := make(chan *imap.MailboxInfo, 10)
	err = c.List("", "*", mailboxes)
	if err != nil {
		return err
	}
	b.Client = c

	log.Println("Mailboxes:")
	for m := range mailboxes {
		b.Inboxes = append(b.Inboxes, m)
		b.Log.Println("* " + m.Name)
	}

	go b.SyncEmailInbox()
	return nil
}
func (b *Bemail) SyncEmailInbox() {
	for {
		time.Sleep(10 * time.Second)

		mbox, err := b.Client.Select("INBOX", false)
		if err != nil {
			b.Log.Println(err)
			continue
		}
		b.Log.Println("Flags for INBOX:", mbox.Flags)

		seqset := new(imap.SeqSet)
		cr := &imap.SearchCriteria{
			SentSince:        time.Now().Add(-5 * time.Minute),
		}
		ser, err := b.Client.Search(cr)
		if err != nil {
			b.Log.Println(err)
			continue
		}
		seqset.AddNum(ser...)

		section := &imap.BodySectionName{}
		secItem := section.FetchItem()
		messages := make(chan *imap.Message, 100)
		errFetch := make(chan error)
		go func() {
			errFetch <- b.Client.Fetch(seqset, []imap.FetchItem{imap.FetchBody, imap.FetchFlags, imap.FetchBodyStructure, secItem, imap.FetchEnvelope}, messages)
		}()
		b.Log.Println("Last 100 messages:")
		for msg := range messages {
			if !b.IsProcessed(msg.Envelope.MessageId) {
				b.Log.Println(msg.Envelope.Date)
				b.Log.Println(msg.InternalDate)

				b.ProcessedEmailMsgID = append(b.ProcessedEmailMsgID, msg.Envelope.MessageId)
				b.HandleIncomingEmail(msg)
			}
		}
		if err := <-errFetch; err != nil {
			b.Log.Println(err)
		}
	}
}

func (b *Bemail) JoinChannel(channel config.ChannelInfo) error {
	return nil
}
