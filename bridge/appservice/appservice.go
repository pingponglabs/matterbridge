package bappservice

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"fmt"
	"mime"
	"regexp"
	"strings"
	"sync"

	"github.com/gorilla/mux"

	// import sqlite3 driver
	_ "github.com/mattn/go-sqlite3"

	"maunium.net/go/mautrix"
	gomatrix "maunium.net/go/mautrix"
	id "maunium.net/go/mautrix/id"

	"github.com/42wim/matterbridge/bridge"
	"github.com/42wim/matterbridge/bridge/config"
	"github.com/42wim/matterbridge/bridge/helper"
	matrix "github.com/matterbridge/gomatrix"
)

var (
	htmlTag            = regexp.MustCompile("</.*?>")
	htmlReplacementTag = regexp.MustCompile("<[^>]*>")
)

type NicknameCacheEntry struct {
	displayName string
	lastUpdated time.Time
}

type httpError struct {
	Errcode      string `json:"errcode"`
	Err          string `json:"error"`
	RetryAfterMs int    `json:"retry_after_ms"`
}

type matrixUsername struct {
	plain     string
	formatted string
}

// SubTextMessage represents the new content of the message in edit messages.
type SubTextMessage struct {
	MsgType       string `json:"msgtype"`
	Body          string `json:"body"`
	FormattedBody string `json:"formatted_body,omitempty"`
	Format        string `json:"format,omitempty"`
}

// MessageRelation explains how the current message relates to a previous message.
// Notably used for message edits.
type MessageRelation struct {
	EventID string `json:"event_id"`
	Type    string `json:"rel_type"`
}

type EditedMessage struct {
	NewContent SubTextMessage  `json:"m.new_content"`
	RelatedTo  MessageRelation `json:"m.relates_to"`
	matrix.TextMessage
}

type InReplyToRelationContent struct {
	EventID string `json:"event_id"`
}

type InReplyToRelation struct {
	InReplyTo InReplyToRelationContent `json:"m.in_reply_to"`
}

type ReplyMessage struct {
	RelatedTo InReplyToRelation `json:"m.relates_to"`
	matrix.TextMessage
}
type memberChan struct {
	member  MemberInfo
	channel string
}
type UserMapInfo struct {
	MtxID    string
	Username string
}
type AppServMatrix struct {
	mc             *matrix.Client
	apsCli         *gomatrix.Client
	UserID         string
	NicknameMap    map[string]NicknameCacheEntry
	RoomMap        map[string]string
	channelsInfo   map[string]*ChannelInfo
	remoteUsername string
	virtualUsers   map[string]MemberInfo
	UsersMap       map[string]UserMapInfo
	memberChan     chan memberChan
	RemoteProtocol string
	AvatarUrl      string
	rateMutex      sync.RWMutex
	RemoteNetwork  string
	DbStore        AppServStore
	sync.RWMutex
	*bridge.Config
}
type ChannelInfo struct {
	RemoteName   string `json:"remote_name,omitempty"`
	MatrixRoomID string `json:"matrix_room_id,omitempty"`
	IsDirect     bool   `json:"is_direct,omitempty"`
	RemoteID     string `json:"remote_id,omitempty"`
}
type ChannelMember struct {
	ChannelID string `json:"name,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	Joined    bool   `json:"joined,omitempty"`
}

type MemberInfo struct {
	Username    string `json:"username,omitempty"`
	MatrixToken string `json:"matrix_token,omitempty"`
	MatrixID    string `json:"matrix_id,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	Registred   bool   `json:"registred,omitempty"`
}

type appserviceData struct {
	RoomsInfo    map[string]*ChannelInfo `json:"rooms_info,omitempty"`
	VirtualUsers map[string]MemberInfo   `json:"virtual_users,omitempty"`
	UsersMap     map[string]UserMapInfo  `json:"users_map,omitempty"`

	RemoteProtocol string `json:"remote_protocol,omitempty"`
	AvatarUrl      string `json:"avatar_url,omitempty"`
}

func New(cfg *bridge.Config) bridge.Bridger {
	b := &AppServMatrix{Config: cfg}
	b.RoomMap = make(map[string]string)
	b.NicknameMap = make(map[string]NicknameCacheEntry)
	b.channelsInfo = make(map[string]*ChannelInfo)
	b.virtualUsers = make(map[string]MemberInfo)
	b.UsersMap = make(map[string]UserMapInfo)
	return b
}

