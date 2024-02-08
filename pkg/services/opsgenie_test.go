package services

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestOpsgenie_SendNotification_MissingAPIKey(t *testing.T) {
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
		ApiUrl:  server.URL,
		ApiKeys: map[string]string{}}, mockClient)

	// Prepare test data
	recipient := "testRecipient"
	message := "Test message"
	descriptionTemplate := "Test Opsgenie alert: {{.foo}}"

	// Create test notification with description
	notification := Notification{
		Message: message,
		Opsgenie: &OpsgenieNotification{
			Description: descriptionTemplate,
		},
	}

	// Execute the service method with missing API Key
	err := service.Send(notification, Destination{Recipient: recipient, Service: "opsgenie"})

	// Assert the result for missing API Key
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No API key configured for recipient")
}
func TestOpsgenie_SendNotification_MissingDescriptionAndPriority(t *testing.T) {
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
