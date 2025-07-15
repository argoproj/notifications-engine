package services

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"text/template"

	slackutil "github.com/argoproj/notifications-engine/pkg/util/slack"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidIconEmoji(t *testing.T) {
	assert.True(t, validIconEmoji.MatchString(":slack:"))
	assert.True(t, validIconEmoji.MatchString(":chart_with_upwards_trend:"))
	assert.False(t, validIconEmoji.MatchString("http://lorempixel.com/48/48"))
}

func TestValidIconURL(t *testing.T) {
	assert.True(t, isValidIconURL("http://lorempixel.com/48/48"))
	assert.True(t, isValidIconURL("https://lorempixel.com/48/48"))
	assert.False(t, isValidIconURL("favicon.ico"))
	assert.False(t, isValidIconURL("ftp://favicon.ico"))
	assert.False(t, isValidIconURL("ftp://lorempixel.com/favicon.ico"))
}

func TestGetTemplater_Slack(t *testing.T) {
	n := Notification{
		Slack: &SlackNotification{
			Username:        "{{.bar}}-{{.foo}}",
			Icon:            ":{{.foo}}:",
			Attachments:     "{{.foo}}",
			Blocks:          "{{.bar}}",
			GroupingKey:     "{{.foo}}-{{.bar}}",
			NotifyBroadcast: true,
		},
	}
	templater, err := n.GetTemplater("", template.FuncMap{})

	require.NoError(t, err)

	var notification Notification
	err = templater(&notification, map[string]any{
		"foo": "hello",
		"bar": "world",
	})

	require.NoError(t, err)

	assert.Equal(t, "world-hello", notification.Slack.Username)
	assert.Equal(t, ":hello:", notification.Slack.Icon)
	assert.Equal(t, "hello", notification.Slack.Attachments)
	assert.Equal(t, "world", notification.Slack.Blocks)
	assert.Equal(t, "hello-world", notification.Slack.GroupingKey)
	assert.True(t, notification.Slack.NotifyBroadcast)
}

func TestBuildMessageOptionsWithNonExistTemplate(t *testing.T) {
	n := Notification{}

	sn, opts, err := buildMessageOptions(n, SlackOptions{})
	require.NoError(t, err)
	assert.Len(t, opts, 1)
	assert.Empty(t, sn.GroupingKey)
	assert.Equal(t, slackutil.Post, sn.DeliveryPolicy)
}

type chatResponseFull struct {
	Channel          string `json:"channel"`
	Timestamp        string `json:"ts"`         // Regular message timestamp
	MessageTimeStamp string `json:"message_ts"` // Ephemeral message timestamp
	Text             string `json:"text"`
}

func TestSlack_SendNotification(t *testing.T) {
	dummyResponse, err := json.Marshal(chatResponseFull{
		Channel:          "test",
		Timestamp:        "1503435956.000247",
		MessageTimeStamp: "1503435956.000247",
		Text:             "text",
	})
	require.NoError(t, err)

	t.Run("only message", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			data, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			v := url.Values{}
			v.Add("channel", "test-channel")
			v.Add("text", "Annotation description")
			v.Add("token", "something-token")
			assert.Equal(t, string(data), v.Encode())

			writer.WriteHeader(http.StatusOK)
			_, err = writer.Write(dummyResponse)
			require.NoError(t, err)
		}))
		defer server.Close()

		service := NewSlackService(SlackOptions{
			ApiURL:             server.URL + "/",
			Token:              "something-token",
			InsecureSkipVerify: true,
		})

		err := service.Send(
			Notification{Message: "Annotation description"},
			Destination{Recipient: "test-channel", Service: "slack"},
		)
		require.NoError(t, err)
	})

	t.Run("attachments", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			data, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			v := url.Values{}
			v.Add("attachments", `[{"pretext":"pre-hello","text":"text-world","blocks":null}]`)
			v.Add("channel", "test")
			v.Add("text", "Attachments description")
			v.Add("token", "something-token")
			assert.Equal(t, string(data), v.Encode())

			writer.WriteHeader(http.StatusOK)
			_, err = writer.Write(dummyResponse)
			require.NoError(t, err)
		}))
		defer server.Close()

		service := NewSlackService(SlackOptions{
			ApiURL:             server.URL + "/",
			Token:              "something-token",
			InsecureSkipVerify: true,
		})

		err := service.Send(
			Notification{
				Message: "Attachments description",
				Slack: &SlackNotification{
					Attachments: `[{"pretext": "pre-hello", "text": "text-world"}]`,
				},
			},
			Destination{Recipient: "test", Service: "slack"},
		)
		require.NoError(t, err)
	})

	t.Run("blocks", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			data, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			v := url.Values{}
			v.Add("attachments", "[]")
			v.Add("blocks", `[{"type":"section","text":{"type":"plain_text","text":"Hello world"}}]`)
			v.Add("channel", "test")
			v.Add("text", "Attachments description")
			v.Add("token", "something-token")
			assert.Equal(t, string(data), v.Encode())

			writer.WriteHeader(http.StatusOK)
			_, err = writer.Write(dummyResponse)
			require.NoError(t, err)
		}))
		defer server.Close()

		service := NewSlackService(SlackOptions{
			ApiURL:             server.URL + "/",
			Token:              "something-token",
			InsecureSkipVerify: true,
		})

		err := service.Send(
			Notification{
				Message: "Attachments description",
				Slack: &SlackNotification{
					Blocks: `[{"type": "section", "text": {"type": "plain_text", "text": "Hello world"}}]`,
				},
			},
			Destination{Recipient: "test", Service: "slack"},
		)
		require.NoError(t, err)
	})
}

