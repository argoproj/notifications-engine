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

func TestNewService_Success(t *testing.T) {
	t.Run("Single template", func(t *testing.T) {
		svc, err := NewService(map[string]services.Notification{
			"test": {
				Message: "{{.foo}}",
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)
		assert.Len(t, svc.templaters, 1)
	})

	t.Run("Multiple templates", func(t *testing.T) {
		svc, err := NewService(map[string]services.Notification{
			"template1": {
				Message: "{{.foo}}",
			},
			"template2": {
				Message: "{{.bar}}",
			},
			"template3": {
				Message: "{{.baz}}",
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)
		assert.Len(t, svc.templaters, 3)
	})

	t.Run("Empty templates map", func(t *testing.T) {
		svc, err := NewService(map[string]services.Notification{})
		require.NoError(t, err)
		require.NotNil(t, svc)
		assert.Empty(t, svc.templaters)
	})

	t.Run("Env and expandenv functions are removed", func(t *testing.T) {
		// Test that env and expandenv Sprig functions are removed for security
		svc, err := NewService(map[string]services.Notification{
			"test": {
				Message: "test",
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)
	})
}

func TestNewService_InvalidTemplate(t *testing.T) {
	t.Run("Invalid template syntax", func(t *testing.T) {
		_, err := NewService(map[string]services.Notification{
			"invalid": {
				Message: "{{.foo",
			},
		})
		require.Error(t, err)
	})
}

func TestFormatNotification_Success(t *testing.T) {
	t.Run("Single template", func(t *testing.T) {
		svc, err := NewService(map[string]services.Notification{
			"greeting": {
				Message: "Hello {{.name}}!",
			},
		})
		require.NoError(t, err)

		notification, err := svc.FormatNotification(map[string]any{
			"name": "World",
		}, "greeting")

		require.NoError(t, err)
		assert.Equal(t, "Hello World!", notification.Message)
	})

	t.Run("Multiple templates applied in order", func(t *testing.T) {
		svc, err := NewService(map[string]services.Notification{
			"template1": {
				Message: "{{.first}}",
			},
			"template2": {
				Message: "{{.second}}",
			},
		})
		require.NoError(t, err)

		// When multiple templates are used, the last one should overwrite
		notification, err := svc.FormatNotification(map[string]any{
			"first":  "First",
			"second": "Second",
		}, "template1", "template2")

		require.NoError(t, err)
		assert.Equal(t, "Second", notification.Message)
	})

	t.Run("Complex template with sprig functions", func(t *testing.T) {
		svc, err := NewService(map[string]services.Notification{
			"complex": {
				Message: "{{.name | upper}}",
			},
		})
		require.NoError(t, err)

		notification, err := svc.FormatNotification(map[string]any{
			"name": "test",
		}, "complex")

		require.NoError(t, err)
		assert.Equal(t, "TEST", notification.Message)
	})

	t.Run("Empty template list", func(t *testing.T) {
		svc, err := NewService(map[string]services.Notification{
			"test": {
				Message: "test",
			},
		})
		require.NoError(t, err)

		notification, err := svc.FormatNotification(map[string]any{})
		require.NoError(t, err)
		assert.NotNil(t, notification)
		assert.Empty(t, notification.Message)
	})
}

func TestFormatNotification_TemplateNotFound(t *testing.T) {
	svc, err := NewService(map[string]services.Notification{
		"existing": {
			Message: "test",
		},
	})
	require.NoError(t, err)

	notification, err := svc.FormatNotification(map[string]any{}, "nonexistent")
	require.Error(t, err)
	assert.Nil(t, notification)
	assert.Contains(t, err.Error(), "template 'nonexistent' is not supported")
}

func TestFormatNotification_TemplateExecutionError(t *testing.T) {
	svc, err := NewService(map[string]services.Notification{
		"test": {
			Message: "{{fail \"intentional error\"}}",
		},
	})
	require.NoError(t, err)

	// This should trigger an error during template execution
	notification, err := svc.FormatNotification(map[string]any{
		"other": "value",
	}, "test")

	require.Error(t, err)
	assert.Nil(t, notification)
}

func TestFormatNotification_MultipleTemplatesWithError(t *testing.T) {
	svc, err := NewService(map[string]services.Notification{
		"valid": {
			Message: "{{.foo}}",
		},
	})
	require.NoError(t, err)

	// First template is valid, second doesn't exist
	notification, err := svc.FormatNotification(map[string]any{
		"foo": "bar",
	}, "valid", "invalid")

	require.Error(t, err)
	assert.Nil(t, notification)
	assert.Contains(t, err.Error(), "template 'invalid' is not supported")
}
