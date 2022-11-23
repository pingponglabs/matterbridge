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

func (b *AppServMatrix) createVirtualUsers(member, remoteID string) (MemberInfo, error) {
	// TODO mapping for username
	ValidUsername := member
	if !validUsernameRegex.MatchString(member) {
		ValidUsername = AlterUsername(member)
	}
	ValidUsername += "_" + randNumberRunes(3)
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
	return MemberInfo{
		Channels:  []ChannelsInvite{},
		Username:  member,
		Token:     resp.AccessToken,
		Id:        string(resp.UserID),
		RemoteId:  remoteID,
		Registred: true,
	}, nil

}
func AlterUsername(s string) string {
	for _, v := range s {
		if !strings.Contains(allowedChars, string(v)) {
			s = strings.ReplaceAll(s, string(v), "__")
		}
	}
	return s
}

func (b *AppServMatrix) GetNotExistUsers(members map[string]string) map[string]string {
	list := map[string]string{}
	for k, v := range members {
		if _, ok := b.getUserMapInfo(k); !ok {
			list[k] = v
		}
	}
	return list
}

func (b *AppServMatrix) addNewMembers(channel string, newMembers map[string]string) {
	roomInfo, ok := b.getRoomInfo(channel)
	if !ok {
		return
	}
	b.Lock()
	defer b.Unlock()
	for k, v := range newMembers {
		exist := false
		for _, m := range roomInfo.Members {
			if k == m.UserID {
				exist = true
				break
			}
		}
		if !exist {
			roomInfo.Members[k] = ChannelMember{
				Name:   v,
				UserID: k,
				Joined: false,
			}
		}
	}

}
func (b *AppServMatrix) addNewMember(channel, newMember, userID string) {
	roomInfo, ok := b.getRoomInfo(channel)
	if !ok {
		return
	}
	b.Lock()

	roomInfo.Members[userID] = ChannelMember{
		Name:   newMember,
		UserID: userID,
		Joined: false,
	}
	b.Unlock()

}

func (b *AppServMatrix) getVirtualUserInfo(mtxID string) (MemberInfo, bool) {
	b.RLock()
	defer b.RUnlock()

	for k, v := range b.virtualUsers {
		if k == mtxID {
			return v, true
		}
	}
	return MemberInfo{}, false
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
func (b *AppServMatrix) newVirtualUserMtxClient(mtxID string) (*matrix.Client, error) {
	b.RLock()
	defer b.RUnlock()

	if val, ok := b.virtualUsers[mtxID]; ok {
		return matrix.NewClient(b.GetString("Server"), val.Id, val.Token)
	}
	return nil, fmt.Errorf(" username %s not exist on the appservice database", mtxID)
}

func (b *AppServMatrix) addUsersId(usersId map[string]string) {
	for k, v := range usersId {
		if userInfo, ok := b.virtualUsers[k]; ok {

			userInfo.RemoteId = v
			b.virtualUsers[k] = userInfo
		}
	}
	b.saveState()
}
func (b *AppServMatrix) getUserMapInfo(userID string) (UserMapInfo, bool) {
	b.RLock()
	defer b.RUnlock()

	if v, ok := b.UsersMap[userID]; ok {
		return v, true

	}
	return UserMapInfo{}, false
}
func (b *AppServMatrix) addUserMapInfo(UserID string, userInfo UserMapInfo) {
	b.Lock()
	b.UsersMap[UserID] = userInfo
	b.Unlock()

}
