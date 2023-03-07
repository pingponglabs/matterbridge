package btwitter

import (
	"github.com/dghubble/go-twitter/twitter"
)

type webhookEvent struct {
	ForUserID           string                         `json:"for_user_id"`
	DirectMessageEvents []*twitter.DirectMessageEvent `json:"direct_message_events"`
	Users               map[string]twitterUser       `json:"users"`

	DirectMessageIndicateTypingEvents []DirectMessageIndicateTypingEvents `json:"direct_message_indicate_typing_events"`
}

type DirectMessageIndicateTypingEvents struct {
	CreatedTimestamp string `json:"created_timestamp"`
	SenderID         string `json:"sender_id"`
	Target           struct {
		RecipientID string `json:"recipient_id"`
	} `json:"target"`
}
