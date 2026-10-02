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

func TestValidEmoji(t *testing.T) {
	assert.True(t, validEmoji.MatchString(":slack:"))
	assert.True(t, validEmoji.MatchString(":chart_with_upwards_trend:"))
	assert.False(t, validEmoji.MatchString("http://lorempixel.com/48/48"))
}

func TestValidAvatarURL(t *testing.T) {
	assert.True(t, isValidAvatarURL("http://lorempixel.com/48/48"))
	assert.True(t, isValidAvatarURL("https://lorempixel.com/48/48"))
	assert.False(t, isValidAvatarURL("favicon.ico"))
	assert.False(t, isValidAvatarURL("ftp://favicon.ico"))
	assert.False(t, isValidAvatarURL("ftp://lorempixel.com/favicon.ico"))
}

func TestSend_RocketChatWebhook(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		assert.JSONEq(t, `{
			"text": "message",
			"alias": "argocd",
			"channel": "#my_channel",
			"attachments": [{
				"title": "title",
				"collapsed": false
			}]
		}`, string(b))

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	service := NewRocketChatService(RocketChatOptions{
		Alias:              "argocd",
		WebhookUrl:         ts.URL,
		InsecureSkipVerify: true,
	})
	err := service.Send(Notification{
		Message: "message",
		RocketChat: &RocketChatNotification{
			Attachments: `[{"title": "title"}]`,
		},
	}, Destination{
		Service:   "rocketchat",
		Recipient: "my_channel",
	})
	require.NoError(t, err)
}

func TestSend_RocketChatWebhook_ChannelAlreadyPrefixed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"text": "message", "channel": "@someuser"}`, string(b))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	service := NewRocketChatService(RocketChatOptions{WebhookUrl: ts.URL})
	err := service.Send(Notification{Message: "message"}, Destination{Recipient: "@someuser"})
	require.NoError(t, err)
}

func TestSend_RocketChatWebhook_InvalidIconAndAvatarAreIgnored(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"text": "message", "channel": "#chan"}`, string(b))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	service := NewRocketChatService(RocketChatOptions{
		WebhookUrl: ts.URL,
		Icon:       "not-an-emoji",
		Avatar:     "not-a-url",
	})
	err := service.Send(Notification{Message: "message"}, Destination{Recipient: "#chan"})
	require.NoError(t, err)
}

func TestSend_RocketChatWebhook_InvalidAttachments(t *testing.T) {
	service := NewRocketChatService(RocketChatOptions{WebhookUrl: "http://example.invalid"})
	err := service.Send(Notification{
		Message:    "message",
		RocketChat: &RocketChatNotification{Attachments: "not-json"},
	}, Destination{Recipient: "#chan"})
	require.ErrorContains(t, err, "failed to unmarshal attachments")
}

func TestSend_RocketChatWebhook_NonSuccessStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer ts.Close()

	service := NewRocketChatService(RocketChatOptions{WebhookUrl: ts.URL})
	err := service.Send(Notification{Message: "message"}, Destination{Recipient: "#chan"})
	require.ErrorContains(t, err, "rocketchat webhook post error")
}

func TestSend_RocketChatWebhook_RequestCreationError(t *testing.T) {
	service := NewRocketChatService(RocketChatOptions{WebhookUrl: "http://x\x00y"})
	err := service.Send(Notification{Message: "message"}, Destination{Recipient: "#chan"})
	require.ErrorContains(t, err, "failed to create request")
}

func TestSend_RocketChatWebhook_RequestError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	ts.Close()

	service := NewRocketChatService(RocketChatOptions{WebhookUrl: ts.URL})
	err := service.Send(Notification{Message: "message"}, Destination{Recipient: "#chan"})
	require.ErrorContains(t, err, "failed to request")
}

func TestGetTemplater_RocketChat(t *testing.T) {
	n := Notification{
		RocketChat: &RocketChatNotification{
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

	assert.Equal(t, "hello", notification.RocketChat.Attachments)
}
