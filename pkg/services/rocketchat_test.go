package services

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"text/template"
)

func TestValidEmoji(t *testing.T) {
	assert.Equal(t, true, validEmoji.MatchString(":slack:"))
	assert.Equal(t, true, validEmoji.MatchString(":chart_with_upwards_trend:"))
	assert.Equal(t, false, validEmoji.MatchString("http://lorempixel.com/48/48"))
}

func TestValidAvatarURL(t *testing.T) {
	assert.Equal(t, true, isValidAvatarURL("http://lorempixel.com/48/48"))
	assert.Equal(t, true, isValidAvatarURL("https://lorempixel.com/48/48"))
	assert.Equal(t, false, isValidAvatarURL("favicon.ico"))
	assert.Equal(t, false, isValidAvatarURL("ftp://favicon.ico"))
	assert.Equal(t, false, isValidAvatarURL("ftp://lorempixel.com/favicon.ico"))
}

func TestGetTemplater_RocketChat(t *testing.T) {
	n := Notification{
		RocketChat: &RocketChatNotification{
			Attachments: "{{.foo}}",
		},
	}
	templater, err := n.GetTemplater("", template.FuncMap{})

	if !assert.NoError(t, err) {
		return
	}

	var notification Notification
	err = templater(&notification, map[string]interface{}{
		"foo": "hello",
	})

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "hello", notification.RocketChat.Attachments)
}
