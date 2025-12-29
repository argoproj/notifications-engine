package services

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

func TestGetTemplater_TeamsWorkflows(t *testing.T) {
	notificationTemplate := Notification{
		TeamsWorkflows: &TeamsWorkflowsNotification{
			Template:        "template {{.value}}",
			Title:           "title {{.value}}",
			Summary:         "summary {{.value}}",
			Text:            "text {{.value}}",
			Facts:           "facts {{.value}}",
			Sections:        "sections {{.value}}",
			PotentialAction: "actions {{.value}}",
			ThemeColor:      "theme color {{.value}}",
			AdaptiveCard:    "adaptiveCard {{.value}}",
		},
	}

	templater, err := notificationTemplate.GetTemplater("test", template.FuncMap{})

	if err != nil {
		t.Error(err)
		return
	}

	notification := Notification{}

	err = templater(&notification, map[string]interface{}{
		"value": "value",
	})

	if err != nil {
		t.Error(err)
		return
	}

	assert.Equal(t, "template value", notification.TeamsWorkflows.Template)
	assert.Equal(t, "title value", notification.TeamsWorkflows.Title)
	assert.Equal(t, "summary value", notification.TeamsWorkflows.Summary)
	assert.Equal(t, "text value", notification.TeamsWorkflows.Text)
	assert.Equal(t, "sections value", notification.TeamsWorkflows.Sections)
	assert.Equal(t, "facts value", notification.TeamsWorkflows.Facts)
	assert.Equal(t, "actions value", notification.TeamsWorkflows.PotentialAction)
	assert.Equal(t, "theme color value", notification.TeamsWorkflows.ThemeColor)
	assert.Equal(t, "adaptiveCard value", notification.TeamsWorkflows.AdaptiveCard)
}

func TestTeamsWorkflows_DefaultMessage(t *testing.T) {
	var receivedBody adaptiveMessage
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, err := io.ReadAll(request.Body)
		assert.NoError(t, err)

		err = json.Unmarshal(data, &receivedBody)
		assert.NoError(t, err)

		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Use a valid Workflows URL pattern
	webhookURL := server.URL + "/powerautomate/webhook"

	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": webhookURL,
		},
	})

	notification := Notification{
		Message: "simple message",
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.NoError(t, err)
	assert.Equal(t, "message", receivedBody.Type)
	assert.Len(t, receivedBody.Attachments, 1)
	assert.Equal(t, "application/vnd.microsoft.card.adaptive", receivedBody.Attachments[0].ContentType)
	assert.NotNil(t, receivedBody.Attachments[0].Content)
	assert.Equal(t, "AdaptiveCard", receivedBody.Attachments[0].Content.Type)
	assert.Equal(t, "1.4", receivedBody.Attachments[0].Content.Version)
}

func TestTeamsWorkflows_AdaptiveCardTemplate(t *testing.T) {
	var receivedBody adaptiveMessage
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, err := io.ReadAll(request.Body)
		assert.NoError(t, err)

		err = json.Unmarshal(data, &receivedBody)
		assert.NoError(t, err)

		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookURL := server.URL + "/powerautomate/webhook"

	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": webhookURL,
		},
	})

	notification := Notification{
		TeamsWorkflows: &TeamsWorkflowsNotification{
			AdaptiveCard: `{
				"type": "AdaptiveCard",
				"version": "1.4",
				"body": [
					{
						"type": "TextBlock",
						"text": "Custom Adaptive Card"
					}
				]
			}`,
		},
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.NoError(t, err)
	assert.Equal(t, "message", receivedBody.Type)
	assert.Len(t, receivedBody.Attachments, 1)
	assert.Equal(t, "application/vnd.microsoft.card.adaptive", receivedBody.Attachments[0].ContentType)
	assert.NotNil(t, receivedBody.Attachments[0].Content)
	assert.Equal(t, "AdaptiveCard", receivedBody.Attachments[0].Content.Type)
	assert.Equal(t, "1.4", receivedBody.Attachments[0].Content.Version)
	assert.Len(t, receivedBody.Attachments[0].Content.Body, 1)
	assert.Equal(t, "TextBlock", receivedBody.Attachments[0].Content.Body[0].Type)
	assert.Equal(t, "Custom Adaptive Card", receivedBody.Attachments[0].Content.Body[0].Text)
}

