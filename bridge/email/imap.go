package bemail

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/42wim/matterbridge/bridge"
	"github.com/42wim/matterbridge/bridge/config"
	"github.com/DusanKasan/parsemail"
	"github.com/k3a/html2text"

	"github.com/emersion/go-imap"

	"github.com/emersion/go-imap/client"
)

type Bemail struct {
	sync.RWMutex
	*bridge.Config
	ProcessedEmailMsgID []string
	LastFetchTimeStamp  time.Time
	*client.Client
	Inboxes []*imap.MailboxInfo

	connectRetryLock sync.Mutex
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
			Username: username,
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
	for _, bodyPart := range body {
		emailContent, err := parsemail.Parse(bodyPart)
		if err != nil {
			b.Log.Infof("parse email body failed : %s", err)
			return
		}
		b.HandleEmailParsed(msg, emailContent)

		/*
			rmail, err := mail.CreateReader(bodyPart)
			if err != nil {
				b.Log.Errorf("mail CreateReader func : %s", err)
				continue
			}
		*/

		// only 50 part is allowed , precaution of endless loop
		/*
			log.Println(emailContent.ContentType)
			for i := 0; i < 50; i++ {
				p, err := rmail.NextPart()
				if err != nil {
					b.Log.Debugf("next Part func : %s", err)
					break
				}
				b.HandleEmailPart(msg, p)
			}
		*/
	}
}
func (b *Bemail) HandleEmailParsed(msg config.Message, emailContent parsemail.Email) {
	if emailContent.TextBody != "" {
		b.HandleEmailText(msg, emailContent.TextBody)
	} else if emailContent.HTMLBody != "" {
		b.HandleEmailHtml(msg, emailContent.HTMLBody)
	}
	if emailContent.Attachments != nil {
		for _, attach := range emailContent.Attachments {
			fileName := attach.Filename

			if attach.ContentType == "image/jpeg" || attach.ContentType == "image/png" {
				b.HandleIncomingAttach(msg, attach.Data, attach.ContentType, fileName)
			}
		}
	}
}

/*
	func (b *Bemail) HandleEmailPart(msg config.Message, part *mail.Part) {
		mediatype, mediaParams, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			b.Log.Infof("parse email header failed : %s", err)
			return
		}
		contentDisposition := strings.Split(part.Header.Get("Content-Disposition"), ";")[0]
		switch mediatype {
		case "multipart/mixed":
		case "text/plain", "text/html":
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
*/
func (b *Bemail) HandleEmailText(rmsg config.Message, text string) {
	rmsg.Text = TrimEmail(text)
	if rmsg.Text == "" {
		return
	}
	b.Remote <- rmsg
	// delay in case there is attachment on the email
	time.Sleep(500 * time.Millisecond)
}

// handle email htmlbody
func (b *Bemail) HandleEmailHtml(rmsg config.Message, html string) {
	plain := html2text.HTML2Text(html)
	rmsg.Text = TrimEmail(plain)
	if rmsg.Text == "" {
		return
	}
	b.Remote <- rmsg
	// delay in case there is attachment on the email
	time.Sleep(500 * time.Millisecond)
}
func TrimEmail(emailText string) string {
	var res string
	emailText = strings.ReplaceAll(emailText, "\r", "")
	sl := strings.Split(emailText, "\n")
	var SkipNextLine bool
	for _, l := range sl {
		if SkipNextLine {
			SkipNextLine = false
			continue
		}
		l = strings.TrimSuffix(l, " ")
		if strings.HasPrefix(l, "On ") && strings.Contains(l, "<") {
			if strings.HasSuffix(l, "=") {
				SkipNextLine = true
				continue
			}
			if strings.HasSuffix(l, ":") {
				continue
			}
		}

		if l == "" || l == "\r" || l == "\n" {
			continue
		}
		if strings.HasPrefix(l, ">") {
			if strings.HasSuffix(l, "=") {
				SkipNextLine = true
			}
			continue
		}
		res += l + "\n"

	}
	return strings.TrimSuffix(res, "\n")
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
		time.Sleep(5 * time.Second)

		_, err := b.Client.Select("INBOX", false)
		if err != nil {

			b.Log.Infof("email bridge : select inbox  failed with error %v\n", err)
			go b.ConnectRetry()
			return
		}
		seqset := new(imap.SeqSet)
		cr := &imap.SearchCriteria{
			SentSince: time.Now().Add(-5 * time.Minute),
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
		for msg := range messages {
			if !b.IsProcessed(msg.Envelope.MessageId) {
				if len(msg.Envelope.From) > 0 {
					b.Log.Infof("email from %s with subject %q sent on %s \n", msg.Envelope.From[0].MailboxName, msg.Envelope.Subject, msg.Envelope.Date)
				}
				b.ProcessedEmailMsgID = append(b.ProcessedEmailMsgID, msg.Envelope.MessageId)
				go b.HandleIncomingEmail(msg)
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

func (b *Bemail) ConnectRetry() {
	b.connectRetryLock.Lock()
	defer b.connectRetryLock.Unlock()
	for i := 0; i < 10; i++ {
		err := b.Client.Logout()
		if err != nil {
			b.Log.Errorf("logout error  : %v", err)
		}
		err = b.Connect()
		if err != nil {
			b.Log.Errorf("failed to connect to server attempt number %v, error message : %v", i+1)
		} else {
			return
		}
		time.Sleep(1 * time.Minute)

	}
	os.Exit(1)

}
