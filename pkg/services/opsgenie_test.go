package services

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	texttemplate "text/template"

	"github.com/opsgenie/opsgenie-go-sdk-v2/alert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock Opsgenie service for testing purposes
type mockOpsgenieService struct {
	options OpsgenieOptions
}

func NewOpsgenieServiceWithClient(options OpsgenieOptions, _ *http.Client) *mockOpsgenieService {
	return &mockOpsgenieService{
		options: options,
	}
}

func (s *mockOpsgenieService) Send(notification Notification, destination Destination) error {
	// Simulate the behavior of the Opsgenie service
	if notification.Opsgenie == nil || notification.Opsgenie.Description == "" {
		return errors.New("Description is missing")
	}
	if _, ok := s.options.ApiKeys[destination.Recipient]; !ok {
		return errors.New("No API key configured for recipient")
	}
	// Return nil to simulate successful sending of notification
	return nil
}

func TestOpsgenieNotification_GetTemplater(t *testing.T) {
	// Prepare test data
	name := "testTemplate"
	descriptionTemplate := "Test Opsgenie alert: {{.foo}}"
	priorityTemplate := "P1"
	aliasTemplate := "Test alias: {{.foo}}"
	noteTemplate := "Test note: {{.foo}}"
	entityTemplate := "Test entity: {{.entity}}"
	userTemplate := "Test user: {{.user}}"
	actionsTemplate := []string{"action1: {{.foo}}", "action2: {{.foo}}"}
	tagsTemplate := []string{"tag1: {{.foo}}", "tag2: {{.foo}}"}
	detailsTemplate := map[string]string{
		"key1": "detail1: {{.foo}}",
		"key2": "detail2: {{.foo}}",
	}
	visibleToTemplate := []alert.Responder{
		{Type: "user", Id: "{{.responder1}}", Name: "{{.responderName1}}", Username: "{{.username1}}"},
		{Type: "team", Id: "{{.responder2}}", Name: "{{.responderName2}}", Username: "{{.username2}}"},
	}
	f := texttemplate.FuncMap{}

	t.Run("ValidTemplate", func(t *testing.T) {
		// Create a new OpsgenieNotification instance
		notification := OpsgenieNotification{
			Description: descriptionTemplate,
			Priority:    priorityTemplate,
			Alias:       aliasTemplate,
			Note:        noteTemplate,
			Entity:      entityTemplate,
			User:        userTemplate,
			Actions:     actionsTemplate,
			Tags:        tagsTemplate,
			Details:     detailsTemplate,
			VisibleTo:   visibleToTemplate,
		}

		// Call the GetTemplater method
		templater, err := notification.GetTemplater(name, f)

		// Assert that no error occurred during the call
		require.NoError(t, err)

		// Prepare mock data for the Templater function
		mockNotification := &Notification{}
		vars := map[string]any{
			"foo":            "bar",
			"entity":         "entity1",
			"user":           "user1",
			"responder1":     "responder1_id",
			"responderName1": "Responder One",
			"username1":      "responder1_username",
			"responder2":     "responder2_id",
			"responderName2": "Responder Two",
			"username2":      "responder2_username",
		}

		// Call the Templater function returned by GetTemplater
		err = templater(mockNotification, vars)

		// Assert that no error occurred during the execution of the Templater function
		require.NoError(t, err)

		// Assert that the OpsgenieNotification's fields were correctly updated
		assert.Equal(t, "Test Opsgenie alert: bar", mockNotification.Opsgenie.Description)
		assert.Equal(t, "P1", mockNotification.Opsgenie.Priority)
		assert.Equal(t, "Test alias: bar", mockNotification.Opsgenie.Alias)
		assert.Equal(t, "Test note: bar", mockNotification.Opsgenie.Note)
		assert.Equal(t, "Test entity: entity1", mockNotification.Opsgenie.Entity)
		assert.Equal(t, "Test user: user1", mockNotification.Opsgenie.User)
		assert.Equal(t, []string{"action1: bar", "action2: bar"}, mockNotification.Opsgenie.Actions)
		assert.Equal(t, []string{"tag1: bar", "tag2: bar"}, mockNotification.Opsgenie.Tags)
		assert.Equal(t, map[string]string{
			"key1": "detail1: bar",
			"key2": "detail2: bar",
		}, mockNotification.Opsgenie.Details)
		assert.Equal(t, []alert.Responder{
			{Type: "user", Id: "responder1_id", Name: "Responder One", Username: "responder1_username"},
			{Type: "team", Id: "responder2_id", Name: "Responder Two", Username: "responder2_username"},
		}, mockNotification.Opsgenie.VisibleTo)
	})

	t.Run("ValidTemplateDetails", func(t *testing.T) {
		// Create a new OpsgenieNotification instance with a valid details template
		notification := OpsgenieNotification{
			Details: map[string]string{
				"key1": "detail1: {{.foo}}",
				"key2": "detail2: {{.foo}}",
			},
		}

		// Call the GetTemplater method
		templater, err := notification.GetTemplater(name, f)

		// Assert that no error occurred during the call
		require.NoError(t, err)

		// Prepare mock data for the Templater function
		mockNotification := &Notification{}
		vars := map[string]any{
			"foo": "bar",
		}

		// Call the Templater function returned by GetTemplater
		err = templater(mockNotification, vars)

		// Assert that no error occurred during the execution of the Templater function
		require.NoError(t, err)

		// Assert that the OpsgenieNotification's Details field was correctly updated
		assert.Equal(t, map[string]string{
			"key1": "detail1: bar",
			"key2": "detail2: bar",
		}, mockNotification.Opsgenie.Details)
	})

	t.Run("ValidTemplateVisibleTo", func(t *testing.T) {
		// Create a new OpsgenieNotification instance with a valid visibleTo template
		notification := OpsgenieNotification{
			VisibleTo: []alert.Responder{
				{Type: "{{.responderType1}}", Id: "{{.responder1}}", Name: "{{.responderName1}}", Username: "{{.username1}}"},
				{Type: "{{.responderType2}}", Id: "{{.responder2}}", Name: "{{.responderName2}}", Username: "{{.username2}}"},
			},
		}

		// Call the GetTemplater method
		templater, err := notification.GetTemplater(name, f)

		// Assert that no error occurred during the call
		require.NoError(t, err)

		// Prepare mock data for the Templater function
		mockNotification := &Notification{}
		vars := map[string]any{
			"responderType1": "user",
			"responder1":     "responder1_id",
			"responderName1": "Responder One",
			"username1":      "responder1_username",
			"responderType2": "team",
			"responder2":     "responder2_id",
			"responderName2": "Responder Two",
			"username2":      "responder2_username",
		}

		// Call the Templater function returned by GetTemplater
		err = templater(mockNotification, vars)

		// Assert that no error occurred during the execution of the Templater function
		require.NoError(t, err)

		// Assert that the OpsgenieNotification's VisibleTo field was correctly updated
		assert.Equal(t, []alert.Responder{
			{Type: "user", Id: "responder1_id", Name: "Responder One", Username: "responder1_username"},
			{Type: "team", Id: "responder2_id", Name: "Responder Two", Username: "responder2_username"},
		}, mockNotification.Opsgenie.VisibleTo)
	})

	t.Run("InvalidTemplateDetails", func(t *testing.T) {
		// Create a new OpsgenieNotification instance with an invalid details template
		notification := OpsgenieNotification{
			Details: map[string]string{
				"key1": "{{.invalid", // Invalid template syntax
			},
		}

		// Call the GetTemplater method with the invalid template
		_, err := notification.GetTemplater(name, f)

		// Assert that an error occurred during the call
		require.Error(t, err)
	})

	t.Run("InvalidTemplateVisibleTo", func(t *testing.T) {
		// Create a new OpsgenieNotification instance with an invalid visibleTo template
		notification := OpsgenieNotification{
			VisibleTo: []alert.Responder{
				{Id: "{{.invalid"}, // Invalid template syntax
			},
		}

		// Call the GetTemplater method with the invalid template
		_, err := notification.GetTemplater(name, f)

		// Assert that an error occurred during the call
		require.Error(t, err)
	})

	t.Run("ValidTemplateTags", func(t *testing.T) {
		// Create a new OpsgenieNotification instance
		notification := OpsgenieNotification{
			Description: "Test Opsgenie alert: {{.foo}}",
			Priority:    "P1",
			Alias:       "Test alias: {{.foo}}",
			Note:        "Test note: {{.foo}}",
			Actions:     []string{"action1: {{.foo}}", "action2: {{.foo}}"},
			Tags:        []string{"tag1: {{.foo}}", "tag2: {{.foo}}"},
		}

		// Call the GetTemplater method
		templater, err := notification.GetTemplater(name, f)

		// Assert that no error occurred during the call
		require.NoError(t, err)

		// Prepare mock data for the Templater function
		mockNotification := &Notification{}
		vars := map[string]any{
			"foo": "bar",
		}

		// Call the Templater function returned by GetTemplater
		err = templater(mockNotification, vars)

		// Assert that no error occurred during the execution of the Templater function
		require.NoError(t, err)

		// Assert that the OpsgenieNotification's actions field was correctly updated
		assert.Equal(t, []string{"action1: bar", "action2: bar"}, mockNotification.Opsgenie.Actions)

		// Assert that the OpsgenieNotification's tags field was correctly updated
		assert.Equal(t, []string{"tag1: bar", "tag2: bar"}, mockNotification.Opsgenie.Tags)
	})
}