func (b *AppServMatrix) Connect() error {
	mx := mux.NewRouter()
	mx.HandleFunc("/transactions/{txnId}", b.handleTransaction).Methods("PUT").Queries("access_token", "{token}")
	mx.HandleFunc("/users/{userId}", b.handleTransaction).Methods("GET").Queries("access_token", "{token}")
	mx.HandleFunc("/rooms/{roomAlias}", b.handleTransaction).Methods("GET").Queries("access_token", "{token}")

	var err error
	b.apsCli, err = gomatrix.NewClient(b.GetString("Server"), id.UserID(b.GetString("MxID")), b.GetString("Token"))
	if err != nil {
		return err
	}
	go func() {
		log.Fatal(http.ListenAndServe(":"+b.GetString("Port"), mx))
	}()
	storePathFlag := flag.Lookup("conf")
	storepath := ""
	if storePathFlag == nil {
		log.Println("conf path flag not defined")
	} else {
		storepath = storePathFlag.Value.String()
	}
	log.Println("store path: ", storepath)
	// replace the the base name of the config file with the appservice prefix
	dbPath := filepath.Join(filepath.Dir(storepath) + "/" + b.GetString("ApsPrefix") + "_" + "appservice.db")
	err = b.DbStore.NewDbConnection("sqlite3", dbPath)
	if err != nil {
		return err
	}

	err = b.DbStore.CreateTables()
	if err != nil {
		return err
	}
	err = b.DbStore.SetConfigName(b.GetString("ApsPrefix"))
	if err != nil {
		return err
	}
	remoteProtocol, avatar, _ := b.DbStore.GetAppServiceInfo(b.GetString("ApsPrefix"))
	b.RemoteProtocol = remoteProtocol
	b.AvatarUrl = avatar
	if b.RemoteProtocol == "" {
		b.RemoteProtocol = b.GetString("RemoteNetwork")
		b.DbStore.SetRemoteProtocol(b.GetString("ApsPrefix"), b.RemoteProtocol)
	}
	b.uploadAvatar()

	go b.initControlRoom()

	return nil
}

type AppEvents struct {
	Events []*matrix.Event `json:"events,omitempty"`
}

func (b *AppServMatrix) handleTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tx := vars["txnId"]

	log.Println(tx)
	tok := vars["token"]
	log.Println(tok)

	postBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		return
	}
	var appsEvents AppEvents
	err = json.Unmarshal(postBody, &appsEvents)
	log.Println(string(postBody))
	if err != nil {
		log.Println(err)
		return
	}
	for _, ev := range appsEvents.Events {
		go b.handleAppServEvent(ev)
	}
	w.WriteHeader(http.StatusOK)
}

func (b *AppServMatrix) handleAppServEvent(event *matrix.Event) {
	b.Log.Println(event)
	if event.Type == "m.room.member" {
		if val, ok := event.Content["membership"]; ok {
			if invite, ok := val.(string); ok && invite == "invite" {
				if valDirect, ok := event.Content["is_direct"]; ok {
					if isDirect, ok := valDirect.(bool); ok && isDirect {
						err := b.handleDirectInvites(*event.StateKey, event.RoomID, event.Sender)
						if err != nil {
							b.Log.Debug(err)
						}
					}
				} else {
					err := b.handleInvites(*event.StateKey, event.RoomID)
					if err != nil {
						b.Log.Debug(err)
					}
				}
			}
		}
	}
	switch event.Type {
	case "m.room.redaction", "m.room.message":
		b.handleEvent(event)
	case "m.room.member":
		b.handleMemberChange(event)
	}

}

func (b *AppServMatrix) handleDirectInvites(mtxUserId, roomId, Sender string) error {
	// create Channel if not exist (direct or not)
	if mtxUserId == "" {
		_, err := b.apsCli.JoinRoom(roomId, "", nil)
		if err != nil {
			return err
		}
		mtxUserId = b.apsCli.UserID.String()
	}
	userInfo, ok := b.getUsernameFromMtxId(mtxUserId)

	if !ok {
		return fmt.Errorf("user %s not exist on appservice database", mtxUserId)
	}

	mc, errmtx := b.newVirtualUserMtxClient(mtxUserId)
	if errmtx != nil {
		return errmtx
	}
	err := mc.SetDisplayName(userInfo.Username + " ( " + b.RemoteProtocol + " )")
	if err != nil {
		log.Println(err)
	}
	_, err = mc.JoinRoom(roomId, "", nil)
	if err != nil {
		return err
	}
	if Sender == b.apsCli.UserID.String() {
		return nil
	}
	channelName := userInfo.UserID

	b.setRoomInfobyMtxID(channelName, []string{mtxUserId}, &ChannelInfo{
		RemoteName:   userInfo.Username,
		MatrixRoomID: roomId,
		IsDirect:     true,
		RemoteID:     userInfo.UserID,
	})
	b.setRoomMap(roomId, channelName)
	// b.saveState()
	return nil

}
func (b *AppServMatrix) handleInvites(mtxID, roomId string) error {
	b.Log.Printf("handling invites for the user %s on room %s", mtxID, roomId)
	if mtxID == "" {
		_, err := b.apsCli.JoinRoom(roomId, "", nil)
		if err != nil {
			return err
		}
		return nil
	}

	userInfo, ok := b.getVirtualUserFromMtxId(mtxID)
	if !ok {
		return fmt.Errorf("user %s not found in the appservice database", mtxID)
	}
	mc, errmtx := b.newVirtualUserMtxClient(mtxID)
	if errmtx != nil {
		return errmtx
	}
	_, err := mc.JoinRoom(roomId, "", nil)
	if err != nil {
		return err
	}
	b.validateInvite(roomId, userInfo.UserID)
	return nil

}
func (b *AppServMatrix) validateInvite(roomId, mtxID string) {

	// update the virtual user joined status
	b.UpdateVirtualUserJoinedStatus(mtxID, roomId, true)

}
func (b *AppServMatrix) UpdateVirtualUserJoinedStatus(mtxID, roomId string, joined bool) {
	channelInfo, ok := b.getChannelInfoByMtxID(roomId)
	if !ok {
		return
	}
	userInfo, ok := b.getVirtualUserFromMtxId(mtxID)
	if !ok {
		return
	}
	err := b.DbStore.updateUserJoinedStatus(userInfo.MatrixID, channelInfo.MatrixRoomID, joined)

	if err != nil {
		b.Log.Error(err)
	}

}

