package bappservice

import (
	"fmt"
	"strings"
)

func (a *AppServMatrix) translateToMatrixMention(platform, text string) string {
	switch platform {
	case "irc":
		var htmlText string
		if sl := strings.Split(text, ":"); len(sl) > 1 {
			username := sl[0]
			if username == a.remoteUsername {
				htmlText = fmt.Sprintf("<a href='https://matrix.to/#/%s'>%s</a>", a.GetString("MainUser"), username+":")
				return strings.ReplaceAll(text, username, htmlText)
			} else {
				if userInfo, ok := a.getVirtualUserInfo(username); ok {
					htmlText = fmt.Sprintf("<a href='https://matrix.to/#/%s'>%s</a>", userInfo.Id, username+":")
					return strings.ReplaceAll(text, username, htmlText)

				}
			}

		}

	case "appservice":
		if sl := strings.Split(text, ":"); len(sl) > 1 {
			OriginUsername := sl[0]
			username := strings.Split(OriginUsername, ":")[0]
			username = username[:len(username)-len(a.GetString("UserSuffix"))]
			username = strings.ReplaceAll(username, a.GetString("ApsPrefix"), "")
			if username == strings.Split(a.GetString("MainUser"), ":")[0] {
				return strings.ReplaceAll(text, OriginUsername, username)
			}
			if _, ok := a.getVirtualUserInfo(username); ok {
				return strings.ReplaceAll(text, OriginUsername, username)
			}

		}
	case "discord":

	}

	return text
}

func (a *AppServMatrix) incomingMention(protocol string, text string, mentions map[string]string) string {

	switch protocol {
	case "irc":
		if strings.Contains(text, "@") {
			for k, v := range a.virtualUsers {

				if strings.Contains(text, k+":") {

					htmlText := fmt.Sprintf("<a href='https://matrix.to/#/%s'>%s</a>", v.Id, k)
					text = strings.ReplaceAll(text, k+":", htmlText)

				}
				if strings.Contains(text, a.remoteUsername+":") {
					htmlText := fmt.Sprintf("<a href='https://matrix.to/#/%s'>%s</a>", a.GetString("MainUser"), a.remoteUsername)

					text = strings.ReplaceAll(text, a.remoteUsername+":", htmlText)

				}
			}
		}
	case "discord":
		if strings.Contains(text, "@") {

			for k, v := range a.virtualUsers {
				if strings.Contains(text, "@"+k) {
					htmlText := fmt.Sprintf("<a href='https://matrix.to/#/%s'>%s</a>", v.Id, k)
					text = strings.ReplaceAll(text, "@"+k, htmlText)

				}
			}
			if strings.Contains(text, "@"+a.remoteUsername) {
				htmlText := fmt.Sprintf("<a href='https://matrix.to/#/%s'>%s</a>", a.GetString("MainUser"), a.remoteUsername)

				text = strings.ReplaceAll(text, "@"+a.remoteUsername, htmlText)

			}
		}

	case "telegram":
		for k, v := range mentions {
			if userInfo, ok := a.getVirtualUserInfo(v); ok {
				htmlText := fmt.Sprintf("<a href='https://matrix.to/#/%s'>%s</a>", userInfo.Id, v)
				text = strings.ReplaceAll(text, k, htmlText)
			}

		}
	}
	return text
}

// TODO handle properly the matrix name mention ( parse rhe html text instead )
func (a *AppServMatrix) outcomingMention(protocol, text, htmlText string) (string, map[string]string) {
	mentionss := make(map[string]string)

	prefix := "<a href=\"https://matrix.to/#/"
	suffix := "</a>"
	for i:=0;i<100;i++ {
		index := strings.Index(htmlText, prefix)
		if index==-1 {
			break
		}
		if indexl := strings.Index(htmlText, suffix); indexl != -1 {
			name := htmlText[index+len(prefix) : indexl]
			sl := strings.Split(name, ">")
			if len(sl) > 1 {
				sl[0] = strings.TrimSuffix(sl[0], "\"")
				mentionss[sl[0]] = sl[1]
			}
			htmlText = strings.Replace(htmlText, suffix, "", 1)

		}
		htmlText = strings.Replace(htmlText, prefix, "", 1)

	}

	mentions := make(map[string]string)
	for id, mention := range mentionss {
		for k, v := range a.virtualUsers {
			if v.Id == id {

				text = strings.ReplaceAll(text, mention, a.externMention(protocol, k))
				mentions[a.externMention(protocol, k)] = v.RemoteId
				break
			}
		}

		mainUser := a.GetString("MainUser")
		if id == mainUser {
			text = strings.ReplaceAll(text, mention, a.externMention(protocol, mainUser))
		}
	}

	return text, mentions
}
func (b *AppServMatrix) externMention(protocol, username string) string {
	switch protocol {
	case "irc":
		return username
	case "discord":

		return "@" + username + " "
	case "telegram":
		return strings.ReplaceAll(username, "-", " ")
	}

	return username
}
func (a *AppServMatrix) generateMatrixHtmlMention(username, text string) string {
	var htmlText string
	if username == a.remoteUsername {
		htmlText = fmt.Sprintf("<a href='https://matrix.to/#/%s'>%s</a>", a.GetString("MainUser"), username)
		return strings.ReplaceAll(text, username, htmlText)
	} else {
		if userInfo, ok := a.getVirtualUserInfo(username); ok {
			htmlText = fmt.Sprintf("<a href='https://matrix.to/#/%s'>%s</a>", userInfo.Id, username)
			return strings.ReplaceAll(text, username, htmlText)

		}
	}
	return text

}
