package bappservice

import (
	"errors"
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
		Username:    member,
		MatrixToken: resp.AccessToken,
		MatrixID:    string(resp.UserID),
		UserID:      remoteID,
		Registred:   true,
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
	var newMembers = make(map[string]string)
	for k, v := range members {
		exist, err := b.DbStore.getUserByID(k)
		if err != nil {
			b.Log.Errorf("Error getting user info from database: %v", err)

		}
		if exist == nil {
			newMembers[k] = v
		}

	}
	return newMembers

}

func (b *AppServMatrix) addNewMembers(channel string, newMembers map[string]string) {
	_, ok := b.getChannelInfo(channel)
	if !ok {
		return
	}
	for k, _ := range newMembers {
		b.addNewMember(channel, k)
	}

}
func (b *AppServMatrix) addNewMember(channel, userID string) {
	exist, err := b.DbStore.isUserInChannel(channel, userID)
	if err != nil {
		b.Log.Errorf("Error getting user info from database: %v", err)

	}
	if !exist {
		b.DbStore.setUserForChannel(channel, userID, false)
	}

}

func (b *AppServMatrix) getVirtualUserInfo(userID string) (*MemberInfo, bool) {

	memberInfo, err := b.DbStore.getUserByID(userID)
	if err != nil {
		if !errors.Is(err, ErrUserNotFound) {
			b.Log.Errorf("Error getting user info from database: %v", err)
		}
		return nil, false
	}
	return memberInfo, true
}

// get virtual user from database by matrix id
func (b *AppServMatrix) getVirtualUserFromMtxId(mtxID string) (*MemberInfo, bool) {

	memberInfo, err := b.DbStore.getUserByMatrixID(mtxID)
	if err != nil {
		if !errors.Is(err, ErrUserNotFound) {
			b.Log.Errorf("Error getting user info from database: %v", err)
		}
		return nil, false
	}
	return memberInfo, true
}

// update virtual user info on database

func (b *AppServMatrix) addVirtualUser(usersInfo MemberInfo) error {

	Err := b.DbStore.registerUser(&usersInfo)
	if Err != nil {
		return Err
	}
	return nil

}

func (b *AppServMatrix) getUsernameFromMtxId(mtxID string) (*MemberInfo, bool) {
	// get user from db by matrix id

	MemberInfo, err := b.DbStore.getUserByMatrixID(mtxID)
	if err != nil {
		if !errors.Is(err, ErrUserNotFound) {
			b.Log.Errorf("Error getting user info from database: %v", err)
		}
		return nil, false
	}
	return MemberInfo, true

}
func (b *AppServMatrix) newVirtualUserMtxClient(mtxID string) (*matrix.Client, error) {
	b.RLock()
	defer b.RUnlock()

	if val, ok := b.getUsernameFromMtxId(mtxID); ok {
		return matrix.NewClient(b.GetString("Server"), val.MatrixID, val.MatrixToken)
	}
	return nil, fmt.Errorf(" username %s not exist on the appservice database", mtxID)
}

func (b *AppServMatrix) getUserMapInfo(userID string) (UserMapInfo, bool) {
	userInfo, err := b.DbStore.getUserByID(userID)
	if err != nil {
		if !errors.Is(err, ErrUserNotFound) {
			b.Log.Errorf("Error getting user info from database: %v", err)
		}
		return UserMapInfo{}, false
	}
	return UserMapInfo{
		Username: userInfo.Username,
		MtxID:    userInfo.MatrixID,
	}, true
}
func (b *AppServMatrix) addUserMapInfo(UserID string, userInfo UserMapInfo) {
	b.Lock()
	b.UsersMap[UserID] = userInfo
	b.Unlock()

}
