package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"

	"github.com/google/go-cmp/cmp"

	"github.com/stretchr/testify/assert"
)

// func TestGoogleChat_CardV2(t *testing.T) {
// 	called := false
// 	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
// 		called = true

// 		bodyBytes, err := io.ReadAll(req.Body)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		bodyParsed := map[string]interface{}{}
// 		err = json.Unmarshal(bodyBytes, &bodyParsed)
// 		if err != nil {
// 			t.Fatal("error on write response body")
// 		}
// 		if diff := cmp.Diff(
// 			bodyParsed,
// 			map[string]interface{}{
// 				"cardsV2": []interface{}{},
// 			},
// 			cmp.FilterPath(
// 				func(path cmp.Path) bool {
// 					return path.Last().String() == ".CardId"
// 				},
// 				cmp.Ignore(),
// 			),
// 		); diff != "" {
// 			assert.Fail(t, "%s", diff)
// 		}

// 		res.WriteHeader(http.StatusOK)
// 		_, err = res.Write([]byte("{}"))
// 		if err != nil {
// 			t.Fatal("error on write response body")
// 		}
// 	}))
// 	defer func() { testServer.Close() }()

// 	opts := GoogleChatOptions{WebhookUrls: map[string]string{"test": testServer.URL}}
// 	service := NewGoogleChatService(opts).(*googleChatService)

// 	notification := Notification{
// 		Message: "message {{.value}}",
// 		GoogleChat: &GoogleChatNotification{
// 			CardsV2: `- sections:
//   - widgets:
//     - textParagraph:
//         textSyntax: "MARKDOWN"
//         text: "❗Application **[my-app](https://argocd.local/applications/my-app)** on **MyEnv** has degraded."
//     - columns:
//         columnItems:
//         - widgets:
//           - decoratedText:
//               topLabelText:
//                 textSyntax: "MARKDOWN"
//                 text: "*Health Status*"
//               text: "Degraded"
//         - widgets:
//           - decoratedText:
//               topLabelText:
//                 textSyntax: "MARKDOWN"
//                 text: "*Repository*"
//               contentText:
//                 textSyntax: "MARKDOWN"
//                 text: "https://github.com/loganmzz/my-repository"
// `,
// 		},
// 	}
// 	destination := Destination{Recipient: "test"}

// 	err := service.Send(notification, destination)
// 	assert.Nil(t, err)
// 	assert.True(t, called)
// }

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
	assert.Equal(t, message.Cards, []interface{}{
		map[string]interface{}{
			"sections": []interface{}{
				map[string]interface{}{
					"widgets": []interface{}{
						map[string]interface{}{
							"textParagraph": map[string]interface{}{
								"text": "text",
							},
						},
						map[string]interface{}{
							"keyValue": map[string]interface{}{
								"topLabel": "topLabel",
							},
						},
						map[string]interface{}{
							"image": map[string]interface{}{
								"imageUrl": "imageUrl",
							},
						},
						map[string]interface{}{
							"buttons": []interface{}{
								map[string]interface{}{
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
	// {
	// 	"sections": []interface{
	// 			Widgets: []interface{
	// 				{
	// 					"textParagraph": {
	// 							"text": "text",
	// 						},
	// 					},
	// 				},
	// 				{
	// 					"keyValue": {
	// 							"topLabel": "topLabel",
	// 						},
	// 					},
	// 				},
	// 				{
	// 					"image": {
	// 							"imageUrl": "imageUrl",
	// 						},
	// 					},
	// 				},
	// 				{
	// 					Buttons: []interface{
	// 						{
	// 							"type": {
	// 								"textButton": {
	// 									"text": "button",
	// 								},
	// 							},
	// 						},
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// },
	// {
	// 	Sections: []*card.Card_Section{
	// 		{
	// 			Widgets: []*card.Widget{
	// 				&card.Widget{
	// 					Data: &card.Widget_TextParagraph{
	// 						TextParagraph: &card.TextParagraph{
	// 							Text: "text",
	// 					},
	// 				},
	// 				&card.Widget{
	// 				  Data: {
	// 						KeyValue: &card.KeyValue{
	// 							TopLabel: "topLabel",
	// 						},
	// 					},
	// 				},
	// 				&card.Widget{
	// 				  Data: {
	// 						Image: &card.Image{
	// 							ImageUrl: "imageUrl",
	// 						},
	// 					},
	// 				},
	// 				&card.Widget{
	// 					Data: {
	// 						Buttons: []*card.Button{
	// 							{
	// 								TextButton: &card.TextButton{
	// 									Text: "button",
	// 								},
	// 							},
	// 						},
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// },
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

	err = templater(&notification, map[string]interface{}{
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
			Card: map[string]interface{}{
				"header": map[string]interface{}{
					"title":        "Action test as been completed",
					"subtitle":     "Argo Notifications",
					"imageUrl":     "https://argo-rollouts.readthedocs.io/en/stable/assets/logo.png",
					"imageType":    "CIRCLE",
					"imageAltText": "Argo Logo",
				},
				"sections": []interface{}{
					map[string]interface{}{
						"header":                    "Metadata",
						"collapsible":               false,
						"uncollapsibleWidgetsCount": float64(1),
						"widgets": []interface{}{
							map[string]interface{}{
								"decoratedText": map[string]interface{}{
									"startIcon": map[string]interface{}{
										"knownIcon": "BOOKMARK",
									},
									"text": "text",
								},
							},
							map[string]interface{}{
								"buttonList": map[string]interface{}{
									"buttons": []interface{}{
										map[string]interface{}{
											"icon": map[string]interface{}{
												"knownIcon": "BOOKMARK",
											},
											"text": "Docs",
											"onClick": map[string]interface{}{
												"openLink": map[string]interface{}{
													"url": "button",
												},
											},
										},
									},
								},
							},
							map[string]interface{}{
								"textParagraph": map[string]interface{}{
									"textSyntax": "MARKDOWN",
									"text":       "**[my-app](https://argocd.local/applications/my-app)**",
								},
							},
							map[string]interface{}{
								"columns": map[string]interface{}{
									"columnItems": []interface{}{
										map[string]interface{}{
											"widgets": []interface{}{
												map[string]interface{}{
													"decoratedText": map[string]interface{}{
														"topLabelText": map[string]interface{}{
															"textSyntax": "MARKDOWN",
															"text":       "*Health Status*",
														},
														"text": "Degraded",
													},
												},
											},
										},
										map[string]interface{}{
											"widgets": []interface{}{
												map[string]interface{}{
													"decoratedText": map[string]interface{}{
														"topLabelText": map[string]interface{}{
															"textSyntax": "MARKDOWN",
															"text":       "*Repository*",
														},
														"contentText": map[string]interface{}{
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
		assert.Fail(t, "%s", diff)
	}
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
