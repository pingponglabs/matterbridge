package bappservice

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	matrix "github.com/matterbridge/gomatrix"
	gomatrix "maunium.net/go/mautrix"
)

var validUsernameRegex = regexp.MustCompile(`^[0-9a-zA-Z_\-=./]+$`)
var allowedChars = "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "0123456789" + "_-=./"

func (b *AppServMatrix) createVirtualUsers(member string) (MemberInfo, error) {
	// TODO mapping for username
	ValidUsername := member
	if !validUsernameRegex.MatchString(member) {
		ValidUsername = AlterUsername(member)
	}
	ValidUsername += "_" + randStringLowerRunes(3)

	resp, _, err := b.apsCli.Register(&gomatrix.ReqRegister{
		Username: b.GetString("ApsPrefix") + ValidUsername + b.GetString("UserSuffix"),
		Type:     "m.login.application_service",
	})
	if err != nil {
		return MemberInfo{}, err
	}
	mc, err := matrix.NewClient(b.GetString("Server"), string(resp.UserID), resp.AccessToken)
	if err != nil {
		log.Println(err)
	}
	err = mc.SetDisplayName(member)
	if err != nil {
		log.Println(err)
	}
	return MemberInfo{Channels: []ChannelsInvite{},
		Token:     resp.AccessToken,
		Id:        string(resp.UserID),
		RemoteId:  "",
		Registred: true}, nil

}
func AlterUsername(s string) string {
	for _, v := range s {
		if !strings.Contains(allowedChars, string(v)) {
			s = strings.ReplaceAll(s, string(v), "__")
		}
	}
	return s
}
func (b *AppServMatrix) UsersState(members []userListState) {
	for i := range members {
		if userInfo, ok := b.getVirtualUserInfo(members[i].name); ok {
			if userInfo.Registred {
				members[i].Registred = true
			}
		} else {
			members[i].NotExist = true

		}

	}
}
func (b *AppServMatrix) GetNotExistUsers(members []string) []string {
	var list []string
	for _, v := range members {
		if _, ok := b.getVirtualUserInfo(v); !ok {
			list = append(list, v)
		}
	}
	return list
}

func (b *AppServMatrix) channelsJoinedUsersState(channel string, externMembers []userListState) {
	roomInfo, ok := b.getRoomInfo(channel)
	if !ok {
		return
	}
	for i := range externMembers {
		exist := false
		for _, member := range roomInfo.Members {
			if externMembers[i].name == member.Name {
				exist = true
				break
			}

		}
		if exist {
			externMembers[i].Joined = true
		}
	}
}

func (b *AppServMatrix) addNewMembers(channel string, newMembers []string) {
	roomInfo, ok := b.getRoomInfo(channel)
	if !ok {
		return
	}

	for _, v := range newMembers {
		exist := false
		for _, m := range roomInfo.Members {
			if v == m.Name {
				exist = true
				break
			}
		}
		if !exist {
			roomInfo.Members = append(roomInfo.Members, ChannelMember{Name: v})

		}
	}

}
func (b *AppServMatrix) addNewMember(channel, newMembers string) {
	roomInfo, ok := b.getRoomInfo(channel)
	if !ok {
		return
	}
	b.Lock()

	roomInfo.Members = append(roomInfo.Members, ChannelMember{Name: newMembers})
	b.Unlock()

}

func (b *AppServMatrix) getVirtualUserInfo(username string) (MemberInfo, bool) {
	b.RLock()
	defer b.RUnlock()

	for k, v := range b.virtualUsers {
		if k == username {
			return v, true
		}
	}
	return MemberInfo{}, false
}
func (b *AppServMatrix) addVirtualUsers(usersListState []userListState) {

	for i := range usersListState {
		if usersListState[i].NotExist {
			b.Lock()
			b.virtualUsers[usersListState[i].name] = MemberInfo{
				Token:     "",
				Id:        "",
				RemoteId:  "",
				Registred: false,
			}
			usersListState[i].NotExist = false
			b.Unlock()
		}
	}
}
func (b *AppServMatrix) addVirtualUser(username string, usersInfo MemberInfo) {
	b.Lock()
	b.virtualUsers[username] = usersInfo
	b.Unlock()

}

func (b *AppServMatrix) getUsernameFromMtxId(userId string) string {
	b.RLock()
	defer b.RUnlock()

	for i, v := range b.virtualUsers {
		if userId == v.Id {
			return i
		}
	}
	return ""

}
func (b *AppServMatrix) newVirtualUserMtxClient(username string) (*matrix.Client, error) {
	b.RLock()
	defer b.RUnlock()

	if val, ok := b.virtualUsers[username]; ok {
		return matrix.NewClient(b.GetString("Server"), val.Id, val.Token)
	}
	return nil, fmt.Errorf("username not exist on the appservice database")
}

func (b *AppServMatrix) addUsersId(usersId map[string]string) {
	for k, v := range usersId {
		k = strings.TrimPrefix(k, "@")
		if userInfo, ok := b.virtualUsers[k]; ok {

			userInfo.RemoteId = v
			b.virtualUsers[k] = userInfo
		}
	}
	b.saveState()
}
