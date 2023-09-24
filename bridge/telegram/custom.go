package btelegram

import (
	"strconv"
	"strings"

	"github.com/42wim/matterbridge/bridge/config"

	tgbotapi "github.com/matterbridge/telegram-bot-api/v6"
)

func (b *Btelegram) HandleSendMentions(message *tgbotapi.Message, usersId, mentions map[string]string) {

	for _, v := range message.Entities {
		if v.Type == "text_mention" {
			mentionText := message.Text[v.Offset : v.Length+v.Offset]
			username := v.User.FirstName + " " + v.User.LastName
			userId := strconv.FormatInt(v.User.ID, 10)
			mentions[mentionText] = username
			usersId[username] = userId

		}
	}
}
func (b *Btelegram) HandleReceiveMentions(message config.Message) []tgbotapi.MessageEntity {
	ent := []tgbotapi.MessageEntity{}
	for k, v := range message.Mentions {
		offset := strings.Index(message.Text, k)
		length := len(k)
		userId, _ := strconv.Atoi(v)
		sl := strings.Split(k, " ")
		firstName := sl[0]
		lastName := ""
		if len(sl) > 1 {
			lastName = sl[1]
		}
		ent = append(ent, tgbotapi.MessageEntity{
			Type:   "text_mention",
			Offset: offset,
			Length: length,
			URL:    "",
			User: &tgbotapi.User{
				ID:                      int64(userId),
				IsBot:                   false,
				FirstName:               firstName,
				LastName:                lastName,
				UserName:                k,
				LanguageCode:            "en",
				CanJoinGroups:           false,
				CanReadAllGroupMessages: false,
				SupportsInlineQueries:   false,
			},
			Language: "en",
		})

	}
	return ent
}
func (b *Btelegram) sendApsMessage(chatid int64, username, text string, parentID int, rmsg config.Message) (string, error) {
	m := tgbotapi.NewMessage(chatid, "")
	m.Text, m.ParseMode = text, tgbotapi.ModeHTML
	m.ReplyToMessageID = parentID
	m.DisableWebPagePreview = b.GetBool("DisableWebPagePreview")
	m.Entities = b.HandleReceiveMentions(rmsg)

	res, err := b.c.Send(m)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(res.MessageID), nil
}
