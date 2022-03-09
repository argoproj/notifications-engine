package services

import (
	_ "github.com/golang/mock/mockgen/model"
	"github.com/stretchr/testify/assert"
	"testing"
	"text/template"

)

func TestGetTemplater_PagerDuty(t *testing.T) {
	n := Notification{
		Pagerduty: &PagerDutyNotification{
			Title: "{{.title}}", Body: "{{.body}}", Urgency: "{{.urg}}", PriorityID: "{{.prid}}",

		},
	}

	templater, err := n.GetTemplater("", template.FuncMap{})
	if !assert.NoError(t, err) {
		return
	}

	var notification Notification

	err = templater(&notification, map[string]interface{}{
		"title": "hello",
		"body": "world",
		"urg": "high",
		"prid": "PE456Y",
	})

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "hello", notification.Pagerduty.Title)
	assert.Equal(t, "world", notification.Pagerduty.Body)
	assert.Equal(t, "high", notification.Pagerduty.Urgency)
	assert.Equal(t, "PE456Y", notification.Pagerduty.PriorityID)
}
