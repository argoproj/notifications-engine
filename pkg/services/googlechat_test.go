package services

import (
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

	templater(&notification, map[string]interface{}{
		"value": "value",
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

	templater(&notification, map[string]interface{}{
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
