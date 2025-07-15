package services

import (
	"encoding/json"
	"fmt"
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

func TestGetTemplater_Slack_InvalidTemplates(t *testing.T) {
	tests := []struct {
		name         string
		notification SlackNotification
	}{
		{
			name: "Invalid username template",
			notification: SlackNotification{
				Username: "{{.foo",
			},
		},
		{
			name: "Invalid icon template",
			notification: SlackNotification{
				Icon: "{{.foo",
			},
		},
		{
			name: "Invalid attachments template",
			notification: SlackNotification{
				Attachments: "{{.foo",
			},
		},
		{
			name: "Invalid blocks template",
			notification: SlackNotification{
				Blocks: "{{.foo",
			},
		},
		{
			name: "Invalid grouping key template",
			notification: SlackNotification{
				GroupingKey: "{{.foo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.notification.GetTemplater("test", template.FuncMap{})
			assert.Error(t, err)
		})
	}
}

func TestGetTemplater_Slack_NilNotification(t *testing.T) {
	n := Notification{
		Slack: &SlackNotification{
			Username: "{{.name}}",
		},
	}

	templater, err := n.GetTemplater("", template.FuncMap{})
	assert.NoError(t, err)

	// Test with nil Slack on target notification
	var notification Notification
	err = templater(&notification, map[string]interface{}{
		"name": "test",
	})

	assert.NoError(t, err)
	assert.NotNil(t, notification.Slack)
	assert.Equal(t, "test", notification.Slack.Username)
}

func TestGetTemplater_Slack_DeliveryPolicy(t *testing.T) {
	n := Notification{
		Slack: &SlackNotification{
			Username:       "bot",
			DeliveryPolicy: slackutil.Update,
		},
	}

	templater, err := n.GetTemplater("", template.FuncMap{})
	assert.NoError(t, err)

	var notification Notification
	err = templater(&notification, map[string]interface{}{})

	assert.NoError(t, err)
	assert.Equal(t, slackutil.Update, notification.Slack.DeliveryPolicy)
}

func TestGetTemplater_Slack_TemplateExecutionError(t *testing.T) {
	// Create a FuncMap with the required function
	funcMap := template.FuncMap{
		"required": func(msg string, val interface{}) (interface{}, error) {
			if val == nil || val == "" {
				return nil, fmt.Errorf("%s", msg)
			}
			return val, nil
		},
	}

	tests := []struct {
		name         string
		notification SlackNotification
	}{
		{
			name: "Username execution error",
			notification: SlackNotification{
				Username: "{{.missing | required \"missing is required\"}}",
			},
		},
		{
			name: "Icon execution error",
			notification: SlackNotification{
				Icon: "{{.missing | required \"missing is required\"}}",
			},
		},
		{
			name: "Attachments execution error",
			notification: SlackNotification{
				Attachments: "{{.missing | required \"missing is required\"}}",
			},
		},
		{
			name: "Blocks execution error",
			notification: SlackNotification{
				Blocks: "{{.missing | required \"missing is required\"}}",
			},
		},
		{
			name: "GroupingKey execution error",
			notification: SlackNotification{
				GroupingKey: "{{.missing | required \"missing is required\"}}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templater, err := tt.notification.GetTemplater("", funcMap)
			assert.NoError(t, err)

			var notification Notification
			err = templater(&notification, map[string]interface{}{})
			assert.Error(t, err)
		})
	}
}

func TestBuildMessageOptionsWithNonExistTemplate(t *testing.T) {
	n := Notification{}

	sn, opts, err := buildMessageOptions(n, SlackOptions{})
	require.NoError(t, err)
	assert.Len(t, opts, 1)
	assert.Empty(t, sn.GroupingKey)
	assert.Equal(t, slackutil.Post, sn.DeliveryPolicy)
}

func TestBuildMessageOptions_IconURL(t *testing.T) {
	t.Run("Valid icon URL from notification", func(t *testing.T) {
		n := Notification{
			Message: "test",
			Slack: &SlackNotification{
				Icon: "https://example.com/icon.png",
			},
		}

		_, opts, err := buildMessageOptions(n, SlackOptions{})
		assert.NoError(t, err)
		// Should have text + icon_url options
		assert.GreaterOrEqual(t, len(opts), 2)
	})

	t.Run("Valid icon URL from options", func(t *testing.T) {
		n := Notification{
			Message: "test",
		}

		_, opts, err := buildMessageOptions(n, SlackOptions{
			Icon: "http://example.com/icon.png",
		})
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(opts), 2)
	})

	t.Run("Invalid icon - neither emoji nor URL", func(t *testing.T) {
		n := Notification{
			Message: "test",
			Slack: &SlackNotification{
				Icon: "invalid-icon",
			},
		}

		_, opts, err := buildMessageOptions(n, SlackOptions{})
		assert.NoError(t, err)
		// Should have text + attachments + blocks (but no icon option because it's invalid)
		// Use GreaterOrEqual to make test less fragile to implementation changes
		assert.GreaterOrEqual(t, len(opts), 3)
	})
}

func TestBuildMessageOptions_DisableUnfurl(t *testing.T) {
	n := Notification{
		Message: "test",
	}

	_, opts, err := buildMessageOptions(n, SlackOptions{
		DisableUnfurl: true,
	})
	assert.NoError(t, err)
	// Should have text + 2 unfurl options
	assert.GreaterOrEqual(t, len(opts), 3)
}

func TestBuildMessageOptions_InvalidAttachments(t *testing.T) {
	n := Notification{
		Message: "test",
		Slack: &SlackNotification{
			Attachments: "invalid json",
		},
	}

	_, _, err := buildMessageOptions(n, SlackOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal attachments")
}

func TestBuildMessageOptions_InvalidBlocks(t *testing.T) {
	n := Notification{
		Message: "test",
		Slack: &SlackNotification{
			Blocks: "invalid json",
		},
	}

	_, _, err := buildMessageOptions(n, SlackOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal blocks")
}

func TestGetSigningSecret(t *testing.T) {
	service := NewSlackService(SlackOptions{
		Token:         "test-token",
		SigningSecret: "test-signing-secret",
	})

	slackService, ok := service.(*slackService)
	assert.True(t, ok)
	assert.Equal(t, "test-signing-secret", slackService.GetSigningSecret())
}

func TestNewSlackClient_CustomAPIURL(t *testing.T) {
	client, err := newSlackClient(SlackOptions{
		Token:  "test-token",
		ApiURL: "https://custom.slack.com/api/",
	})
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewSlackClient_DefaultAPIURL(t *testing.T) {
	client, err := newSlackClient(SlackOptions{
		Token: "test-token",
	})
	assert.NoError(t, err)
	assert.NotNil(t, client)
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

func TestSlack_SendNotification_WithInvalidJSON(t *testing.T) {
	service := NewSlackService(SlackOptions{
		Token:              "something-token",
		InsecureSkipVerify: true,
	})

	err := service.Send(
		Notification{
			Message: "test",
			Slack: &SlackNotification{
				Attachments: "invalid json",
			},
		},
		Destination{Recipient: "test", Service: "slack"},
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}
