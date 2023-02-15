package bappservice

import (
	"database/sql"
	"errors"
)

// store is a bridge appservice store

// create the database for appservice
const (
	UserTableName                = "user"
	ChannelTableName             = "channel"
	DatabaseNamePrefix           = "appservice_"
	JunctionChannelUserTableName = "channel_user"
)

type AppServStore struct {
	db *sql.DB
}

func (b *AppServStore) NewDbConnection(driverName, dataSource string) error {
	var err error
	b.db, err = sql.Open(driverName, dataSource)
	if err != nil {
		return err
	}
	err = b.db.Ping()
	if err != nil {
		return err
	}
	return nil
}
func (b *AppServStore) CreateTables() error {
	err := b.createInfoTable("info")
	if err != nil {
		return err
	}
	err = b.createChannelTable(ChannelTableName)
	if err != nil {
		return err
	}
	err = b.createVirtualUsersTable(UserTableName)
	if err != nil {
		return err
	}
	err = b.createChanelVirtualUserJunctionTable(JunctionChannelUserTableName)
	return nil
}

/*
func (b *AppServStore) createDatabase(name string) error {
	_, err := b.db.Exec(`create database if not exists ` + DatabaseNamePrefix + name + `;`)
	return err
}
*/

type appserviceTable struct {
	RoomsInfo    map[string]*ChannelInfo `json:"rooms_info,omitempty"`
	VirtualUsers map[string]MemberInfo   `json:"virtual_users,omitempty"`
	UsersMap     map[string]UserMapInfo  `json:"users_map,omitempty"`

	RemoteProtocol string `json:"remote_protocol,omitempty"`
	AvatarUrl      string `json:"avatar_url,omitempty"`
}

func (b *AppServStore) createAppserviceTable(name string) error {

	_, err := b.db.Exec(`create table if not exists info (
		id INTEGER not null autoincrement primary key,
		room
	);`)
	return err
}
func (b *AppServStore) createChannelTable(name string) error {

	_, err := b.db.Exec(`CREATE TABLE IF NOT EXISTS channel (
		id INTEGER NOT NULL  PRIMARY KEY AUTOINCREMENT,
		remote_name varchar(255) NOT NULL,
		matrix_room_id varchar(255) UNIQUE NOT NULL,
		is_direct BOOLEAN NOT NULL,
		remote_id varchar(255) UNIQUE NOT NULL,
		UNIQUE (id)
	);`)
	return err
}

func (b *AppServStore) createVirtualUsersTable(name string) error {

	_, err := b.db.Exec(`CREATE TABLE IF NOT EXISTS user (
		id INTEGER NOT NULL  PRIMARY KEY AUTOINCREMENT,
		username varchar(255) NOT NULL,
		matrix_token varchar(255) NOT NULL,
		matrix_id varchar(255) UNIQUE NOT NULL,
		remote_id varchar(255) UNIQUE NOT NULL,
		registred BOOLEAN NOT NULL,
		UNIQUE (id)

	);`)
	return err
}
func (b *AppServStore) createInfoTable(name string) error {
	_, err := b.db.Exec(`CREATE TABLE IF NOT EXISTS ` + name + ` (
		id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		remote_protocol VARCHAR(255) NOT NULL,
		avatar_url VARCHAR(255) NOT NULL
	);`)
	return err
}
func (b *AppServStore) createChanelVirtualUserJunctionTable(name string) error {
	_, err := b.db.Exec(`
		CREATE TABLE channel_user (
		id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		channel_id    INTEGER REFERENCES channel (remote_id) ON UPDATE CASCADE,
		user_id INTEGER REFERENCES user (remote_id) ON UPDATE CASCADE,
		joined BOOLEAN NOT NULL,
		UNIQUE (ID)
	);`)
	return err
}

func (b *AppServStore) createChannel(channel *ChannelInfo) error {
	_, err := b.db.Exec(`insert into `+ChannelTableName+` (remote_name, matrix_room_id, is_direct, remote_id) values ($1, $2, $3, $4)`, channel.RemoteName, channel.MatrixRoomID, channel.IsDirect, channel.RemoteID)
	return err
}