func TestTeamsWorkflows_MessageFields(t *testing.T) {
	var receivedBody adaptiveMessage
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, err := io.ReadAll(request.Body)
		assert.NoError(t, err)

		err = json.Unmarshal(data, &receivedBody)
		assert.NoError(t, err)

		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookURL := server.URL + "/powerautomate/webhook"

	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": webhookURL,
		},
	})

	notification := Notification{
		Message: "welcome message",
		TeamsWorkflows: &TeamsWorkflowsNotification{
			Title:   "Title",
			Summary: "Summary",
			Text:    "Text",
			Facts: `[{
				"name": "Status",
				"value": "Success"
			}]`,
			Sections: `[{
				"facts": [{
					"name": "Namespace",
					"value": "default"
				}]
			}]`,
			PotentialAction: `[{
				"@type": "OpenUri",
				"name": "View Details",
				"targets": [{
					"os": "default",
					"uri": "https://example.com"
				}]
			}]`,
			ThemeColor: "Good",
		},
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.NoError(t, err)
	assert.Equal(t, "message", receivedBody.Type)
	assert.Len(t, receivedBody.Attachments, 1)
	card := receivedBody.Attachments[0].Content
	assert.NotNil(t, card)

	// Check title
	foundTitle := false
	for _, element := range card.Body {
		if element.Type != "TextBlock" || element.Text != "Title" {
			continue
		}
		foundTitle = true
		assert.Equal(t, "Large", element.Size)
		assert.Equal(t, "Bolder", element.Weight)
		assert.Equal(t, "Good", element.Color)
		break
	}
	assert.True(t, foundTitle, "Title should be present")

	// Check text
	foundText := false
	for _, element := range card.Body {
		if element.Type == "TextBlock" && element.Text == "Text" {
			foundText = true
			break
		}
	}
	assert.True(t, foundText, "Text should be present")

	// Check facts
	foundFactSet := false
	for _, element := range card.Body {
		if element.Type != "FactSet" {
			continue
		}
		foundFactSet = true
		assert.Len(t, element.Facts, 1)
		assert.Equal(t, "Status", element.Facts[0].Title)
		assert.Equal(t, "Success", element.Facts[0].Value)
		break
	}
	assert.True(t, foundFactSet, "FactSet should be present")

	// Check actions
	if len(card.Actions) > 0 {
		assert.Equal(t, "Action.OpenUrl", card.Actions[0].Type)
		assert.Equal(t, "View Details", card.Actions[0].Title)
		assert.Equal(t, "https://example.com", card.Actions[0].URL)
	}
}

func TestTeamsWorkflows_ValidateWebhookURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid Power Automate URL",
			url:         "https://api.powerautomate.com/webhook/abc123",
			expectError: false,
		},
		{
			name:        "Valid Power Platform URL",
			url:         "https://api.powerplatform.com/webhook/abc123",
			expectError: false,
		},
		{
			name:        "Valid Flow Microsoft URL",
			url:         "https://flow.microsoft.com/webhook/abc123",
			expectError: false,
		},
		{
			name:        "Valid URL with powerautomate path",
			url:         "https://example.com/powerautomate/webhook",
			expectError: false,
		},
		{
			name:        "Valid localhost URL (HTTP allowed)",
			url:         "http://localhost:8080/powerautomate/webhook",
			expectError: false,
		},
		{
			name:        "Empty URL",
			url:         "",
			expectError: true,
			errorMsg:    "webhook URL cannot be empty",
		},
		{
			name:        "Invalid URL format - malformed",
			url:         "://invalid",
			expectError: true,
			errorMsg:    "invalid webhook URL format",
		},
		{
			name:        "Relative URL",
			url:         "/webhook",
			expectError: true,
			errorMsg:    "webhook URL must use HTTPS scheme",
		},
		{
			name:        "HTTP URL (not localhost)",
			url:         "http://api.powerautomate.com/webhook",
			expectError: true,
			errorMsg:    "webhook URL must use HTTPS scheme",
		},
		{
			name:        "Invalid scheme",
			url:         "ftp://api.powerautomate.com/webhook",
			expectError: true,
			errorMsg:    "webhook URL must use HTTPS scheme",
		},
		{
			name:        "No host",
			url:         "https:///webhook",
			expectError: true,
			errorMsg:    "webhook URL must have a valid host",
		},
		{
			name:        "Invalid URL pattern",
			url:         "https://example.com/webhook",
			expectError: true,
			errorMsg:    "webhook URL does not appear to be a valid Microsoft Teams Workflows URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWorkflowsWebhookURL(tt.url)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTeamsWorkflows_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookURL := server.URL + "/powerautomate/webhook"

	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": webhookURL,
		},
	})

	notification := Notification{
		Message: "test message",
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.NoError(t, err)
}

