package services

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

func TestGetTemplater_AwsSqs(t *testing.T) {
	n := Notification{
		Message: "{{.message}}",
		AwsSqs:  &AwsSqsNotification{
			// Method: "{{.method}}",
		},
	}

	templater, err := n.GetTemplater("", template.FuncMap{})
	if !assert.NoError(t, err) {
		return
	}

	var notification Notification

	err = templater(&notification, map[string]interface{}{
		"method":  "GET",
		"message": "{\"JSON\"}",
	})

	if !assert.NoError(t, err) {
		return
	}

	// assert.Equal(t, "GET", notification.AwsSqs.Method)
	assert.Equal(t, "{\"JSON\"}", notification.Message)

}