func (b *AppServMatrix) Disconnect() error {
	return nil
}

// TODO : STILL NOT UPDATED
func (b *AppServMatrix) JoinChannel(channel config.ChannelInfo) error {

	go func() {
		time.Sleep(10 * time.Second)
		channels := b.GetAllMapChannels()
		for _, v := range channels {
			if !strings.HasPrefix(v, "#") {
				continue
			}
			rmsg := config.Message{
				Username: b.getDisplayName("appservice"),
				Channel:  v,
				Account:  b.Account,
				UserID:   "appservice",
				Event:    "join",
				Text:     "hi",

				Avatar: "", // b.getAvatarURL(ev.Sender)
			}
			rmsg.TargetPlatform = b.RemoteProtocol

			b.Log.Debugf("<= Sending message from %s on %s to gateway", "appservice", b.Account)
			b.Remote <- rmsg
		}
	}()
	return nil

}

type userListState struct {
	name      string
	Registred bool
	Joined    bool
	Leaved    bool
	NotExist  bool
}

func (b *AppServMatrix) handleChannelInfoEvent(channelName, channelID string, members map[string]string) {
	list := b.GetNotExistUsers(members)
	go b.registerUsersList(list)
	b.leaveUsersInChannel(channelID, members)

	if !b.isChannelExist(channelID) {

		roomId, err := b.createRoom(channelName, []string{b.GetString("MainUser")}, false)
		if err != nil {
			log.Println(fmt.Errorf("failed to create room %s : %w", channelName, err))
			return
		}
		b.sendRoomAvatarEvent(roomId)

		b.AddNewChannel(channelName, roomId, channelID, false)

		b.setRoomMap(roomId, channelID)
	}
	b.addNewMembers(channelID, members)

	go b.InviteUsersLoop(channelID)

}
func (b *AppServMatrix) InviteUsersLoop(channelID string) {
	roomInfo, ok := b.getChannelInfo(channelID)
	if !ok {
		return
	}

	members, err := b.DbStore.getChannelMembersState(channelID)
	if err != nil {
		b.Log.Error(err)
		return
	}
	for _, v := range members {
		if !v.Joined {
			time.Sleep(1 * time.Second)

			memberInfo, ok := b.getVirtualUserInfo(v.UserID)
			if !ok {
				continue
			}
			err := b.inviteToRoom(roomInfo.MatrixRoomID, []string{memberInfo.MatrixID})
			if respErr, ok := err.(mautrix.HTTPError); ok {
				if respErr.RespError.ErrCode == "M_FORBIDDEN" && respErr.RespError.Err == "User is already joined to room" {
					b.UpdateVirtualUserJoinedStatus(memberInfo.MatrixID, roomInfo.MatrixRoomID, true)
				} else {
					b.Log.Error(err)
				}

			}
		}
	}

}

func (b *AppServMatrix) registerUsersList(users map[string]string) {
	for k, v := range users {
		memberID, err := b.createVirtualUsers(v, k)
		if err != nil {
			log.Println(err)
			time.Sleep(100 * time.Millisecond)

			continue
		}
		time.Sleep(1 * time.Second)
		b.addVirtualUser(memberID)
		b.addUserMapInfo(k, UserMapInfo{
			MtxID:    memberID.MatrixID,
			Username: v,
		})

	}
}

