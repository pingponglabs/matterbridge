package bappservice

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/42wim/matterbridge/bridge/config"
	gomatrix "maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	id "maunium.net/go/mautrix/id"
)

func (b *AppServMatrix) initControlRoom() {
	controlRoom := b.GetString("ApsPrefix") + "appservice_control"
	if _, ok := b.getChannelInfo(controlRoom); ok {
		return
	}

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
	b.setRoomInfo(controlRoom, nil, &ChannelInfo{
		RemoteName:   controlRoom,
		MatrixRoomID: roomId,
		IsDirect:     true,
		RemoteID:     controlRoom,
	})


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
	b.DbStore.SetAvatarUrl(b.GetString("ApsPrefix"), b.AvatarUrl)
}

func (b *AppServMatrix) isChannelExist(channelName string) bool {
	_, ok := b.getChannelInfo(channelName)
	return ok

}

func (b *AppServMatrix) AddNewChannel(channel, roomId, remoteId string, isDirect bool) error {

	if _, ok := b.getChannelInfo(channel); ok {
		return fmt.Errorf("channel already exist")
	}
	return b.DbStore.createChannel(&ChannelInfo{
		RemoteName:   channel,
		MatrixRoomID: roomId,
		IsDirect:     isDirect,
		RemoteID:     remoteId,
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
	roomName = roomName + " ( " + b.RemoteProtocol + " )"
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
func (b *AppServMatrix) RemoveUserFromRoom(reason string, mtxID, alias string) {
	userId, ok := b.getVirtualUserInfo(mtxID)
	if !ok {
		log.Println(fmt.Errorf("user %s not exist on appservice database", mtxID))
		return
	}

	_, err := b.apsCli.KickUser(id.RoomID(alias), &gomatrix.ReqKickUser{
		Reason: reason,
		UserID: id.UserID(userId.MatrixID),
	})
	if err != nil {
		log.Println(err)
	}
}

func (b *AppServMatrix) adjustChannel(rmsg *config.Message) {
	switch b.RemoteProtocol {
	case "discord":
		rmsg.Channel = "ID:" + rmsg.Channel
	}
}