func TestTeamsWorkflows_StatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, err := writer.Write([]byte("error details"))
		assert.NoError(t, err)
	}))
	defer server.Close()

	webhookURL := server.URL + "/powerautomate/webhook"

	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": webhookURL,
		},
	})

	notification := Notification{
		Message: "test message",
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "teams workflows webhook post error")
	assert.Contains(t, err.Error(), "status 400")
	assert.Contains(t, err.Error(), "error details")
}

func TestTeamsWorkflows_StatusErrorNoBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	webhookURL := server.URL + "/powerautomate/webhook"

	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": webhookURL,
		},
	})

	notification := Notification{
		Message: "test message",
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "teams workflows webhook post error")
	assert.Contains(t, err.Error(), "status 500")
	assert.Contains(t, err.Error(), "no error details provided")
}

func TestTeamsWorkflows_MissingRecipient(t *testing.T) {
	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{},
	})

	notification := Notification{
		Message: "test message",
	}

	err := service.Send(notification,
		Destination{
			Recipient: "nonexistent",
			Service:   "teams-workflows",
		},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no teams-workflows webhook configured for recipient")
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestTeamsWorkflows_InvalidURL(t *testing.T) {
	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": "https://example.com/webhook", // Invalid URL pattern
		},
	})

	notification := Notification{
		Message: "test message",
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid webhook URL for recipient")
	assert.Contains(t, err.Error(), "webhook URL does not appear to be a valid Microsoft Teams Workflows URL")
}

func TestTeamsWorkflows_InvalidFactsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookURL := server.URL + "/powerautomate/webhook"

	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": webhookURL,
		},
	})

	notification := Notification{
		TeamsWorkflows: &TeamsWorkflowsNotification{
			Facts: "invalid json {",
		},
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "teams-workflows facts unmarshalling error")
}

func TestTeamsWorkflows_InvalidSectionsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookURL := server.URL + "/powerautomate/webhook"

	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": webhookURL,
		},
	})

	notification := Notification{
		TeamsWorkflows: &TeamsWorkflowsNotification{
			Sections: "invalid json {",
		},
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "teams-workflows sections unmarshalling error")
}

func TestTeamsWorkflows_InvalidPotentialActionJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookURL := server.URL + "/powerautomate/webhook"

	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": webhookURL,
		},
	})

	notification := Notification{
		TeamsWorkflows: &TeamsWorkflowsNotification{
			PotentialAction: "invalid json {",
		},
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "teams-workflows potential action unmarshalling error")
}

func TestTeamsWorkflows_InvalidAdaptiveCardJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookURL := server.URL + "/powerautomate/webhook"

	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": webhookURL,
		},
	})

	notification := Notification{
		TeamsWorkflows: &TeamsWorkflowsNotification{
			AdaptiveCard: "invalid json {",
		},
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "teams-workflows adaptiveCard unmarshalling error")
}