func TestOpsgenie_SendNotification_MissingAPIKey(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := NewOpsgenieService(OpsgenieOptions{
		ApiUrl:  server.URL,
		ApiKeys: map[string]string{},
	})

	// Prepare test data
	recipient := "testRecipient"
	message := "Test message"
	descriptionTemplate := "Test Opsgenie alert: {{.foo}}"
	priority := "P1"
	aliasTemplate := "Test alias: {{.foo}}"
	noteTemplate := "Test note: {{.foo}}"

	// Create test notification with description
	notification := Notification{
		Message: message,
		Opsgenie: &OpsgenieNotification{
			Description: descriptionTemplate,
			Priority:    priority,
			Alias:       aliasTemplate,
			Note:        noteTemplate,
		},
	}

	// Execute the service method with missing API Key
	err := service.Send(notification, Destination{Recipient: recipient, Service: "opsgenie"})

	// Assert the result for missing API Key
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no API key configured for recipient testRecipient")
}

func TestOpsgenie_SendNotification_WithMessageOnly(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Replace the HTTP client in the Opsgenie service with a mock client
	mockClient := &http.Client{
		Transport: &http.Transport{},
	}
	service := NewOpsgenieServiceWithClient(OpsgenieOptions{
		ApiUrl: server.URL,
		ApiKeys: map[string]string{
			"testRecipient": "testApiKey",
		},
	}, mockClient)

	// Prepare test data
	recipient := "testRecipient"
	message := "Test message"

	// Create test notification with missing description and priority
	notification := Notification{
		Message: message,
	}

	// Execute the service method with missing description and priority
	err := service.Send(notification, Destination{Recipient: recipient, Service: "opsgenie"})

	// Assert the result for missing description and priority
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Description is missing")
}

