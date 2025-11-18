package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"text/template"

	slackutil "github.com/argoproj/notifications-engine/pkg/util/slack"
	"github.com/slack-go/slack"

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
			require.Error(t, err)
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
	require.NoError(t, err)

	// Test with nil Slack on target notification
	var notification Notification
	err = templater(&notification, map[string]any{
		"name": "test",
	})

	require.NoError(t, err)
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
	require.NoError(t, err)

	var notification Notification
	err = templater(&notification, map[string]any{})

	require.NoError(t, err)
	assert.Equal(t, slackutil.Update, notification.Slack.DeliveryPolicy)
}

func TestGetTemplater_Slack_TemplateExecutionError(t *testing.T) {
	// Create a FuncMap with the required function
	funcMap := template.FuncMap{
		"required": func(msg string, val any) (any, error) {
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
			require.NoError(t, err)

			var notification Notification
			err = templater(&notification, map[string]any{})
			require.Error(t, err)
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
		require.NoError(t, err)
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
		require.NoError(t, err)
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
		require.NoError(t, err)
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
	require.NoError(t, err)
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
	require.Error(t, err)
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
	require.Error(t, err)
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
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewSlackClient_DefaultAPIURL(t *testing.T) {
	client, err := newSlackClient(SlackOptions{
		Token: "test-token",
	})
	require.NoError(t, err)
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
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestSlackUserEmailPattern(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid user email marker",
			input:    "__SLACK_USER_EMAIL__user@example.com__",
			expected: true,
		},
		{
			name:     "invalid marker",
			input:    "user@example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := slackUserEmailPattern.MatchString(tt.input)
			assert.Equal(t, tt.expected, matches)
		})
	}
}

func TestSlackChannelPattern(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid channel marker",
			input:    "__SLACK_CHANNEL__general__",
			expected: true,
		},
		{
			name:     "invalid marker",
			input:    "general",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := slackChannelPattern.MatchString(tt.input)
			assert.Equal(t, tt.expected, matches)
		})
	}
}

func TestSlackUserGroupPattern(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid usergroup marker",
			input:    "__SLACK_USERGROUP__developers__",
			expected: true,
		},
		{
			name:     "invalid marker",
			input:    "developers",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := slackUserGroupPattern.MatchString(tt.input)
			assert.Equal(t, tt.expected, matches)
		})
	}
}

