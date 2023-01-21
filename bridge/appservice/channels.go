package bappservice

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	gomatrix "maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	id "maunium.net/go/mautrix/id"
)

func (b *AppServMatrix) initControlRoom() {
	controlRoom := b.GetString("ApsPrefix") + "appservice_control"
	if _, ok := b.getRoomInfo(controlRoom); ok {
		return
	}
	b.uploadAvatar()

	time.Sleep(15 * time.Second)

	resp, err := b.apsCli.CreateRoom(&gomatrix.ReqCreateRoom{
		Name:   controlRoom,
		Invite: []id.UserID{id.UserID(b.GetString("MainUser"))},

		IsDirect: true,
	})
	if err != nil {
		b.Log.Debug("failed creating control room", err)
		return
	}
	roomId := resp.RoomID.String()

	b.sendRoomAvatarEvent(roomId)
	b.setRoomInfo(controlRoom, &MatrixRoomInfo{
		RoomName: controlRoom,
		Alias:    roomId,
		Members:  nil,
		IsDirect: true,
		RemoteId: controlRoom,
	})

	b.setRoomMap(roomId, controlRoom)
	b.saveState()

}
func (b *AppServMatrix) sendRoomAvatarEvent(roomId string) error {

	content := AvatarContent{
		Info: ImageInfo{
			Mimetype: "png",
			Height:   128,
			Width:    128,
		},
		URL: b.AvatarUrl,
	}
	_, err := b.apsCli.SendStateEvent(id.RoomID(roomId), event.StateRoomAvatar, "", content)
	return err
}

func (b *AppServMatrix) uploadAvatar() {
	if b.AvatarUrl != "" {
		return
	}
	path := b.GetString("AvatarUrl")
	file, err := os.ReadFile(path)
	if err != nil {
		b.Log.Debug("failed to read avatar file", err)
		return
	}
	url, err := b.apsCli.UploadMedia(
		gomatrix.ReqUploadMedia{
			ContentBytes:  file,
			Content:       nil,
			ContentLength: 0,
			ContentType:   "",
			FileName:      filepath.Base(path),
			UnstableMXC:   id.ContentURI{},
		},
	)
	if err != nil {
		b.Log.Debug("failed to upload avatar to server", err)

		return
	}

	b.AvatarUrl = url.ContentURI.String()
}

func (b *AppServMatrix) isChannelExist(channelName string) bool {
	_, ok := b.getRoomInfo(channelName)

	return ok

}
func (b *AppServMatrix) newUsersInChannel(channelName string, ExternMembers []string) []string {
	newMembers := []string{}
	roomInfo, ok := b.getRoomInfo(channelName)
	if !ok {
		return ExternMembers
	}
	b.RLock()
	for _, ExternMember := range ExternMembers {
		exist := false
		for _, member := range roomInfo.Members {
			if ExternMember == member.Name {
				exist = true
				break
			}

		}
		if !exist {
			newMembers = append(newMembers, ExternMember)
		}
	}
	b.RUnlock()
	return newMembers

}

func (b *AppServMatrix) joinRoom(roomId, userID, Token string) error {
	cli, err := gomatrix.NewClient(b.GetString("Server"), id.UserID(userID), Token)
	if err != nil {
		return err
	}
	_, err = cli.JoinRoom(roomId, "", nil)
	if err != nil {
		return err
	}
	return nil
}
func (b *AppServMatrix) AddNewChannel(channel, roomId, remoteId ,spaceId string, isDirect bool,) {

	b.setRoomInfo(channel, &MatrixRoomInfo{
		RoomName: channel,
		Alias:    roomId,
		Members:  []ChannelMember{},
		IsDirect: isDirect,
		RemoteId: remoteId,
		SpaceId: spaceId,
	})
}

var whitespace = "\t\n\x0b\x0c\r "

func AlterAlias(s string) string {
	for _, v := range s {
		if strings.Contains(whitespace, string(v)) {
			s = strings.ReplaceAll(s, string(v), "__")
		}
	}
	return s
}
func (b *AppServMatrix) createRoom(roomName string, members []string, isDirect bool) (string, error) {
	preset := "public_chat"
	invites := []id.UserID{}
	for _, v := range members {
		invites = append(invites, id.UserID(v))
	}
	alias := roomName + "-" + randStringLowerRunes(4)

	if strings.ContainsAny(alias, whitespace) {
		alias = AlterAlias(alias)
	}
	if isDirect {
		roomName = ""
		preset = "private_chat"
		alias = ""

	}

	resp, err := b.apsCli.CreateRoom(&gomatrix.ReqCreateRoom{
		RoomAliasName: alias,
		Name:          roomName,
		Topic:         "",
		Invite:        invites,
		Preset:        preset,
		IsDirect:      isDirect,
	})
	if err != nil {
		return "", err
	}

	return string(resp.RoomID), nil
}
func (b *AppServMatrix) inviteToRoom(roomId string, invites []string) error {
	var invitesId []id.UserID
	for _, v := range invites {
		invitesId = append(invitesId, id.UserID(v))
	}
	for _, v := range invitesId {
		_, err := b.apsCli.InviteUser(id.RoomID(roomId), &gomatrix.ReqInviteUser{
			Reason: "",
			UserID: v,
		})
		if err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
}
func (b *AppServMatrix) inviteUserToRoom(roomId string, inviteId string) error {

	_, err := b.apsCli.InviteUser(id.RoomID(roomId), &gomatrix.ReqInviteUser{
		Reason: "",
		UserID: id.UserID(inviteId),
	})
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}
func (b *AppServMatrix) removeUsersFromChannel(channel string, members []string) {
	chanInfo, ok := b.getRoomInfo(channel)
	if !ok {
		return
	}
	for _, member := range members {

		for i, v := range chanInfo.Members {
			if v.Name == member {
				chanInfo.Members = append(chanInfo.Members[:i], chanInfo.Members[i+1:]...)
				break
			}
		}
		userId, ok := b.getVirtualUserInfo(member)
		if !ok {
			log.Println(fmt.Errorf("user %s not exist on appservice database", member))
			continue
		}

		_, err := b.apsCli.KickUser(id.RoomID(b.getRoomID(channel)), &gomatrix.ReqKickUser{
			Reason: "user parts",
			UserID: id.UserID(userId.Id),
		})
		if err != nil {
			log.Println(err)
		}
	}
}

func (b *AppServMatrix) getWhatsappName(channelJid string) string {
	b.RLock()
	defer b.RUnlock()
	for i, v := range b.roomsInfo {
		if v.RemoteId == channelJid {
			return i
		}
	}
	return ""
}
