package bwhatsapp

import (
	"fmt"
	"time"

	"github.com/42wim/matterbridge/bridge/config"
	"go.mau.fi/whatsmeow/types"
)

var GroupInfo = make(map[string]string)

func (b *Bwhatsapp) IsSetupDM(userID string) bool {
	v, ok := b.dmSetupList[userID]
	return v && ok
}
func (b *Bwhatsapp) HandleDirectMessage(rmsg *config.Message) {

	b.dmSetupList[rmsg.UserID] = true
	rmsg.Channel = rmsg.UserID
	rmsg.Event = "direct_msg"
	rmsg.Protocol = "whatsapp"
	rmsg.ChannelName = rmsg.Username
	rmsg.ChannelId = rmsg.UserID

}
func (b *Bwhatsapp) HandleGroupMessage(rmsg *config.Message, jid types.JID) {
	channelName := GroupInfo[jid.String()]
	if channelName == "" {
		gInfo, err := b.wc.GetGroupInfo(jid)
		if err != nil {
			b.Log.Info(err)
		}
		channelName = gInfo.GroupName.Name
		GroupInfo[jid.String()] = channelName
	}

	rmsg.Event = "new_users"
	rmsg.Protocol = "whatsapp"
	rmsg.ChannelId = rmsg.Channel
	rmsg.ChannelName = channelName
	rmsg.ExtraNetworkInfo.UsersMemberId = map[string]string{
		rmsg.UserID: rmsg.Username,
	}

}
func (b *Bwhatsapp) SendGroupsInfo(groups []*types.GroupInfo) {
	time.Sleep(10 * time.Second)

	for _, group := range groups {
		GroupInfo[group.JID.String()] = group.GroupName.Name
		var contacts []string
		contactsName := make(map[string]string)
		for _, contact := range group.Participants {
			contactNum := contact.JID.String()
			contactName := b.contacts[contact.JID]
			contacts = append(contacts, contactName.FullName)
			contactsName[contactNum] = contactName.FullName

		}
		b.Remote <- config.Message{
			Username: b.GetString("Number"), Text: "new_users",
			Channel: group.JID.String(), Account: b.Account,
			Event:    "new_users",
			Protocol: "whatsapp",
			ExtraNetworkInfo: config.ExtraNetworkInfo{
				ChannelUsersMember: contacts,
				ChannelId:          group.JID.String(),
				ChannelName:        group.GroupName.Name,
				UsersMemberId:      contactsName,
			},
		}
	}

}

// Disconnect is called while reconnecting to the bridge
// Required implementation of the Bridger interface
func (b *Bwhatsapp) ApsDisconnect() error {
	b.wc.Disconnect()

	return nil
}

// JoinChannel Join a WhatsApp group specified in gateway config as channel='number-id@g.us' or channel='Channel name'
// Required implementation of the Bridger interface
// https://github.com/42wim/matterbridge/blob/2cfd880cdb0df29771bf8f31df8d990ab897889d/bridge/bridge.go#L11-L16
func (b *Bwhatsapp) JoinApsChannel(channel config.ChannelInfo) error {
	byJid := isGroupJid(channel.Name)

	groups, err := b.wc.GetJoinedGroups()
	if err != nil {
		return err
	}
	if b.GetBool("AppServiceLink") {
		go b.SendGroupsInfo(groups)
		return nil
	}
	// verify if we are member of the given group
	if byJid {
		gJID, err := types.ParseJID(channel.Name)
		if err != nil {
			return err
		}

		for _, group := range groups {
			if group.JID == gJID {
				return nil
			}
		}
	}

	foundGroups := []string{}

	for _, group := range groups {
		if group.Name == channel.Name {
			foundGroups = append(foundGroups, group.Name)
		}
	}

	switch len(foundGroups) {
	case 0:
		// didn't match any group - print out possibilites
		for _, group := range groups {
			b.Log.Infof("%s %s", group.JID, group.Name)
		}
		return fmt.Errorf("please specify group's JID from the list above instead of the name '%s'", channel.Name)
	case 1:
		return fmt.Errorf("group name might change. Please configure gateway with channel=\"%v\" instead of channel=\"%v\"", foundGroups[0], channel.Name)
	default:
		return fmt.Errorf("there is more than one group with name '%s'. Please specify one of JIDs as channel name: %v", channel.Name, foundGroups)
	}
}
