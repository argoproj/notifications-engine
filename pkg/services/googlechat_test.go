package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

func TestTextMessage_GoogleChat(t *testing.T) {
	notificationTemplate := Notification{
		Message: "message {{.value}}",
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

	assert.Nil(t, notification.GoogleChat)

	message, err := googleChatNotificationToMessage(notification)
	if err != nil {
		t.Error(err)
		return
	}

	assert.NotNil(t, message)
	assert.Equal(t, message.Text, "message value")
}

func TestTextMessageWithThreadKey_GoogleChat(t *testing.T) {
	notificationTemplate := Notification{
		Message: "message {{.value}}",
		GoogleChat: &GoogleChatNotification{
			ThreadKey: "{{.threadKey}}",
		},
	}

	templater, err := notificationTemplate.GetTemplater("test", template.FuncMap{})
	if err != nil {
		t.Error(err)
		return
	}

	notification := Notification{}

	err = templater(&notification, map[string]interface{}{
		"value":     "value",
		"threadKey": "testThreadKey",
	})

	if err != nil {
		t.Error(err)
		return
	}

	assert.NotNil(t, notification.GoogleChat)
	assert.Equal(t, notification.GoogleChat.ThreadKey, "testThreadKey")

	message, err := googleChatNotificationToMessage(notification)
	if err != nil {
		t.Error(err)
		return
	}

	assert.NotNil(t, message)
	assert.Equal(t, message.Text, "message value")
}

func TestCardMessage_GoogleChat(t *testing.T) {
	notificationTemplate := Notification{
		GoogleChat: &GoogleChatNotification{
			Cards: `- sections:
  - widgets:
    - textParagraph:
        text: {{.text}}
    - keyValue:
        topLabel: {{.topLabel}}
    - image:
        imageUrl: {{.imageUrl}}
    - buttons:
      - textButton:
          text: {{.button}}`,
		},
	}

	templater, err := notificationTemplate.GetTemplater("test", template.FuncMap{})
	if err != nil {
		t.Error(err)
		return
	}

	notification := Notification{}

	err = templater(&notification, map[string]interface{}{
		"text":     "text",
		"topLabel": "topLabel",
		"imageUrl": "imageUrl",
		"button":   "button",
	})

	if err != nil {
		t.Error(err)
		return
	}

	message, err := googleChatNotificationToMessage(notification)
	if err != nil {
		t.Error(err)
		return
	}

	assert.NotNil(t, message)
	assert.Equal(t, message.Cards, []cardMessage{
		{
			Sections: []cardSection{
				{
					Widgets: []cardWidget{
						{
							TextParagraph: map[string]interface{}{
								"text": "text",
							},
						}, {
							Keyvalue: map[string]interface{}{
								"topLabel": "topLabel",
							},
						}, {
							Image: map[string]interface{}{
								"imageUrl": "imageUrl",
							},
						}, {
							Buttons: []map[string]interface{}{
								{
									"textButton": map[string]interface{}{
										"text": "button",
									},
								},
							},
						},
					},
				},
			},
		},
	})
}

func TestCreateClient_NoError(t *testing.T) {
	opts := GoogleChatOptions{WebhookUrls: map[string]string{"test": "testUrl"}}
	service := NewGoogleChatService(opts).(*googleChatService)
	client, err := service.getClient("test")
	assert.Nil(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "testUrl", client.url)
}

func TestCreateClient_Error(t *testing.T) {
	opts := GoogleChatOptions{WebhookUrls: map[string]string{"test": "testUrl"}}
	service := NewGoogleChatService(opts).(*googleChatService)
	client, err := service.getClient("another")
	assert.NotNil(t, err)
	assert.Nil(t, client)
}

func TestSendMessage_NoError(t *testing.T) {
	called := false
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		called = true

		if err := req.ParseForm(); err != nil {
			t.Fatal("error on parse form")
		}
		assert.False(t, req.Form.Has("threadKey"), "threadKey query param should not be set")

		res.WriteHeader(http.StatusOK)
		_, err := res.Write([]byte("{}"))
		if err != nil {
			t.Fatal("error on write response body")
		}
	}))
	defer func() { testServer.Close() }()

	opts := GoogleChatOptions{WebhookUrls: map[string]string{"test": testServer.URL}}
	service := NewGoogleChatService(opts).(*googleChatService)
	notification := Notification{Message: ""}
	destination := Destination{Recipient: "test"}
	err := service.Send(notification, destination)
	assert.Nil(t, err)
	assert.True(t, called)
}

func TestSendMessageWithThreadKey_NoError(t *testing.T) {
	called := false
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		called = true

		if err := req.ParseForm(); err != nil {
			t.Fatal("error on parse form")
		}
		assert.Equal(t, "testThreadKey", req.Form.Get("threadKey"), "threadKey query param should be set")

		res.WriteHeader(http.StatusOK)
		_, err := res.Write([]byte("{}"))
		if err != nil {
			t.Fatal("error on write response body")
		}
	}))
	defer func() { testServer.Close() }()

	opts := GoogleChatOptions{WebhookUrls: map[string]string{"test": testServer.URL}}
	service := NewGoogleChatService(opts).(*googleChatService)
	notification := Notification{Message: "", GoogleChat: &GoogleChatNotification{ThreadKey: "testThreadKey"}}
	destination := Destination{Recipient: "test"}
	err := service.Send(notification, destination)
	assert.Nil(t, err)
	assert.True(t, called)
}