func (b *AppServMatrix) handleDirectMessages(channelName, channelID string) {

	_, ok := b.getVirtualUserInfo(channelID)
	if !ok {
		memberID, err := b.createVirtualUsers(channelName, channelID)
		if err != nil {
			log.Println(err)
			return
		}
		b.addVirtualUser(memberID)

	}
	mtxInfo, ok := b.getVirtualUserInfo(channelID)
	if !ok {
		log.Println(fmt.Errorf("failed to get virtual user info %s ", channelName))
		return
	}

	mc, errmx := b.newVirtualUserMtxClient(mtxInfo.MatrixID)
	if errmx != nil {
		log.Println(fmt.Errorf("failed to create virtual user client %s ", channelName))
	}
	displayName := ""
	displayNameReq, err := mc.GetOwnDisplayName()
	if err != nil {
		log.Println(fmt.Errorf("failed to get display name %s ", channelName))
	} else {
		displayName = displayNameReq.DisplayName
	}

	resp, err := mc.CreateRoom(&matrix.ReqCreateRoom{

		Name:   displayName + " ( " + b.RemoteProtocol + " )",
		Topic:  channelName + " direct message room",
		Invite: []string{b.GetString("MainUser")},

		IsDirect: true,
	})
	time.Sleep(2 * time.Second)
	if err != nil {
		log.Println(fmt.Errorf("failed to  create direct room : %w", err))
		return
	}
	b.AddNewChannel(channelName, resp.RoomID, channelID, true)
	b.addNewMember(channelName, channelID)

	b.setRoomMap(resp.RoomID, channelID)
}
func (b *AppServMatrix) handleJoinUsers(channel string, users map[string]string) {
	for k, v := range users {
		if v == b.remoteUsername {
			return
		}
		memberInfo, ok := b.getVirtualUserInfo(v)
		if !ok {
			memberID, err := b.createVirtualUsers(v, k)
			if err != nil {
				log.Println(err)
				return
			}
			b.addVirtualUser(memberID)
			if memberInfo, ok = b.getVirtualUserInfo(v); !ok {
				return
			}
		}
		roomId := b.getRoomID(channel)
		err := b.inviteUserToRoom(roomId, memberInfo.MatrixID)
		if err != nil {
			continue
		}
		b.addNewMember(channel, k)
	}

}
func (b *AppServMatrix) HandleLeaveUsers(channel string, usersID []string) {
	// remove users from channel on conjunction table on database
	channelInfo, err := b.DbStore.getChannelByName(channel)
	if err != nil {
		if err == ErrChannelNotFound {
			b.Log.Error("channel not found", err)
		} else {
			b.Log.Error("failed to get channel", err)
		}
		return
	}

	for _, userID := range usersID {
		// get user info from database
		userInfo, ok := b.getVirtualUserInfo(userID)
		if !ok {
			continue
		}

		err = b.DbStore.removeUserFromChannel(channel, userID)
		if err != nil {
			b.Log.Error("failed to delete user from channel", err)
		}

		b.RemoveUserFromRoom("user parts", userInfo.MatrixID, channelInfo.MatrixRoomID)

	}

}

// TODO : NEED TO BE UPDATED
func (b *AppServMatrix) handleQuitUsers(reason string, members map[string]string) {
	rooms := b.getAllRoomInfo()
	for _, chanInfo := range rooms {
		b.leaveUsersInChannel(chanInfo.RemoteID, members)
	}

}

// TODO : NEED TO BE UPDATED
func (b *AppServMatrix) leaveUsersInChannel(channelName string, externMembers map[string]string) {
	roomInfo, ok := b.getChannelInfo(channelName)
	if !ok {
		return
	}

	// remove users from channel on conjunction table on database
	for _, userID := range externMembers {
		// get user info from database
		userInfo, ok := b.getVirtualUserInfo(userID)
		if !ok {
			continue
		}

		err := b.DbStore.removeUserFromChannel(channelName, userID)
		if err != nil {
			b.Log.Error("failed to delete user from channel", err)
			continue
		}

		b.RemoveUserFromRoom("user parts", userInfo.MatrixID, roomInfo.MatrixRoomID)
	}
}

func (b *AppServMatrix) handleTelegramMsg(msg *config.Message) {
	switch msg.ChannelType {
	case "channel":
		msg.Username = msg.ChannelName + "_bot"
		msg.Event = "direct_msg"
	case "private":
		msg.Event = "direct_msg"
		msg.Channel = msg.UserID
		msg.ChannelName = msg.Username
	case "group":
		msg.Event = "new_users"
	default:
		msg.Event = "new_users"

	}
}
func (b *AppServMatrix) HandleActionCommand(msg config.Message) {
	switch msg.ActionCommand {
	case "join":
		b.handleJoinUsers(msg.Channel, msg.UsersMemberId)
	case "quit":
		b.handleQuitUsers(msg.Channel, msg.UsersMemberId)
	case "part":
		b.HandleLeaveUsers(msg.Channel, msg.ChannelUsersMember)
	}
}

var once sync.Once

