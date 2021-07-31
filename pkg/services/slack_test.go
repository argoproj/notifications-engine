package services

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

func TestValidIconEmoij(t *testing.T) {
	assert.Equal(t, true, validIconEmoji.MatchString(":slack:"))
	assert.Equal(t, true, validIconEmoji.MatchString(":chart_with_upwards_trend:"))
	assert.Equal(t, false, validIconEmoji.MatchString("http://lorempixel.com/48/48"))
}

func TestValidIconURL(t *testing.T) {
	assert.Equal(t, true, isValidIconURL("http://lorempixel.com/48/48"))
	assert.Equal(t, true, isValidIconURL("https://lorempixel.com/48/48"))
	assert.Equal(t, false, isValidIconURL("favicon.ico"))
	assert.Equal(t, false, isValidIconURL("ftp://favicon.ico"))
	assert.Equal(t, false, isValidIconURL("ftp://lorempixel.com/favicon.ico"))
}

func TestGetTemplater_Slack(t *testing.T) {
	n := Notification{
		Slack: &SlackNotification{
			Attachments:     "{{.foo}}",
			Blocks:          "{{.bar}}",
			GroupingKey:     "{{.foo}}-{{.bar}}",
			NotifyBroadcast: true,
		},
	}
	templater, err := n.GetTemplater("", template.FuncMap{})

	if !assert.NoError(t, err) {
		return
	}

	var notification Notification
	err = templater(&notification, map[string]interface{}{
		"foo": "hello",
		"bar": "world",
	})

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "hello", notification.Slack.Attachments)
	assert.Equal(t, "world", notification.Slack.Blocks)
	assert.Equal(t, "hello-world", notification.Slack.GroupingKey)
	assert.Equal(t, true, notification.Slack.NotifyBroadcast)
}
