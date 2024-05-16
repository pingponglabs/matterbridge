package birc

import (
	"io/ioutil"
	"strings"

	"github.com/42wim/matterbridge/bridge/config"
	"github.com/lrstanley/girc"
	"github.com/paulrosania/go-charset/charset"
	"github.com/saintfish/chardet"
)

func (b *Birc) HandleCannotSendChannel(client *girc.Client, event girc.Event) {

	rmsg := config.Message{
		Username: strings.ToLower(event.Params[0]),
		Channel:  strings.ToLower(event.Params[1]),
		Account:  b.Account,
		UserID:   strings.ToLower(event.Params[0]) + "@" + event.Source.Name,
		Event:    config.EventJoinLeave, // add this event to make it appear as notice on matrix
	}

	b.Log.Debugf("== Receiving Error response event: %s %s %#v", event.Source.Name, event.Last(), event)

	rmsg.Text += event.StripAction()

	// start detecting the charset
	mycharset := b.GetString("Charset")
	if mycharset == "" {
		// detect what were sending so that we convert it to utf-8
		detector := chardet.NewTextDetector()
		result, err := detector.DetectBest([]byte(rmsg.Text))
		if err != nil {
			b.Log.Infof("detection failed for rmsg.Text: %#v", rmsg.Text)
			return
		}
		b.Log.Debugf("detected %s confidence %#v", result.Charset, result.Confidence)
		mycharset = result.Charset
		// if we're not sure, just pick ISO-8859-1
		if result.Confidence < 80 {
			mycharset = "ISO-8859-1"
		}
	}
	switch mycharset {
	case "gbk", "gb18030", "gb2312", "big5", "euc-kr", "euc-jp", "shift-jis", "iso-2022-jp":
		rmsg.Text = toUTF8(b.GetString("Charset"), rmsg.Text)
	default:
		r, err := charset.NewReader(mycharset, strings.NewReader(rmsg.Text))
		if err != nil {
			b.Log.Errorf("charset to utf-8 conversion failed: %s", err)
			return
		}
		output, _ := ioutil.ReadAll(r)
		rmsg.Text = string(output)
	}

	b.Log.Debugf("<= Sending cannotSendToChannel error Notice from %s on %s to gateway", event.Params[1], b.Account)
	b.Remote <- rmsg
}
