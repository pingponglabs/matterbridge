package bslack

import "strings"

func (b *Bslack) AddMention(text string) string {
	replaceFunc := func(match string) string {
		userID := strings.Trim(match, "@<>")
		if username := b.users.getUsername(userID); userID != "" {
			return "@" + username
		}
		return match
	}
	return mentionRE.ReplaceAllStringFunc(text, replaceFunc)
}
