package bwhatsapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/42wim/matterbridge/bridge/config"
	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
)

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
func (b *Bwhatsapp) SendGroupsInfo(groups []*types.GroupInfo) {
	time.Sleep(10 * time.Second)

	for _, group := range groups {
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
func (b *Bwhatsapp) ApsConnect() error {
	device, err := b.getDevice()
	if err != nil {
		return err
	}

	number := b.GetString(cfgNumber)
	if number == "" {
		return errors.New("whatsapp's telephone number need to be configured")
	}

	b.Log.Debugln("Connecting to WhatsApp..")

	b.wc = whatsmeow.NewClient(device, waLog.Stdout("Client", "INFO", true))
	b.wc.AddEventHandler(b.eventHandler)

	firstlogin := false
	var qrChan <-chan whatsmeow.QRChannelItem
	if b.wc.Store.ID == nil {
		firstlogin = true
		qrChan, err = b.wc.GetQRChannel(context.Background())
		b.Remote <- config.Message{
			Text:             "hi",
			Channel:          "#testing",
			Username:         "system",
			UserID:           "",
			Avatar:           "",
			Account:          b.Account,
			Event:            "event",
			Protocol:         "whatsapp",
			Gateway:          "",
			ParentID:         "",
			Timestamp:        time.Time{},
			ID:               "",
			Extra:            map[string][]interface{}{},
			ExtraNetworkInfo: config.ExtraNetworkInfo{},
		}
		if err != nil && !errors.Is(err, whatsmeow.ErrQRStoreContainsID) {
			return errors.New("failed to to get QR channel:" + err.Error())
		}
	}

	err = b.wc.Connect()
	if err != nil {
		return errors.New("failed to connect to WhatsApp: " + err.Error())
	}

	if b.wc.Store.ID == nil {
		for evt := range qrChan {
			if evt.Event == "code" {
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else {
				b.Log.Infof("QR channel result: %s", evt.Event)
			}
		}
	}

	// disconnect and reconnect on our first login/pairing
	// for some reason the GetJoinedGroups in JoinChannel doesn't work on first login
	if firstlogin {
		b.wc.Disconnect()
		time.Sleep(time.Second)

		err = b.wc.Connect()
		if err != nil {
			return errors.New("failed to connect to WhatsApp: " + err.Error())
		}
	}

	b.Log.Infoln("WhatsApp connection successful")

	b.contacts, err = b.wc.Store.Contacts.GetAllContacts()
	if err != nil {
		return errors.New("failed to get contacts: " + err.Error())
	}

	b.startedAt = time.Now()

	// map all the users
	for id, contact := range b.contacts {
		if !isGroupJid(id.String()) && id.String() != "status@broadcast" {
			// it is user
			b.users[id.String()] = contact
		}
	}

	// get user avatar asynchronously
	b.Log.Info("Getting user avatars..")

	for jid := range b.users {
		info, err := b.GetProfilePicThumb(jid)
		if err != nil {
			b.Log.Warnf("Could not get profile photo of %s: %v", jid, err)
		} else {
			b.Lock()
			if info != nil {
				b.userAvatars[jid] = info.URL
			}
			b.Unlock()
		}
	}

	b.Log.Info("Finished getting avatars..")

	return nil
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
