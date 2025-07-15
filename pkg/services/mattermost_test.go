package services

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSend_Mattermost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		assert.JSONEq(t, `{
			"channel_id": "channel",
			"message": "message",
			"props": {
				"attachments": [{
					"title": "title",
					"title_link": "https://argocd.example.com/applications/argocd-notifications",
					"color": "#18be52",
					"fields": [{
						"title": "Sync Status",
						"value": "Synced",
						"short": true
					}, {
						"title": "Repository",
						"value": "https://example.com",
						"short": true
					}]
				}]
			}
		}`, string(b))
	}))
	defer ts.Close()

	service := NewMattermostService(MattermostOptions{
		ApiURL:             ts.URL,
		Token:              "token",
		InsecureSkipVerify: true,
	})
	err := service.Send(Notification{
		Message: "message",
		Mattermost: &MattermostNotification{
			Attachments: `[{
				"title": "title",
				"title_link": "https://argocd.example.com/applications/argocd-notifications",
				"color": "#18be52",
				"fields": [{
					"title": "Sync Status",
					"value": "Synced",
					"short": true
				}, {
					"title": "Repository",
					"value": "https://example.com",
					"short": true
				}]
			}]`,
		},
	}, Destination{
		Service:   "mattermost",
		Recipient: "channel",
	})
	require.NoError(t, err)
}

func TestGetTemplater_Mattermost(t *testing.T) {
	n := Notification{
		Mattermost: &MattermostNotification{
			Attachments: "{{.foo}}",
		},
	}
	templater, err := n.GetTemplater("", template.FuncMap{})

	require.NoError(t, err)

	var notification Notification
	err = templater(&notification, map[string]any{
		"foo": "hello",
	})

	require.NoError(t, err)

	assert.Equal(t, "hello", notification.Mattermost.Attachments)
}
