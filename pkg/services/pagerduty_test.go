package services

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTemplater_PagerDuty(t *testing.T) {
	n := Notification{
		PagerDuty: &PagerDutyNotification{
			Title: "{{.title}}", Body: "{{.body}}", Urgency: "{{.urg}}", PriorityId: "{{.prid}}",
		},
	}

	templater, err := n.GetTemplater("", template.FuncMap{})
	require.NoError(t, err)

	var notification Notification

	err = templater(&notification, map[string]any{
		"title": "hello",
		"body":  "world",
		"urg":   "high",
		"prid":  "PE456Y",
	})

	require.NoError(t, err)

	assert.Equal(t, "hello", notification.PagerDuty.Title)
	assert.Equal(t, "world", notification.PagerDuty.Body)
	assert.Equal(t, "high", notification.PagerDuty.Urgency)
	assert.Equal(t, "PE456Y", notification.PagerDuty.PriorityId)
}
