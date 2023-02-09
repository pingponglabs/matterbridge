package bappservice

import "github.com/42wim/matterbridge/bridge/config"

func (a *AppServMatrix) controllAction(msg config.Message) {

	switch msg.Event {
	case "join":
		a.Remote <- config.Message{
			Text:     "join",
			Channel:  msg.ChannelName,
			Username: a.Name,
			UserID:   a.UserID,
			Account:  a.Account,
			Event:    "join",
			Protocol: "appservice",
			ExtraNetworkInfo: config.ExtraNetworkInfo{
				TargetPlatform: a.RemoteProtocol,
				ChannelName:    msg.ChannelName,
			},
		}
	case "facebook-event","twitter-event","email-event":
		msg.Protocol = "appservice"
		msg.TargetPlatform = a.RemoteProtocol
		msg.Username = a.Name
		msg.UserID = a.UserID
		msg.Account = a.Account
		a.Remote <- msg
	
	}

}
