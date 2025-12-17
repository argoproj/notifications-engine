package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"

	"github.com/google/go-cmp/cmp"

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
	assert.Equal(t, []any{
		map[string]any{
			"sections": []any{
				map[string]any{
					"widgets": []any{
						map[string]any{
							"textParagraph": map[string]any{
								"text": "text",
							},
						},
						map[string]any{
							"keyValue": map[string]any{
								"topLabel": "topLabel",
							},
						},
						map[string]any{
							"image": map[string]any{
								"imageUrl": "imageUrl",
							},
						},
						map[string]any{
							"buttons": []any{
								map[string]any{
									"textButton": map[string]any{
										"text": "button",
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
                    url: "{{ .button }}"
        - textParagraph:
            textSyntax: "MARKDOWN"
            text: "**[my-app](https://argocd.local/applications/my-app)**"
        - columns:
            columnItems:
            - widgets:
              - decoratedText:
                  topLabelText:
                    textSyntax: "MARKDOWN"
                    text: "*Health Status*"
                  text: "Degraded"
            - widgets:
              - decoratedText:
                  topLabelText:
                    textSyntax: "MARKDOWN"
                    text: "*Repository*"
                  contentText:
                    textSyntax: "MARKDOWN"
                    text: "https://github.com/my-org/my-repository"`,
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

	expected := []CardV2{
		{
			CardId: "",
			Card: map[string]any{
				"header": map[string]any{
					"title":        "Action test as been completed",
					"subtitle":     "Argo Notifications",
					"imageUrl":     "https://argo-rollouts.readthedocs.io/en/stable/assets/logo.png",
					"imageType":    "CIRCLE",
					"imageAltText": "Argo Logo",
				},
				"sections": []any{
					map[string]any{
						"header":                    "Metadata",
						"collapsible":               false,
						"uncollapsibleWidgetsCount": float64(1),
						"widgets": []any{
							map[string]any{
								"decoratedText": map[string]any{
									"startIcon": map[string]any{
										"knownIcon": "BOOKMARK",
									},
									"text": "text",
								},
							},
							map[string]any{
								"buttonList": map[string]any{
									"buttons": []any{
										map[string]any{
											"icon": map[string]any{
												"knownIcon": "BOOKMARK",
											},
											"text": "Docs",
											"onClick": map[string]any{
												"openLink": map[string]any{
													"url": "button",
												},
											},
										},
									},
								},
							},
							map[string]any{
								"textParagraph": map[string]any{
									"textSyntax": "MARKDOWN",
									"text":       "**[my-app](https://argocd.local/applications/my-app)**",
								},
							},
							map[string]any{
								"columns": map[string]any{
									"columnItems": []any{
										map[string]any{
											"widgets": []any{
												map[string]any{
													"decoratedText": map[string]any{
														"topLabelText": map[string]any{
															"textSyntax": "MARKDOWN",
															"text":       "*Health Status*",
														},
														"text": "Degraded",
													},
												},
											},
										},
										map[string]any{
											"widgets": []any{
												map[string]any{
													"decoratedText": map[string]any{
														"topLabelText": map[string]any{
															"textSyntax": "MARKDOWN",
															"text":       "*Repository*",
														},
														"contentText": map[string]any{
															"textSyntax": "MARKDOWN",
															"text":       "https://github.com/my-org/my-repository",
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
			},
		},
	}
	assert.NotNil(t, message)
	if diff := cmp.Diff(
		message.CardsV2,
		expected,
		cmp.FilterPath(
			func(path cmp.Path) bool {
				return path.Last().String() == ".CardId"
			},
			cmp.Ignore(),
		),
	); diff != "" {
		assert.Fail(t, diff)
	}
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