var ErrChannelNotFound = errors.New("channel not found")

// getChannelByName returns a channel by its name
func (b *AppServStore) getChannelByName(name string) (*ChannelInfo, error) {
	var channel ChannelInfo
	err := b.db.QueryRow(`select remote_name, matrix_room_id, is_direct, remote_id from `+ChannelTableName+` where remote_name = $1`, name).Scan(&channel.RemoteName, &channel.MatrixRoomID, &channel.IsDirect, &channel.RemoteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = ErrChannelNotFound
		}
	}
	return &channel, err
}

// getChannelByID returns a channel by its ID
func (b *AppServStore) getChannelByID(id string) (*ChannelInfo, error) {
	var channel ChannelInfo
	err := b.db.QueryRow(`select remote_name, matrix_room_id, is_direct, remote_id from `+ChannelTableName+` where remote_id = $1`, id).Scan(&channel.RemoteName, &channel.MatrixRoomID, &channel.IsDirect, &channel.RemoteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = ErrChannelNotFound
		}
	}
	return &channel, err
}

// getChannelByMatrixID returns a channel by its Matrix ID
func (b *AppServStore) getChannelByMatrixID(id string) (*ChannelInfo, error) {
	var channel ChannelInfo
	err := b.db.QueryRow(`select remote_name, matrix_room_id, is_direct, remote_id from `+ChannelTableName+` where matrix_room_id = $1`, id).Scan(&channel.RemoteName, &channel.MatrixRoomID, &channel.IsDirect, &channel.RemoteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = ErrChannelNotFound
		}
	}
	return &channel, err
}

// updateChannel updates a channel
func (b *AppServStore) updateChannel(channel *ChannelInfo) error {
	_, err := b.db.Exec(`update `+ChannelTableName+` set remote_name = $1, matrix_room_id = $2, is_direct = $3, remote_id = $4 where remote_name = $1`, channel.RemoteName, channel.MatrixRoomID, channel.IsDirect, channel.RemoteID)
	return err
}

var ErrUserNotFound = errors.New("user not found")

// get user by its name
func (b *AppServStore) getUserbyName(name string) (*MemberInfo, error) {
	var user MemberInfo
	err := b.db.QueryRow(`select username, matrix_token, matrix_id, remote_id, registred from `+UserTableName+` where username = $1`, name).Scan(&user.Username, &user.MatrixToken, &user.MatrixID, &user.UserID, &user.Registred)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrUserNotFound
	}
	return &user, err
}

// get user by its id
func (b *AppServStore) getUserByID(id string) (*MemberInfo, error) {
	var user MemberInfo
	err := b.db.QueryRow(`select username, matrix_token, matrix_id, remote_id, registred from `+UserTableName+` where remote_id = $1`, id).Scan(&user.Username, &user.MatrixToken, &user.MatrixID, &user.UserID, &user.Registred)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrUserNotFound
	}
	return &user, err
}

// get user by its matrix id

func (b *AppServStore) getUserByMatrixID(id string) (*MemberInfo, error) {
	var user MemberInfo
	err := b.db.QueryRow(`select username, matrix_token, matrix_id, remote_id, registred from `+UserTableName+` where matrix_id = $1`, id).Scan(&user.Username, &user.MatrixToken, &user.MatrixID, &user.UserID, &user.Registred)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrUserNotFound
	}
	return &user, err
}

// register user
func (b *AppServStore) registerUser(user *MemberInfo) error {
	_, err := b.db.Exec(`insert into `+UserTableName+` (username, matrix_token, matrix_id, remote_id, registred) values ($1, $2, $3, $4, $5)`, user.Username, user.MatrixToken, user.MatrixID, user.UserID, user.Registred)
	return err
}

// update user
func (b *AppServStore) updateUser(user *MemberInfo) error {
	_, err := b.db.Exec(`update `+UserTableName+` set username = $1, matrix_token = $2, matrix_id = $3, remote_id = $4, registred = $5 where username = $1`, user.Username, user.MatrixToken, user.MatrixID, user.UserID, user.Registred)
	return err
}