func (b *AppServMatrix) Send(msg config.Message) (string, error) {

	if msg.Protocol == "api" {
		if msg.ActionCommand != "imessage" {
			go b.controllAction(msg)
			return "", nil
		} else {
			msg.Protocol = "imessage"
		}
	}
	once.Do(func() {
		if b.RemoteProtocol == "" {
			b.DbStore.SetRemoteProtocol(b.GetString("ApsPrefix"), msg.Protocol)
			b.RemoteProtocol = msg.Protocol
		}
	})
	b.Log.Debugf("=> Receiving %#v", msg)
	switch msg.Protocol {

	case "irc":

	case "discord":
		msg.Channel = msg.ChannelId
		// TODO change channel name to channel ID
	case "telegram":
		b.handleTelegramMsg(&msg)
	case "whatsapp":
		msg.Username = strings.TrimPrefix(msg.Username, "+")
	case "imessage":
		msg.Username = msg.ChannelName
		msg.UserID = msg.ChannelId
		msg.Channel = msg.ChannelId
		b.AdjustExtra(&msg)
	}
	if msg.Text == "new_users" {
		msg.Text = ""
	}

	// Make a action /me of the message

	//TODO handle virtualUser creation here
	switch msg.Event {
	case "new_users":
		b.remoteUsername = msg.Username
		b.handleChannelInfoEvent(msg.ChannelName, msg.ChannelId, msg.UsersMemberId)
		// TODO create virtual users and join channels
	case "direct_msg":
		if !b.isChannelExist(msg.ChannelId) {
			b.handleDirectMessages(msg.ChannelName, msg.ChannelId)
		}
		msg.Channel = msg.ChannelId
		// TODO create virtual users and join channels

	case config.EventJoinLeave:

		b.HandleActionCommand(msg)
		return "", nil
		// TODO create virtual users and join channels

	}
	if msg.Text == "" && msg.Extra == nil {
		return "", nil
	}
	return b.SendMtx(msg)
}
func (b *AppServMatrix) SendMtx(msg config.Message) (string, error) {

	b.Log.Debugf("=> Receiving %#v", msg)

	channel := b.getRoomID(msg.Channel)
	b.Log.Debugf("Channel %s maps to channel id %s", msg.Channel, channel)

	mtxInfo, ok := b.getUserMapInfo(msg.UserID)
	if !ok {
		b.Log.Errorf("userID %s for name %s not exist in the appservice database", msg.UserID, msg.Username)
		return "", nil

	}
	mc, errmtx := b.newVirtualUserMtxClient(mtxInfo.MtxID)
	if errmtx != nil {
		b.Log.Debug(errmtx)
		mc, errmtx = matrix.NewClient(b.GetString("Server"), string(b.apsCli.UserID), b.apsCli.AccessToken)
		if errmtx != nil {
			return "", errmtx
		}
	}

	formattedText := b.incomingMention(msg.Protocol, msg.Text, msg.Mentions)

	if msg.Event == config.EventUserAction {
		m := matrix.TextMessage{
			MsgType:       "m.emote",
			Body:          msg.Text,
			FormattedBody: helper.ParseMarkdown(formattedText),
			Format:        "org.matrix.custom.html",
		}

		if b.GetBool("HTMLDisable") {
			m.Format = ""
			m.FormattedBody = ""
		}

		msgID := ""

		err := b.retry(func() error {
			resp, err := mc.SendMessageEvent(channel, "m.room.message", m)
			if err != nil {
				return err
			}

			msgID = resp.EventID

			return err
		})

		return msgID, err
	}

	// Delete message
	if msg.Event == config.EventMsgDelete {
		if msg.ID == "" {
			return "", nil
		}

		msgID := ""

		err := b.retry(func() error {
			resp, err := mc.RedactEvent(channel, msg.ID, &matrix.ReqRedact{})
			if err != nil {
				return err
			}

			msgID = resp.EventID

			return err
		})

		return msgID, err
	}

	// Upload a file if it exists
	if msg.Extra != nil {
		for _, rmsg := range helper.HandleExtra(&msg, b.General) {
			rmsg := rmsg

			err := b.retry(func() error {
				_, err := mc.SendText(channel, rmsg.Text)

				return err
			})
			if err != nil {
				b.Log.Errorf("sendText failed: %s", err)
			}
		}
		// check if we have files to upload (from slack, telegram or mattermost)
		if len(msg.Extra["file"]) > 0 {
			return b.handleUploadFiles(&msg, channel)
		}
	}

	// Edit message if we have an ID
	if msg.ID != "" {
		rmsg := EditedMessage{
			TextMessage: matrix.TextMessage{
				Body:          msg.Text,
				MsgType:       "m.text",
				Format:        "org.matrix.custom.html",
				FormattedBody: helper.ParseMarkdown(formattedText),
			},
		}

		rmsg.NewContent = SubTextMessage{
			Body:          rmsg.TextMessage.Body,
			FormattedBody: rmsg.TextMessage.FormattedBody,
			Format:        rmsg.TextMessage.Format,
			MsgType:       "m.text",
		}

		if b.GetBool("HTMLDisable") {
			rmsg.TextMessage.Format = ""
			rmsg.TextMessage.FormattedBody = ""
			rmsg.NewContent.Format = ""
			rmsg.NewContent.FormattedBody = ""
		}

		rmsg.RelatedTo = MessageRelation{
			EventID: msg.ID,
			Type:    "m.replace",
		}

		err := b.retry(func() error {
			_, err := mc.SendMessageEvent(channel, "m.room.message", rmsg)

			return err
		})
		if err != nil {
			return "", err
		}

		return msg.ID, nil
	}

	// Use notices to send join/leave events
	if msg.Event == config.EventJoinLeave {
		m := matrix.TextMessage{
			MsgType:       "m.notice",
			Body:          msg.Text,
			FormattedBody: formattedText,
			Format:        "org.matrix.custom.html",
		}

		if b.GetBool("HTMLDisable") {
			m.Format = ""
			m.FormattedBody = ""
		}

		var (
			resp *matrix.RespSendEvent
			err  error
		)

		err = b.retry(func() error {
			resp, err = mc.SendMessageEvent(channel, "m.room.message", m)

			return err
		})
		if err != nil {
			return "", err
		}

		return resp.EventID, err
	}

	if msg.ParentValid() {
		m := ReplyMessage{
			TextMessage: matrix.TextMessage{
				MsgType:       "m.text",
				Body:          msg.Text,
				FormattedBody: helper.ParseMarkdown(formattedText),
				Format:        "org.matrix.custom.html",
			},
		}

		if b.GetBool("HTMLDisable") {
			m.TextMessage.Format = ""
			m.TextMessage.FormattedBody = ""
		}

		m.RelatedTo = InReplyToRelation{
			InReplyTo: InReplyToRelationContent{
				EventID: msg.ParentID,
			},
		}

		var (
			resp *matrix.RespSendEvent
			err  error
		)

		err = b.retry(func() error {
			resp, err = mc.SendMessageEvent(channel, "m.room.message", m)

			return err
		})
		if err != nil {
			return "", err
		}

		return resp.EventID, err
	}

	if b.GetBool("HTMLDisable") {
		var (
			resp *matrix.RespSendEvent
			err  error
		)

		err = b.retry(func() error {
			resp, err = mc.SendText(channel, msg.Text)

			return err
		})
		if err != nil {
			return "", err
		}

		return resp.EventID, err
	}

	// Post normal message with HTML support (eg riot.im)
	var (
		resp *matrix.RespSendEvent
		err  error
	)

	err = b.retry(func() error {
		resp, err = mc.SendFormattedText(channel, msg.Text,
			helper.ParseMarkdown(formattedText))

		return err
	})
	if err != nil {
		return "", err
	}

	return resp.EventID, err
}

