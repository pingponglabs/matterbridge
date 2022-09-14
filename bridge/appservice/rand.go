package bappservice

import (
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())

}

var letterLowerRunes = []rune("abcdefghijklmnopqrstuvwxyz")

func randStringLowerRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterLowerRunes[rand.Intn(len(letterLowerRunes))]
	}
	return string(b)
}
