package services

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTemplater_PagerDuty(t *testing.T) {
	n := Notification{
		Pagerduty: &PagerDutyNotification{
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

	assert.Equal(t, "hello", notification.Pagerduty.Title)
	assert.Equal(t, "world", notification.Pagerduty.Body)
	assert.Equal(t, "high", notification.Pagerduty.Urgency)
	assert.Equal(t, "PE456Y", notification.Pagerduty.PriorityId)
}