func (b *AppServMatrix) handlematrix() {
	syncer := b.mc.Syncer.(*matrix.DefaultSyncer)
	syncer.OnEventType("m.room.redaction", b.handleEvent)
	syncer.OnEventType("m.room.message", b.handleEvent)
	syncer.OnEventType("m.room.member", b.handleMemberChange)
	go func() {
		for {
			if b == nil {
				return
			}
			if err := b.mc.Sync(); err != nil {
				b.Log.Println("Sync() returned ", err)
			}
		}
	}()
}

func (b *AppServMatrix) handleEdit(ev *matrix.Event, rmsg config.Message) bool {
	relationInterface, present := ev.Content["m.relates_to"]
	newContentInterface, present2 := ev.Content["m.new_content"]
	if !(present && present2) {
		return false
	}

	var relation MessageRelation
	if err := interface2Struct(relationInterface, &relation); err != nil {
		b.Log.Warnf("Couldn't parse 'm.relates_to' object with value %#v", relationInterface)
		return false
	}

	var newContent SubTextMessage
	if err := interface2Struct(newContentInterface, &newContent); err != nil {
		b.Log.Warnf("Couldn't parse 'm.new_content' object with value %#v", newContentInterface)
		return false
	}

	if relation.Type != "m.replace" {
		return false
	}

	rmsg.ID = relation.EventID
	rmsg.Text = newContent.Body
	b.Remote <- rmsg

	return true
}

func (b *AppServMatrix) handleReply(ev *matrix.Event, rmsg config.Message) bool {
	relationInterface, present := ev.Content["m.relates_to"]
	if !present {
		return false
	}

	var relation InReplyToRelation
	if err := interface2Struct(relationInterface, &relation); err != nil {
		// probably fine
		return false
	}

	body := rmsg.Text

	if !b.GetBool("keepquotedreply") {
		for strings.HasPrefix(body, "> ") {
			lineIdx := strings.IndexRune(body, '\n')
			if lineIdx == -1 {
				body = ""
			} else {
				body = body[(lineIdx + 1):]
			}
		}
	}

	rmsg.Text = body
	rmsg.ParentID = relation.InReplyTo.EventID
	b.Remote <- rmsg

	return true
}

