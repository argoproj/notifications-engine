package services

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	texttemplate "text/template"

	"github.com/stretchr/testify/assert"
)

// Mock Opsgenie service for testing purposes
type mockOpsgenieService struct {
	options OpsgenieOptions
	client  *http.Client
}

func NewOpsgenieServiceWithClient(options OpsgenieOptions, client *http.Client) *mockOpsgenieService {
	return &mockOpsgenieService{
		options: options,
		client:  client,
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
	priority := "P1"
	aliasTemplate := "Test alias: {{.foo}}"
	noteTemplate := "Test note: {{.foo}}"
	f := texttemplate.FuncMap{}

	t.Run("ValidTemplate", func(t *testing.T) {
		// Create a new OpsgenieNotification instance
		notification := OpsgenieNotification{
			Description: descriptionTemplate,
			Priority:    priority,
			Alias:       aliasTemplate,
			Note:        noteTemplate,
		}

		// Call the GetTemplater method
		templater, err := notification.GetTemplater(name, f)

		// Assert that no error occurred during the call
		assert.NoError(t, err)

		// Prepare mock data for the Templater function
		mockNotification := &Notification{}
		vars := map[string]interface{}{
			"foo": "bar",
		}

		// Call the Templater function returned by GetTemplater
		err = templater(mockNotification, vars)

		// Assert that no error occurred during the execution of the Templater function
		assert.NoError(t, err)

		// Assert that the OpsgenieNotification's description field was correctly updated
		assert.Equal(t, "Test Opsgenie alert: bar", mockNotification.Opsgenie.Description)

		// Assert that the OpsgenieNotification's priority field was correctly updated
		assert.Equal(t, "P1", mockNotification.Opsgenie.Priority)

		// Assert that the OpsgenieNotification's alias field was correctly updated
		assert.Equal(t, "Test alias: bar", mockNotification.Opsgenie.Alias)

		// Assert that the OpsgenieNotification's note field was correctly updated
		assert.Equal(t, "Test note: bar", mockNotification.Opsgenie.Note)
	})

	t.Run("InvalidTemplateDescription", func(t *testing.T) {
		// Create a new OpsgenieNotification instance with an invalid description template
		notification := OpsgenieNotification{
			Description: "{{.invalid", // Invalid template syntax
		}

		// Call the GetTemplater method with the invalid template
		_, err := notification.GetTemplater(name, f)

		// Assert that an error occurred during the call
		assert.Error(t, err)
	})

	t.Run("InvalidPriorityP0", func(t *testing.T) {
		// Create a new OpsgenieNotification instance with an invalid priority value
		notification := OpsgenieNotification{
			Priority: "P0", // Invalid priority value
		}

		// Call the GetTemplater method with the invalid template
		_, err := notification.GetTemplater(name, f)

		// Assert that an error occurred during the call
		assert.Error(t, err)
	})

	t.Run("InvalidPriorityP8", func(t *testing.T) {
		// Create a new OpsgenieNotification instance with an invalid priority value
		notification := OpsgenieNotification{
			Priority: "P8", // Invalid priority value
		}

		// Call the GetTemplater method with the invalid template
		_, err := notification.GetTemplater(name, f)

		// Assert that an error occurred during the call
		assert.Error(t, err)
	})

	t.Run("InvalidTemplateAlias", func(t *testing.T) {
		// Create a new OpsgenieNotification instance with an invalid alias template
		notification := OpsgenieNotification{
			Alias: "{{.invalid", // Invalid template syntax
		}

		// Call the GetTemplater method with the invalid template
		_, err := notification.GetTemplater(name, f)

		// Assert that an error occurred during the call
		assert.Error(t, err)
	})

	t.Run("InvalidTemplateNote", func(t *testing.T) {
		// Create a new OpsgenieNotification instance with an invalid note template
		notification := OpsgenieNotification{
			Note: "{{.invalid", // Invalid template syntax
		}

		// Call the GetTemplater method with the invalid template
		_, err := notification.GetTemplater(name, f)

		// Assert that an error occurred during the call
		assert.Error(t, err)
	})
}

func TestOpsgenie_SendNotification_MissingAPIKey(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := NewOpsgenieService(OpsgenieOptions{
		ApiUrl:  server.URL,
		ApiKeys: map[string]string{}})

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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no API key configured for recipient testRecipient")
}
func TestOpsgenie_SendNotification_WithMessageOnly(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		}}, mockClient)

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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Description is missing")
}

func TestOpsgenie_SendNotification_WithDescriptionOnly(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	assert.NoError(t, err) // Expect no error
}

func TestOpsgenie_SendNotification_WithDescriptionAndPriority(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		}}, mockClient)

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
	assert.NoError(t, err) // Expect no error
}

func TestOpsgenie_SendNotification_WithAllFields(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		}}, mockClient)

	// Prepare test data
	recipient := "testRecipient"
	message := "Test message"
	descriptionTemplate := "Test Opsgenie alert: {{.foo}}"
	aliasTemplate := "Test alias: {{.foo}}"
	priority := "P1"
	noteTemplate := "Test note: {{.foo}}"

	// Create test notification with description and priority
	notification := Notification{
		Message: message,
		Opsgenie: &OpsgenieNotification{
			Description: descriptionTemplate,
			Priority:    priority,
			Alias:       aliasTemplate,
			Note:        noteTemplate,
		},
	}

	// Execute the service method with description and priority
	err := service.Send(notification, Destination{Recipient: recipient, Service: "opsgenie"})

	// Assert the result for description and priority present
	assert.NoError(t, err) // Expect no error
}