func TestSlack_SetUsernameAndIcon(t *testing.T) {
	dummyResponse, err := json.Marshal(chatResponseFull{
		Channel:          "test",
		Timestamp:        "1503435956.000247",
		MessageTimeStamp: "1503435956.000247",
		Text:             "text",
	})
	require.NoError(t, err)

	t.Run("no set", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			data, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			v := url.Values{}
			v.Add("channel", "test")
			v.Add("text", "test")
			v.Add("token", "something-token")
			assert.Equal(t, string(data), v.Encode())

			writer.WriteHeader(http.StatusOK)
			_, err = writer.Write(dummyResponse)
			require.NoError(t, err)
		}))
		defer server.Close()

		service := NewSlackService(SlackOptions{
			ApiURL:             server.URL + "/",
			Token:              "something-token",
			InsecureSkipVerify: true,
		})

		err := service.Send(
			Notification{
				Message: "test",
			},
			Destination{Recipient: "test", Service: "slack"},
		)
		require.NoError(t, err)
	})

	t.Run("set service config", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			data, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			v := url.Values{}
			v.Add("channel", "test")
			v.Add("icon_emoji", ":smile:")
			v.Add("text", "test")
			v.Add("token", "something-token")
			v.Add("username", "foo")

			assert.Equal(t, string(data), v.Encode())

			writer.WriteHeader(http.StatusOK)
			_, err = writer.Write(dummyResponse)
			require.NoError(t, err)
		}))
		defer server.Close()

		service := NewSlackService(SlackOptions{
			ApiURL:             server.URL + "/",
			Token:              "something-token",
			Username:           "foo",
			Icon:               ":smile:",
			InsecureSkipVerify: true,
		})

		err := service.Send(
			Notification{
				Message: "test",
			},
			Destination{Recipient: "test", Service: "slack"},
		)
		require.NoError(t, err)
	})

	t.Run("set service config and template", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			data, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			v := url.Values{}
			v.Add("attachments", "[]")
			v.Add("channel", "test")
			v.Add("icon_emoji", ":wink:")
			v.Add("text", "test")
			v.Add("token", "something-token")
			v.Add("username", "template set")

			assert.Equal(t, string(data), v.Encode())

			writer.WriteHeader(http.StatusOK)
			_, err = writer.Write(dummyResponse)
			require.NoError(t, err)
		}))
		defer server.Close()

		service := NewSlackService(SlackOptions{
			ApiURL:             server.URL + "/",
			Token:              "something-token",
			Username:           "foo",
			Icon:               ":smile:",
			InsecureSkipVerify: true,
		})

		err := service.Send(
			Notification{
				Message: "test",
				Slack: &SlackNotification{
					Username: "template set",
					Icon:     ":wink:",
				},
			},
			Destination{Recipient: "test", Service: "slack"},
		)
		require.NoError(t, err)
	})
}
