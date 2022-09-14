package birc

import (
	"io/ioutil"
	"sort"
	"strings"

	"github.com/42wim/matterbridge/bridge/config"
	"github.com/lrstanley/girc"
	"github.com/paulrosania/go-charset/charset"
	"github.com/saintfish/chardet"
)

func (b *Birc) GetChannelNames() {

}

func (b *Birc) HandleStoreNames(client *girc.Client, event girc.Event) {
	channel := event.Params[2]
	b.names[channel] = append(
		b.names[channel],
		strings.Split(strings.TrimSpace(event.Last()), " ")...)
	for i := range b.names[channel] {
		if b.names[channel][i] == event.Params[0] {
			b.names[channel] = append(b.names[channel][:i], b.names[channel][i+1:]...)
			break
		}
	}
}
func (b *Birc) HandleTopicChannel(client *girc.Client, event girc.Event) {

	rmsg := config.Message{
		Username: strings.ToLower(event.Params[0]),
		Channel:  strings.ToLower(event.Params[1]),
		Account:  b.Account,
		UserID:   strings.ToLower(event.Params[0]) + "@" + event.Source.Name,
		Event:    config.EventJoinLeave, // add this event to make it appear as notice on matrix
	}

	b.Log.Debugf("== Receiving Error response event: %s %s %#v", event.Source.Name, event.Last(), event)
	b.Log.Debugf("== Receiving topic response event: %#v", rmsg)

}
func (b *Birc) HandleEndNames(client *girc.Client, event girc.Event) {
	channel := event.Params[1]
	members := make([]string, len(b.names[channel]))

	sort.Strings(b.names[channel])
	copy(members, b.names[channel])
	b.Remote <- config.Message{
		Username: b.Nick, Text: b.formatnicks(b.names[channel]),
		Channel: channel, Account: b.Account,
		ExtraNetworkInfo: config.ExtraNetworkInfo{ChannelUsersMember: members},
		Event:            "new_users",
	}
	delete(b.names, channel)
}
func (b *Birc) handleDirectMsg(client *girc.Client, event girc.Event) {

	if b.skipPrivMsg(event) && !(event.Params[0] == b.Nick) {

		return
	}

	rmsg := config.Message{
		Username:         event.Source.Name,
		Channel:          strings.ToLower(event.Params[0]),
		Account:          b.Account,
		UserID:           event.Source.Ident + "@" + event.Source.Host,
		ExtraNetworkInfo: config.ExtraNetworkInfo{ChannelUsersMember: []string{event.Source.Name}},
		//OriginChannel:      strings.ToLower(event.Params[0]),
	}

	b.Log.Debugf("== Receiving PRIVMSG: %s %s %#v", event.Source.Name, event.Last(), event)

	// set action event
	if event.IsAction() {
		rmsg.Event = config.EventUserAction
	}

	// set NOTICE event
	if event.Command == "NOTICE" {
		rmsg.Event = config.EventNoticeIRC
	}
	rmsg.Event = "direct_msg"
	// strip action, we made an event if it was an action
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

	b.Log.Debugf("<= Sending message from %s on %s to gateway", event.Params[0], b.Account)
	b.Remote <- rmsg
}
