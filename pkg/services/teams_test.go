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

func TestGetTemplater_Teams(t *testing.T) {
	notificationTemplate := Notification{
		Teams: &TeamsNotification{
			Template:        "template {{.value}}",
			Title:           "title {{.value}}",
			Summary:         "summary {{.value}}",
			Text:            "text {{.value}}",
			Facts:           "facts {{.value}}",
			Sections:        "sections {{.value}}",
			PotentialAction: "actions {{.value}}",
			ThemeColor:      "theme color {{.value}}",
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

	assert.Equal(t, notification.Teams.Template, "template value")
	assert.Equal(t, notification.Teams.Title, "title value")
	assert.Equal(t, notification.Teams.Summary, "summary value")
	assert.Equal(t, notification.Teams.Text, "text value")
	assert.Equal(t, notification.Teams.Sections, "sections value")
	assert.Equal(t, notification.Teams.Facts, "facts value")
	assert.Equal(t, notification.Teams.PotentialAction, "actions value")
	assert.Equal(t, notification.Teams.ThemeColor, "theme color value")
}

func TestTeams_DefaultMessage(t *testing.T) {
	var receivedBody teamsMessage
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, err := io.ReadAll(request.Body)
		assert.NoError(t, err)

		err = json.Unmarshal(data, &receivedBody)
		assert.NoError(t, err)

		_, err = writer.Write([]byte("1"))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewTeamsService(TeamsOptions{
		RecipientUrls: map[string]string{
			"test": server.URL,
		},
	})

	notification := Notification{
		Message: "simple message",
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "test",
		},
	)

	assert.NoError(t, err)

	assert.Equal(t, receivedBody.Text, notification.Message)
}

func TestTeams_TemplateMessage(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, err := io.ReadAll(request.Body)
		assert.NoError(t, err)

		receivedBody = string(data)

		_, err = writer.Write([]byte("1"))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewTeamsService(TeamsOptions{
		RecipientUrls: map[string]string{
			"test": server.URL,
		},
	})

	notification := Notification{
		Teams: &TeamsNotification{
			Template: "template body",
		},
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "test",
		},
	)

	assert.NoError(t, err)

	assert.Equal(t, receivedBody, notification.Teams.Template)
}

func TestTeams_MessageFields(t *testing.T) {
	var receivedBody teamsMessage
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, err := io.ReadAll(request.Body)
		assert.NoError(t, err)

		err = json.Unmarshal(data, &receivedBody)
		assert.NoError(t, err)

		_, err = writer.Write([]byte("1"))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewTeamsService(TeamsOptions{
		RecipientUrls: map[string]string{
			"test": server.URL,
		},
	})

	notification := Notification{
		Message: "welcome message",
		Teams: &TeamsNotification{
			Text:            "Text",
			Facts:           "[{\"facts\": true}]",
			Sections:        "[{\"sections\": true}]",
			PotentialAction: "[{\"actions\": true}]",
			Title:           "Title",
			Summary:         "Summary",
			ThemeColor:      "#000080",
		},
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "test",
		},
	)

	assert.NoError(t, err)

	assert.Contains(t, receivedBody.Text, notification.Teams.Text)
	assert.Contains(t, receivedBody.Title, notification.Teams.Title)
	assert.Contains(t, receivedBody.Summary, notification.Teams.Summary)
	assert.Contains(t, receivedBody.ThemeColor, notification.Teams.ThemeColor)
	assert.Contains(t, receivedBody.PotentialAction, teamsAction{"actions": true})
	assert.Contains(t, receivedBody.Sections, teamsSection{"sections": true})
	assert.EqualValues(t, receivedBody.Sections[len(receivedBody.Sections)-1]["facts"],
		[]interface{}{
			map[string]interface{}{
				"facts": true,
			},
		})
}

func TestTeams_Office365Connector_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, err := writer.Write([]byte("1"))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewTeamsService(TeamsOptions{
		RecipientUrls: map[string]string{
			"test": server.URL,
		},
	})

	notification := Notification{
		Message: "test message",
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams",
		},
	)

	assert.NoError(t, err)
}

func TestTeams_Office365Connector_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, err := writer.Write([]byte("error message"))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewTeamsService(TeamsOptions{
		RecipientUrls: map[string]string{
			"test": server.URL,
		},
	})

	notification := Notification{
		Message: "test message",
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams",
		},
	)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "teams webhook post error")
	assert.Contains(t, err.Error(), "error message")
}

func TestTeams_WorkflowsWebhook_StatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, err := writer.Write([]byte("1"))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewTeamsService(TeamsOptions{
		RecipientUrls: map[string]string{
			"test": server.URL,
		},
	})

	notification := Notification{
		Message: "test message",
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams",
		},
	)

	// Teams service only checks response body, not status code
	// If body is "1", it succeeds regardless of status code
	assert.NoError(t, err)
}

func TestTeams_Office365Connector_NonOneResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, err := writer.Write([]byte("not one"))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewTeamsService(TeamsOptions{
		RecipientUrls: map[string]string{
			"test": server.URL,
		},
	})

	notification := Notification{
		Message: "test message",
	}

	err := service.Send(notification,
		Destination{
			Recipient: "test",
			Service:   "teams",
		},
	)

	// Office365-connector requires "1" response, so this should fail
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "teams webhook post error")
	assert.Contains(t, err.Error(), "not one")
}