func (b *AppServMatrix) handleMemberChange(ev *matrix.Event) {
	// Update the displayname on join messages, according to https://matrix.org/docs/spec/client_server/r0.6.1#events-on-change-of-profile-information
	if ev.Content["membership"] == "join" {
		if dn, ok := ev.Content["displayname"].(string); ok {
			b.cacheDisplayName(ev.Sender, dn)
		}
	}
}

func (b *AppServMatrix) handleEvent(ev *matrix.Event) {
	if strings.Contains(ev.Sender, b.GetString("ApsPrefix")) || ev.Sender == string(b.apsCli.UserID) {
		return
	}

	b.Log.Debugf("== Receiving event: %#v", ev)
	if ev.Sender == b.UserID {
		return
	}
	channel, ok := b.getRoomMapChannel(ev.RoomID)
	if !ok {
		b.Log.Debugf("Unknown room %s", ev.RoomID)
		return
	}

	// Create our message
	rmsg := config.Message{
		Username: b.getDisplayName(ev.Sender),
		Channel:  channel,
		Account:  b.Account,
		UserID:   ev.Sender,
		ID:       ev.ID,
		Protocol: "appservice",

		//	Avatar:   b.getAvatarURL(ev.Sender),
	}
	rmsg.TargetPlatform = b.RemoteProtocol
	if channelInfo, ok := b.getChannelInfo(channel); ok {
		b.RLock()
		if channelInfo.IsDirect {
			rmsg.Event = "direct_msg"
		}
		rmsg.ChannelId = channelInfo.RemoteID
		rmsg.ChannelName = channelInfo.RemoteName
		b.adjustChannel(&rmsg)
		b.RUnlock()
	}
	// Remove homeserver suffix if configured
	if b.GetBool("NoHomeServerSuffix") {
		re := regexp.MustCompile("(.*?):.*")
		rmsg.Username = re.ReplaceAllString(rmsg.Username, `$1`)
	}

	// Delete event
	if ev.Type == "m.room.redaction" {
		rmsg.Event = config.EventMsgDelete
		rmsg.ID = ev.Redacts
		rmsg.Text = config.EventMsgDelete
		b.Remote <- rmsg
		return
	}

	// Text must be a string
	if rmsg.Text, ok = ev.Content["body"].(string); !ok {
		b.Log.Errorf("Content[body] is not a string: %T\n%#v",
			ev.Content["body"], ev.Content)
		return
	}
	var htmlText string
	if htmlText, ok = ev.Content["formatted_body"].(string); !ok {
		b.Log.Errorf("Content[formatted_body] is not a string: %T\n%#v",
			ev.Content["formatted_body"], ev.Content)

	}
	log.Println(htmlText)
	rmsg.Text, rmsg.Mentions = b.outcomingMention(b.RemoteProtocol, rmsg.Text, htmlText)

	if rmsg.Channel == b.GetString("ApsPrefix")+"appservice_control" {

		if strings.HasPrefix(rmsg.Text, "/appservice") {
			sl := strings.Split(rmsg.Text, " ")
			if len(sl) == 3 {
				if sl[1] == "join" {
					rmsg.Channel = sl[2]
					rmsg.Event = sl[1]
				}
			}
		}
	}
	// Do we have a /me action
	if ev.Content["msgtype"].(string) == "m.emote" {
		rmsg.Event = config.EventUserAction
	}

	// Is it an edit?
	if b.handleEdit(ev, rmsg) {
		return
	}

	// Is it a reply?
	if b.handleReply(ev, rmsg) {
		return
	}

	// Do we have attachments
	if b.containsAttachment(ev.Content) {
		err := b.handleDownloadFile(&rmsg, ev.Content)
		if err != nil {
			b.Log.Errorf("download failed: %#v", err)
		}
	}

	b.Log.Debugf("<= Sending message from %s on %s to gateway", ev.Sender, b.Account)
	b.Remote <- rmsg

	// not crucial, so no ratelimit check here
	if err := b.apsCli.MarkRead(id.RoomID(ev.RoomID), id.EventID(ev.ID)); err != nil {
		b.Log.Errorf("couldn't mark message as read %s", err.Error())
	}

}

