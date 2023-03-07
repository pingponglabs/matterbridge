package bdiscord

import (
	"fmt"
	"strings"
	"time"

	"github.com/42wim/matterbridge/bridge/config"
	"github.com/bwmarrin/discordgo"
)

var UserIDChannelIDMap = map[string]string{}

func (b *Bdiscord) SendChannelsAndMembers() {
	time.Sleep(10 * time.Second)
	slUser := []string{}
	userIdmap := map[string]string{}
	for k, v := range b.nickMemberMap {
		slUser = append(slUser, k)
		userIdmap[v.User.ID] = k
	}
	for _, v := range b.channels {

		b.Remote <- config.Message{
			Username: b.nick, Text: "new_users",
			Channel: v.Name, Account: b.Account,
			Event:    "new_users",
			Protocol: "discord",
			ExtraNetworkInfo: config.ExtraNetworkInfo{
				ChannelUsersMember: slUser,
				ChannelId:          v.ID,
				ChannelName:        v.Name,
				UsersMemberId:      userIdmap,
			},
		}
	}
}
func (b *Bdiscord) HandleDirectMessage(msg config.Message) (string, error) {
	b.Log.Debugf("=> Receiving %#v", msg)
	msg.Channel = strings.TrimPrefix(msg.Channel, "ID:")
	channelID := b.GetChannelIDFromUserID(msg.Channel)
	if !b.IsDMChannelIDExist(channelID) {
		dmChannel, err := b.c.UserChannelCreate(msg.Channel)
		if err != nil {
			b.Log.Errorf("failed creating direct message channel for user %s", msg.Channel)
			return "", nil
		}
		b.channels = append(b.channels, dmChannel)
		b.Lock()
		UserIDChannelIDMap[msg.Channel] = dmChannel.ID
		b.Unlock()

	}
	channelID = b.GetChannelIDFromUserID(msg.Channel)

	if !b.IsDMChannelIDExist(channelID) {
		return "", fmt.Errorf("Could not find channelID for %v", channelID)
	}

	if msg.Event == config.EventUserTyping {
		if b.GetBool("ShowUserTyping") {
			err := b.c.ChannelTyping(channelID)
			return "", err
		}
		return "", nil
	}

	// Make a action /me of the message
	if msg.Event == config.EventUserAction {
		msg.Text = "_" + msg.Text + "_"
	}

	// Handle prefix hint for unthreaded messages.
	if msg.ParentNotFound() {
		msg.ParentID = ""
	}

	// Use webhook to send the message
	useWebhooks := b.shouldMessageUseWebhooks(&msg)
	if useWebhooks && msg.Event != config.EventMsgDelete && msg.ParentID == "" {
		return b.handleEventWebhook(&msg, channelID)
	}

	return b.handleEventBotUser(&msg, channelID)
}

func (b *Bdiscord) IsDMChannelIDExist(ID string) bool {

	b.channelsMutex.RLock()
	defer b.channelsMutex.RUnlock()

	for _, channel := range b.channels {
		if channel.ID == ID && channel.Type == discordgo.ChannelTypeDM {
			return true
		}
	}
	return false
}
func (b *Bdiscord) GetChannelIDFromUserID(ID string) string {

	b.channelsMutex.RLock()
	defer b.channelsMutex.RUnlock()

	if channel, ok := UserIDChannelIDMap[ID]; ok {
		return channel
	}
	return ""
}
