package bdiscord

import (
	"fmt"
	"time"

	"github.com/42wim/matterbridge/bridge/config"
	"github.com/bwmarrin/discordgo"
)

func (b *Bdiscord) SendChannelsAndMembers() {
	time.Sleep(10 * time.Second)
	slUser := []string{}
	for k, _ := range b.nickMemberMap {
		slUser = append(slUser, k)
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
			},
		}
	}
}
func (b *Bdiscord) HandleDirectMessage(msg config.Message) (string, error) {
	b.Log.Debugf("=> Receiving %#v", msg)
	channelID := b.getDMChannelID(msg.Channel)
	if channelID == "" {
		dmChannel, err := b.c.UserChannelCreate(b.nickMemberMap[msg.Channel].User.ID)
		if err != nil {
			b.Log.Errorf("failed creating direct message channel for user %s", msg.Channel)
			return "", nil
		}
		dmChannel.Name = msg.Channel
		b.channels = append(b.channels, dmChannel)
	}
	channelID = b.getDMChannelID(msg.Channel)
	if channelID == "" {
		return "", fmt.Errorf("Could not find channelID for %v", msg.Channel)
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
func (b *Bdiscord) getDMChannelID(name string) string {

	b.channelsMutex.RLock()
	defer b.channelsMutex.RUnlock()

	for _, channel := range b.channels {
		if channel.Name == name && channel.Type == discordgo.ChannelTypeDM {
			return channel.ID
		}
	}
	return ""
}