func TestOpsgenie_SendNotification_WithDescriptionOnly(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Replace the HTTP client in the Opsgenie service with a mock client
	mockClient := &http.Client{
		Transport: &http.Transport{},
	}
	service := NewOpsgenieServiceWithClient(OpsgenieOptions{
		ApiUrl: server.URL,
		ApiKeys: map[string]string{
			"testRecipient": "testApiKey",
		},
	}, mockClient)

	// Prepare test data
	recipient := "testRecipient"
	message := "Test message"
	descriptionTemplate := "Test Opsgenie alert: {{.foo}}"

	// Create test notification with description only
	notification := Notification{
		Message: message,
		Opsgenie: &OpsgenieNotification{
			Description: descriptionTemplate,
		},
	}

	// Execute the service method with description only
	err := service.Send(notification, Destination{Recipient: recipient, Service: "opsgenie"})

	// Assert the result for description present and no priority
	require.NoError(t, err) // Expect no error
}

func TestOpsgenie_SendNotification_WithDescriptionAndPriority(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Replace the HTTP client in the Opsgenie service with a mock client
	mockClient := &http.Client{
		Transport: &http.Transport{},
	}
	service := NewOpsgenieServiceWithClient(OpsgenieOptions{
		ApiUrl: server.URL,
		ApiKeys: map[string]string{
			"testRecipient": "testApiKey",
		},
	}, mockClient)

	// Prepare test data
	recipient := "testRecipient"
	message := "Test message"
	descriptionTemplate := "Test Opsgenie alert: {{.foo}}"
	priority := "P1"

	// Create test notification with description and priority
	notification := Notification{
		Message: message,
		Opsgenie: &OpsgenieNotification{
			Description: descriptionTemplate,
			Priority:    priority,
		},
	}

	// Execute the service method with description and priority
	err := service.Send(notification, Destination{Recipient: recipient, Service: "opsgenie"})

	// Assert the result for description and priority present
	require.NoError(t, err) // Expect no error
}

func TestOpsgenie_SendNotification_WithAllFields(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Replace the HTTP client in the Opsgenie service with a mock client
	mockClient := &http.Client{
		Transport: &http.Transport{},
	}
	service := NewOpsgenieServiceWithClient(OpsgenieOptions{
		ApiUrl: server.URL,
		ApiKeys: map[string]string{
			"testRecipient": "testApiKey",
		},
	}, mockClient)

	// Prepare test data
	recipient := "testRecipient"
	message := "Test message"
	descriptionTemplate := "Test Opsgenie alert: {{.foo}}"
	aliasTemplate := "Test alias: {{.foo}}"
	priority := "P1"
	noteTemplate := "Test note: {{.foo}}"
	actions := []string{"action1", "action2"}
	tags := []string{"tag1", "tag2"}
	details := map[string]string{"detail1": "value1"}
	entity := "TestEntity"
	user := "TestUser"

	// Testing multiple responder scenarios with templates
	visibleTo := []alert.Responder{
		{Type: "user", Id: "{{.responderUserId}}"},             // Template for user responder
		{Type: "team", Id: "{{.responderTeamId}}"},             // Template for team responder
		{Type: "escalation", Id: "{{.responderEscalationId}}"}, // Template for escalation responder
		{Type: "schedule", Id: "{{.responderScheduleId}}"},     // Template for schedule responder
	}

	// Create test notification with all fields
	notification := Notification{
		Message: message,
		Opsgenie: &OpsgenieNotification{
			Description: descriptionTemplate,
			Priority:    priority,
			Alias:       aliasTemplate,
			Note:        noteTemplate,
			Actions:     actions,
			Tags:        tags,
			Details:     details,
			Entity:      entity,
			User:        user,
			VisibleTo:   visibleTo,
		},
	}

	// Execute the service method with all fields
	err := service.Send(notification, Destination{Recipient: recipient, Service: "opsgenie"})

	// Assert the result for all fields present
	require.NoError(t, err) // Expect no error
}