// handleDownloadFile handles file download
func (b *AppServMatrix) handleDownloadFile(rmsg *config.Message, content map[string]interface{}) error {
	var (
		ok                        bool
		url, name, msgtype, mtype string
		info                      map[string]interface{}
		size                      float64
	)

	rmsg.Extra = make(map[string][]interface{})
	if url, ok = content["url"].(string); !ok {
		return fmt.Errorf("url isn't a %T", url)
	}
	url = strings.Replace(url, "mxc://", b.GetString("Server")+"/_matrix/media/v1/download/", -1)

	if info, ok = content["info"].(map[string]interface{}); !ok {
		return fmt.Errorf("info isn't a %T", info)
	}
	if size, ok = info["size"].(float64); !ok {
		return fmt.Errorf("size isn't a %T", size)
	}
	if name, ok = content["body"].(string); !ok {
		return fmt.Errorf("name isn't a %T", name)
	}
	if msgtype, ok = content["msgtype"].(string); !ok {
		return fmt.Errorf("msgtype isn't a %T", msgtype)
	}
	if mtype, ok = info["mimetype"].(string); !ok {
		return fmt.Errorf("mtype isn't a %T", mtype)
	}

	// check if we have an image uploaded without extension
	if !strings.Contains(name, ".") {
		if msgtype == "m.image" {
			mext, _ := mime.ExtensionsByType(mtype)
			if len(mext) > 0 {
				name += mext[0]
			}
		} else {
			// just a default .png extension if we don't have mime info
			name += ".png"
		}
	}

	// actually download the file
	data, err := helper.DownloadFile(url)
	if err != nil {
		return fmt.Errorf("download %s failed %#v", url, err)
	}
	// add the downloaded data to the message
	helper.HandleDownloadData(b.Log, rmsg, name, "", url, data, b.General)
	return nil
}

// handleUploadFiles handles native upload of files.
func (b *AppServMatrix) handleUploadFiles(msg *config.Message, channel string) (string, error) {
	for _, f := range msg.Extra["file"] {
		if fi, ok := f.(config.FileInfo); ok {
			b.handleUploadFile(msg, channel, &fi)
		}
	}
	return "", nil
}

// handleUploadFile handles native upload of a file.
func (b *AppServMatrix) handleUploadFile(msg *config.Message, channel string, fi *config.FileInfo) {
	mtxInfo, ok := b.getUserMapInfo(msg.UserID)
	if !ok {
		b.Log.Errorf("userID %s for name %s not exist in the appservice database", msg.UserID, msg.Username)

	}
	mc, errmtx := b.newVirtualUserMtxClient(mtxInfo.MtxID)
	if errmtx != nil {
		b.Log.Debug(errmtx)
		mc, errmtx = matrix.NewClient(b.GetString("Server"), string(b.apsCli.UserID), b.apsCli.AccessToken)
		if errmtx != nil {
			b.Log.Debug(errmtx)
			return
		}
	}
	content := bytes.NewReader(*fi.Data)
	sp := strings.Split(fi.Name, ".")
	mtype := mime.TypeByExtension("." + sp[len(sp)-1])
	// image and video uploads send no username, we have to do this ourself here #715

	var err error
	b.Log.Debugf("uploading file: %s %s", fi.Name, mtype)

	var res *matrix.RespMediaUpload

	err = b.retry(func() error {
		res, err = mc.UploadToContentRepo(content, mtype, int64(len(*fi.Data)))

		return err
	})

	if err != nil {
		b.Log.Errorf("file upload failed: %#v", err)
		return
	}

	switch {
	case strings.Contains(mtype, "video"):
		b.Log.Debugf("sendVideo %s", res.ContentURI)
		err = b.retry(func() error {
			_, err = mc.SendVideo(channel, fi.Name, res.ContentURI)

			return err
		})
		if err != nil {
			b.Log.Errorf("sendVideo failed: %#v", err)
		}
	case strings.Contains(mtype, "image"):
		b.Log.Debugf("sendImage %s", res.ContentURI)
		err = b.retry(func() error {
			_, err = mc.SendImage(channel, fi.Name, res.ContentURI)

			return err
		})
		if err != nil {
			b.Log.Errorf("sendImage failed: %#v", err)
		}
	case strings.Contains(mtype, "audio"):
		b.Log.Debugf("sendAudio %s", res.ContentURI)
		err = b.retry(func() error {
			_, err = mc.SendMessageEvent(channel, "m.room.message", matrix.AudioMessage{
				MsgType: "m.audio",
				Body:    fi.Name,
				URL:     res.ContentURI,
				Info: matrix.AudioInfo{
					Mimetype: mtype,
					Size:     uint(len(*fi.Data)),
				},
			})

			return err
		})
		if err != nil {
			b.Log.Errorf("sendAudio failed: %#v", err)
		}
	default:
		b.Log.Debugf("sendFile %s", res.ContentURI)
		err = b.retry(func() error {
			_, err = mc.SendMessageEvent(channel, "m.room.message", matrix.FileMessage{
				MsgType: "m.file",
				Body:    fi.Name,
				URL:     res.ContentURI,
				Info: matrix.FileInfo{
					Mimetype: mtype,
					Size:     uint(len(*fi.Data)),
				},
			})

			return err
		})
		if err != nil {
			b.Log.Errorf("sendFile failed: %#v", err)
		}
	}
	b.Log.Debugf("result: %#v", res)
}
