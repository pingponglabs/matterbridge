package bemail

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strings"

	"github.com/42wim/matterbridge/bridge/config"
	"github.com/jordan-wright/email"
)

func (b *Bemail) Send(msg config.Message) (string, error) {
	b.Log.Debugf("=> Receiving %#v", msg)
	emailUser := b.GetString("Username")
	emailPass := b.GetString("Password")
	smtpAddrr := b.GetString("SMTP")
	smtpHost := smtpAddrr

	if sl := strings.Split(smtpAddrr, ":"); len(sl) > 1 {
		smtpHost = sl[0]
	}

	e := email.NewEmail()
	username := b.GetString("username")
	if !strings.Contains(username, "@") {
		username = fmt.Sprintf("%s@%s", username, smtpHost)
	}
	e.From = "<" + username + ">"
	e.To = []string{msg.Channel}

	e.Subject = "mediamagic Ai"
	if msg.Extra != nil {
		if msg.Protocol == "appservice" {
			msg.Text = ""
		}
		err := b.HandleMedia(e, &msg)
		if err != nil {
			return "", err
		}
	} else {
		b.HandleText(e, msg.Text)
	}
	certs := tls.Config{
		InsecureSkipVerify: true,
		ServerName: 	   smtpHost,
	}
	err := e.SendWithTLS(smtpAddrr, smtp.PlainAuth("", emailUser, emailPass, smtpHost), &certs)
	if err != nil {
		b.Log.Errorf("send mail failed to address %s : %s", msg.Channel, err)
	}
	return "", err

}

func (b *Bemail) HandleMedia(e *email.Email, msg *config.Message) error {
	if msg.Extra == nil {
		return fmt.Errorf("nil extra map")
	}

	for _, f := range msg.Extra["file"] {
		if fi, ok := f.(config.FileInfo); ok {
			bdata := *(fi.Data)
			contentType := http.DetectContentType(bdata[:512])
			content := bytes.NewReader(*fi.Data)
			if isMediaImage(fi.Name) {
				b.HandleMediaEmbed(e, fi.Name, fi.URL)
			}
			return b.HandleMediaAttach(e, content, fi.Name, contentType)
		}
	}
	return fmt.Errorf("no file attachment on extra")
}
func (b *Bemail) HandleMediaEmbed(e *email.Email, mediaName, link string) {
	htmlTemplate := fmt.Sprintf(`<img src="%s" alt="%s">`, link, mediaName)
	e.HTML = []byte(htmlTemplate)
}
func (b *Bemail) HandleMediaAttach(e *email.Email, r io.Reader, mediaName, mediaType string) error {
	attch, err := e.Attach(r, mediaName, mediaType)
	if err != nil {
		return err
	}
	e.Attachments = append(e.Attachments, attch)
	return nil
}
func (b *Bemail) HandleText(e *email.Email, text string) {
	e.Text = []byte(text)
}
func isMediaImage(name string) bool {
	mediaExt := ""
	if sl := strings.Split(name, "."); len(sl) > 1 {
		mediaExt = sl[len(sl)-1]
	}
	switch mediaExt {
	case "jpg", "png", "jpeg":
		return true
	}
	return false
}