func TestTeamsWorkflows_FactsWithNonStringValue(t *testing.T) {
	var receivedBody adaptiveMessage
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, err := io.ReadAll(request.Body)
		assert.NoError(t, err)

		err = json.Unmarshal(data, &receivedBody)
		assert.NoError(t, err)

		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookURL := server.URL + "/powerautomate/webhook"

	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": webhookURL,
		},
	})

	notification := Notification{
		TeamsWorkflows: &TeamsWorkflowsNotification{
			Facts: `[{
				"name": "Count",
				"value": 42
			}]`,
		},
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.NoError(t, err)
	card := receivedBody.Attachments[0].Content
	foundFactSet := false
	for _, element := range card.Body {
		if element.Type != "FactSet" {
			continue
		}
		foundFactSet = true
		assert.Len(t, element.Facts, 1)
		assert.Equal(t, "Count", element.Facts[0].Title)
		assert.Equal(t, "42", element.Facts[0].Value) // Should be converted to string
		break
	}
	assert.True(t, foundFactSet, "FactSet should be present")
}

func TestTeamsWorkflows_SectionsWithFacts(t *testing.T) {
	var receivedBody adaptiveMessage
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, err := io.ReadAll(request.Body)
		assert.NoError(t, err)

		err = json.Unmarshal(data, &receivedBody)
		assert.NoError(t, err)

		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookURL := server.URL + "/powerautomate/webhook"

	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": webhookURL,
		},
	})

	notification := Notification{
		TeamsWorkflows: &TeamsWorkflowsNotification{
			Sections: `[{
				"facts": [{
					"name": "Namespace",
					"value": "default"
				}, {
					"name": "Cluster",
					"value": "production"
				}]
			}]`,
		},
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.NoError(t, err)
	card := receivedBody.Attachments[0].Content
	foundFactSet := false
	for _, element := range card.Body {
		if element.Type != "FactSet" {
			continue
		}
		foundFactSet = true
		assert.Len(t, element.Facts, 2)
		assert.Equal(t, "Namespace", element.Facts[0].Title)
		assert.Equal(t, "default", element.Facts[0].Value)
		assert.Equal(t, "Cluster", element.Facts[1].Title)
		assert.Equal(t, "production", element.Facts[1].Value)
		break
	}
	assert.True(t, foundFactSet, "FactSet should be present")
}

func TestTeamsWorkflows_ActionOpenUri(t *testing.T) {
	var receivedBody adaptiveMessage
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, err := io.ReadAll(request.Body)
		assert.NoError(t, err)

		err = json.Unmarshal(data, &receivedBody)
		assert.NoError(t, err)

		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookURL := server.URL + "/powerautomate/webhook"

	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": webhookURL,
		},
	})

	notification := Notification{
		TeamsWorkflows: &TeamsWorkflowsNotification{
			PotentialAction: `[{
				"@type": "OpenUri",
				"name": "View in Argo CD",
				"targets": [{
					"os": "default",
					"uri": "https://argocd.example.com/app/test"
				}]
			}]`,
		},
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.NoError(t, err)
	card := receivedBody.Attachments[0].Content
	assert.Len(t, card.Actions, 1)
	assert.Equal(t, "Action.OpenUrl", card.Actions[0].Type)
	assert.Equal(t, "View in Argo CD", card.Actions[0].Title)
	assert.Equal(t, "https://argocd.example.com/app/test", card.Actions[0].URL)
}

func TestTeamsWorkflows_ActionNonOpenUri(t *testing.T) {
	var receivedBody adaptiveMessage
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, err := io.ReadAll(request.Body)
		assert.NoError(t, err)

		err = json.Unmarshal(data, &receivedBody)
		assert.NoError(t, err)

		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookURL := server.URL + "/powerautomate/webhook"

	service := NewTeamsWorkflowsService(TeamsWorkflowsOptions{
		RecipientUrls: map[string]string{
			"test": webhookURL,
		},
	})

	notification := Notification{
		TeamsWorkflows: &TeamsWorkflowsNotification{
			PotentialAction: `[{
				"@type": "HttpPOST",
				"name": "Post Action",
				"targets": [{
					"uri": "https://example.com/post"
				}]
			}]`,
		},
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams-workflows",
		},
	)

	assert.NoError(t, err)
	card := receivedBody.Attachments[0].Content
	// Non-OpenUri actions should not be converted
	assert.Len(t, card.Actions, 0)
}
