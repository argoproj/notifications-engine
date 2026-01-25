package services

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidEmoji(t *testing.T) {
	assert.True(t, validEmoji.MatchString(":slack:"))
	assert.True(t, validEmoji.MatchString(":chart_with_upwards_trend:"))
	assert.False(t, validEmoji.MatchString("http://lorempixel.com/48/48"))
}

func TestValidAvatarURL(t *testing.T) {
	assert.True(t, isValidAvatarURL("http://lorempixel.com/48/48"))
	assert.True(t, isValidAvatarURL("https://lorempixel.com/48/48"))
	assert.False(t, isValidAvatarURL("favicon.ico"))
	assert.False(t, isValidAvatarURL("ftp://favicon.ico"))
	assert.False(t, isValidAvatarURL("ftp://lorempixel.com/favicon.ico"))
}

func TestGetTemplater_RocketChat(t *testing.T) {
	n := Notification{
		RocketChat: &RocketChatNotification{
			Attachments: "{{.foo}}",
		},
	}
	templater, err := n.GetTemplater("", template.FuncMap{})

	require.NoError(t, err)

	var notification Notification
	err = templater(&notification, map[string]any{
		"foo": "hello",
	})

	require.NoError(t, err)

	assert.Equal(t, "hello", notification.RocketChat.Attachments)
}
