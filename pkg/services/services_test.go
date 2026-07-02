package services

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDestinations_Merge(t *testing.T) {
	a := Destinations{"trigger1": {{Service: "slack", Recipient: "a"}}}
	b := Destinations{
		"trigger1": {{Service: "slack", Recipient: "b"}},
		"trigger2": {{Service: "email", Recipient: "c"}},
	}

	a.Merge(b)

	assert.Equal(t, Destinations{
		"trigger1": {{Service: "slack", Recipient: "a"}, {Service: "slack", Recipient: "b"}},
		"trigger2": {{Service: "email", Recipient: "c"}},
	}, a)
}

func TestDestinations_Dedup(t *testing.T) {
	d := Destinations{
		"trigger1": {
			{Service: "slack", Recipient: "a"},
			{Service: "slack", Recipient: "a"},
			{Service: "slack", Recipient: "b"},
			{Service: "email", Recipient: "a"},
		},
	}

	deduped := d.Dedup()

	assert.Equal(t, Destinations{
		"trigger1": {
			{Service: "slack", Recipient: "a"},
			{Service: "slack", Recipient: "b"},
			{Service: "email", Recipient: "a"},
		},
	}, deduped)
}

func TestGetTemplater(t *testing.T) {
	n := Notification{Message: "{{.foo}}"}

	templater, err := n.GetTemplater("", template.FuncMap{})
	require.NoError(t, err)

	var notification Notification

	err = templater(&notification, map[string]any{
		"foo": "hello",
	})

	require.NoError(t, err)

	assert.Equal(t, "hello", notification.Message)
}