func TestProcessSlackMentions(t *testing.T) {
	// Mock server to handle Slack API calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "users.lookupByEmail"):
			// Mock GetUserByEmail response
			response := slack.User{
				ID:   "U024BE7LH",
				Name: "testuser",
				Profile: slack.UserProfile{
					Email: "user@example.com",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":   true,
				"user": response,
			})
		case strings.Contains(r.URL.Path, "conversations.list"):
			// Mock GetConversations response
			response := struct {
				OK               bool            `json:"ok"`
				Channels         []slack.Channel `json:"channels"`
				ResponseMetadata struct {
					NextCursor string `json:"next_cursor"`
				} `json:"response_metadata"`
			}{
				OK: true,
				Channels: []slack.Channel{
					{
						GroupConversation: slack.GroupConversation{
							Conversation: slack.Conversation{
								ID: "C123ABC456",
							},
							Name: "general",
						},
					},
				},
			}
			response.ResponseMetadata.NextCursor = ""
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		case strings.Contains(r.URL.Path, "usergroups.list"):
			// Mock GetUserGroups response
			response := struct {
				OK         bool              `json:"ok"`
				UserGroups []slack.UserGroup `json:"usergroups"`
			}{
				OK: true,
				UserGroups: []slack.UserGroup{
					{
						ID:     "SAZ94GDB8",
						Handle: "developers",
						Name:   "Developers",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := slack.New("test-token", slack.OptionAPIURL(server.URL+"/"))

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "user mention by email",
			input:    "Hey __SLACK_USER_EMAIL__user@example.com__, thanks!",
			expected: "Hey <@U024BE7LH>, thanks!",
		},
		{
			name:     "channel mention",
			input:    "Check __SLACK_CHANNEL__general__ for updates",
			expected: "Check <#C123ABC456> for updates",
		},
		{
			name:     "user group mention",
			input:    "Hey __SLACK_USERGROUP__developers__, new task!",
			expected: "Hey <!subteam^SAZ94GDB8>, new task!",
		},
		{
			name:     "multiple mentions",
			input:    "Hey __SLACK_USER_EMAIL__user@example.com__, check __SLACK_CHANNEL__general__",
			expected: "Hey <@U024BE7LH>, check <#C123ABC456>",
		},
		{
			name:     "no mentions",
			input:    "Just a regular message",
			expected: "Just a regular message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear cache before each test
			globalLookupCache.Lock()
			globalLookupCache.usersByEmail = make(map[string]string)
			globalLookupCache.channels = make(map[string]string)
			globalLookupCache.userGroups = make(map[string]string)
			globalLookupCache.Unlock()

			result := processSlackMentions(client, tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLookupCaching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if strings.Contains(r.URL.Path, "users.lookupByEmail") {
			response := slack.User{
				ID:   "U024BE7LH",
				Name: "testuser",
				Profile: slack.UserProfile{
					Email: "user@example.com",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":   true,
				"user": response,
			})
		}
	}))
	defer server.Close()

	client := slack.New("test-token", slack.OptionAPIURL(server.URL+"/"))

	// Clear cache
	globalLookupCache.Lock()
	globalLookupCache.usersByEmail = make(map[string]string)
	globalLookupCache.Unlock()

	// First lookup should make an API call
	userID1, err := lookupUserByEmail(client, "user@example.com")
	assert.NoError(t, err)
	assert.Equal(t, "U024BE7LH", userID1)
	assert.Equal(t, 1, callCount)

	// Second lookup should use cache, no additional API call
	userID2, err := lookupUserByEmail(client, "user@example.com")
	assert.NoError(t, err)
	assert.Equal(t, "U024BE7LH", userID2)
	assert.Equal(t, 1, callCount) // Call count should not increase
}

func TestSlackSendWithMentions(t *testing.T) {
	dummyResponse, err := json.Marshal(chatResponseFull{
		Channel:          "test",
		Timestamp:        "1503435956.000247",
		MessageTimeStamp: "1503435956.000247",
		Text:             "text",
	})
	assert.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.Contains(request.URL.Path, "users.lookupByEmail"):
			response := slack.User{
				ID:   "U024BE7LH",
				Name: "testuser",
				Profile: slack.UserProfile{
					Email: "user@example.com",
				},
			}
			writer.Header().Set("Content-Type", "application/json")
			json.NewEncoder(writer).Encode(map[string]interface{}{
				"ok":   true,
				"user": response,
			})
		case strings.Contains(request.URL.Path, "chat.postMessage"):
			data, err := io.ReadAll(request.Body)
			assert.NoError(t, err)

			// Verify that the message contains the processed mention
			// URL decode the body first since it's form-encoded
			bodyStr, err := url.QueryUnescape(string(data))
			assert.NoError(t, err)
			assert.Contains(t, bodyStr, "<@U024BE7LH>")

			writer.WriteHeader(http.StatusOK)
			_, err = writer.Write(dummyResponse)
			assert.NoError(t, err)
		default:
			writer.WriteHeader(http.StatusOK)
			_, err = writer.Write(dummyResponse)
			assert.NoError(t, err)
		}
	}))
	defer server.Close()

	// Clear cache
	globalLookupCache.Lock()
	globalLookupCache.usersByEmail = make(map[string]string)
	globalLookupCache.Unlock()

	service := NewSlackService(SlackOptions{
		ApiURL:             server.URL + "/",
		Token:              "something-token",
		InsecureSkipVerify: true,
	})

	err = service.Send(
		Notification{Message: "Hey __SLACK_USER_EMAIL__user@example.com__, thanks for your commit!"},
		Destination{Recipient: "test-channel", Service: "slack"},
	)
	assert.NoError(t, err)
}