// update user joined status

// get channels for a user
func (b *AppServStore) getChannelsForUser(userID string) ([]*ChannelInfo, error) {
	rows, err := b.db.Query(`select remote_name, matrix_room_id, is_direct, remote_id from `+ChannelTableName+` where remote_id in (select channel_id from channel_user where user_id = $1)`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []*ChannelInfo
	for rows.Next() {
		var channel ChannelInfo
		err := rows.Scan(&channel.RemoteName, &channel.MatrixRoomID, &channel.IsDirect, &channel.RemoteID)
		if err != nil {
			return nil, err
		}
		channels = append(channels, &channel)
	}
	return channels, nil
}

// check if a user is in a channel
func (b *AppServStore) isUserInChannel(channelID, userID string) (bool, error) {
	var count int
	err := b.db.QueryRow(`select count(*) from channel_user where channel_id = $1 and user_id = $2`, channelID, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// set user for a channel
func (b *AppServStore) setUserForChannel(channelID, userID string, joined bool) error {
	_, err := b.db.Exec(`insert into channel_user (channel_id, user_id, joined) values ($1, $2, $3)`, channelID, userID, joined)

	return err
}

// set user for a channel by its matrix id
func (b *AppServStore) setUserForChannelByMatrixID(channelID, matrixID string, joined bool) error {
	_, err := b.db.Exec(`insert into channel_user (channel_id, user_id, joined) values ($1, (select remote_id from user where matrix_id = $2), $3)`, channelID, matrixID, joined)
	return err
}

// get users for a channel
func (b *AppServStore) getUsersForChannel(channelID string) ([]*MemberInfo, error) {
	// use left join
	rows, err := b.db.Query(`select user.username, user.matrix_token, user.matrix_id, user.remote_id, user.registred, channel_user.joined from user inner join channel_user on  user.remote_id=channel_user.user_id  where channel_user.channel_id = select channel_id = $1`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*MemberInfo
	for rows.Next() {
		var user MemberInfo
		err := rows.Scan(&user.Username, &user.MatrixToken, &user.MatrixID, &user.UserID, &user.Registred)
		if err != nil {
			return nil, err
		}
		users = append(users, &user)
	}
	return users, nil
}
func (b *AppServStore) getChannelMembersState(channelID string) ([]ChannelMember, error) {
	// select channel_id,remote_id,joined
	rows, err := b.db.Query(`select channel_id, user_id, joined from channel_user where channel_id = $1`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []ChannelMember
	for rows.Next() {
		var member ChannelMember
		err := rows.Scan(&member.ChannelID, &member.UserID, &member.Joined)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, nil

}

// set channel for a user
func (b *AppServStore) setChannelForUser(channelID, userID string, joined bool) error {
	_, err := b.db.Exec(`insert into channel_user (channel_id, user_id, joined) values ($1, $2, $3)`, channelID, userID, joined)
	return err
}

// remove user and channel relation on channel_user table
func (b *AppServStore) removeUserFromChannel(channelID, userID string) error {
	_, err := b.db.Exec(`delete from channel_user where channel_id=$1 and user_id= $2`, channelID, userID)
	return err
}

// remove channel and user relation on channel_user table
func (b *AppServStore) removeChannelFromUser(channelID, userID string) error {
	_, err := b.db.Exec(`delete from channel_user where channel_id=$1 and user_id= $2`, channelID, userID)
	return err
}

// update user joined status
func (b *AppServStore) updateUserJoinedStatus(mtxID, MatrixRoomID string, joined bool) error {
	_, err := b.db.Exec(`update channel_user set joined = $1 where channel_id = (select channel_id from `+ChannelTableName+` where matrix_room_id = $2) and user_id = (select remote_id from `+UserTableName+` where matrix_id = $3)`, joined, MatrixRoomID, mtxID)
	return err
}

// intellesense for  sql queries
//
