package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/api/chat/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	err = templater(&notification, map[string]any{
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
	assert.Equal(t, "message value", message.Text)
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

	err = templater(&notification, map[string]any{
		"value":     "value",
		"threadKey": "testThreadKey",
	})
	if err != nil {
		t.Error(err)
		return
	}

	assert.NotNil(t, notification.GoogleChat)
	assert.Equal(t, "testThreadKey", notification.GoogleChat.ThreadKey)

	message, err := googleChatNotificationToMessage(notification)
	if err != nil {
		t.Error(err)
		return
	}

	assert.NotNil(t, message)
	assert.Equal(t, "message value", message.Text)
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

	err = templater(&notification, map[string]any{
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
	assert.Equal(t, []chat.Card{
		{
			Sections: []*chat.Section{
				{
					Widgets: []*chat.WidgetMarkup{
						{
							TextParagraph: &chat.TextParagraph{
								Text: "text",
							},
						}, {
							KeyValue: &chat.KeyValue{
								TopLabel: "topLabel",
							},
						}, {
							Image: &chat.Image{
								ImageUrl: "imageUrl",
							},
						}, {
							Buttons: []*chat.Button{
								{
									TextButton: &chat.TextButton{
										Text: "button",
									},
								},
							},
						},
					},
				},
			},
		},
	}, message.Cards)
}

func TestCardV2Message_GoogleChat(t *testing.T) {
	notificationTemplate := Notification{
		GoogleChat: &GoogleChatNotification{
			CardsV2: `
- header:
    title: "Action {{ .action }} as been completed"
    subtitle: Argo Notifications
    imageUrl: https://argo-rollouts.readthedocs.io/en/stable/assets/logo.png
    imageType: CIRCLE
    imageAltText: Argo Logo
  sections:
    - header: Metadata
      collapsible: false
      uncollapsibleWidgetsCount: 1
      widgets:
        - decoratedText:
            startIcon:
                knownIcon: BOOKMARK
            text: "{{ .text }}"
        - buttonList:
            buttons:
              - icon:
                  knownIcon: BOOKMARK
                text: Docs
                onClick:
                  openLink:
                    url: "{{ .button }}"`,
		},
	}

	templater, err := notificationTemplate.GetTemplater("test", template.FuncMap{})
	if err != nil {
		t.Error(err)
		return
	}

	notification := Notification{}

	err = templater(&notification, map[string]any{
		"action": "test",
		"text":   "text",
		"button": "button",
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

	expected := []chat.CardWithId{
		{
			Card: &chat.GoogleAppsCardV1Card{
				Header: &chat.GoogleAppsCardV1CardHeader{
					Title:        "Action test as been completed",
					Subtitle:     "Argo Notifications",
					ImageUrl:     "https://argo-rollouts.readthedocs.io/en/stable/assets/logo.png",
					ImageType:    "CIRCLE",
					ImageAltText: "Argo Logo",
				},
				Sections: []*chat.GoogleAppsCardV1Section{
					{
						Collapsible:               false,
						Header:                    "Metadata",
						UncollapsibleWidgetsCount: 1,
						Widgets: []*chat.GoogleAppsCardV1Widget{
							{
								DecoratedText: &chat.GoogleAppsCardV1DecoratedText{
									StartIcon: &chat.GoogleAppsCardV1Icon{
										KnownIcon: "BOOKMARK",
									},
									Text: "text",
								},
							},
							{
								ButtonList: &chat.GoogleAppsCardV1ButtonList{
									Buttons: []*chat.GoogleAppsCardV1Button{
										{
											Icon: &chat.GoogleAppsCardV1Icon{
												KnownIcon: "BOOKMARK",
											},
											Text: "Docs",
											OnClick: &chat.GoogleAppsCardV1OnClick{
												OpenLink: &chat.GoogleAppsCardV1OpenLink{
													Url: "button",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	assert.NotNil(t, message)
	assert.True(t, cmp.Equal(message.CardsV2, expected, cmpopts.IgnoreFields(chat.CardWithId{}, "CardId")),
		cmp.Diff(message.CardsV2, expected, cmpopts.IgnoreFields(chat.CardWithId{}, "CardId")))
}

func TestCreateClient_NoError(t *testing.T) {
	opts := GoogleChatOptions{WebhookUrls: map[string]string{"test": "testUrl"}}
	service := NewGoogleChatService(opts).(*googleChatService)
	client, err := service.getClient("test")
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "testUrl", client.url)
}

func TestCreateClient_Error(t *testing.T) {
	opts := GoogleChatOptions{WebhookUrls: map[string]string{"test": "testUrl"}}
	service := NewGoogleChatService(opts).(*googleChatService)
	client, err := service.getClient("another")
	require.Error(t, err)
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
	require.NoError(t, err)
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
	require.NoError(t, err)
	assert.True(t, called)
}
