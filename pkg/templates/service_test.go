package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/notifications-engine/pkg/services"
)

func TestFormat_Message(t *testing.T) {
	svc, err := NewService(map[string]services.Notification{
		"test": {
			Message: "{{.foo}}",
		},
	})

	require.NoError(t, err)

	notification, err := svc.FormatNotification(map[string]any{
		"foo": "hello",
	}, "test")

	require.NoError(t, err)

	assert.Equal(t, "hello", notification.Message)
}
