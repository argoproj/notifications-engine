package services

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

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

func TestNewService_RetiredTeams(t *testing.T) {
	_, err := NewService("teams", []byte("recipientUrls: {}"))
	require.ErrorContains(t, err, "teams")
}

func TestGetTemplater_IgnoresRetiredTeams(t *testing.T) {
	var n Notification
	require.NoError(t, yaml.Unmarshal([]byte("message: hello\nteams:\n  title: '{{obsoleteFunction}}'\nteams-workflows:\n  title: '{{.title}}'\n"), &n))
	templater, err := n.GetTemplater("mixed", nil)
	require.NoError(t, err)
	var result Notification
	require.NoError(t, templater(&result, map[string]any{"title": "Supported"}))
	require.NotNil(t, result.TeamsWorkflows)
	assert.Equal(t, "Supported", result.TeamsWorkflows.Title)
}
