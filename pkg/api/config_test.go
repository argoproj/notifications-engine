package api

import (
	"testing"

	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/argoproj/notifications-engine/pkg/subscriptions"

	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var emptySecret = &corev1.Secret{Data: map[string][]byte{}}

func TestParseConfig_Services(t *testing.T) {
	cfg, err := ParseConfig(&corev1.ConfigMap{Data: map[string]string{
		"service.slack": `
token: my-token
`,
	}}, emptySecret)

	require.NoError(t, err)

	assert.NotNil(t, cfg.Services["slack"])
}

func TestParseConfig_Templates(t *testing.T) {
	cfg, err := ParseConfig(&corev1.ConfigMap{Data: map[string]string{
		"template.my-template": `
message: hello world
`,
	}}, emptySecret)

	require.NoError(t, err)

	assert.Equal(t, map[string]services.Notification{
		"my-template": {Message: "hello world"},
	}, cfg.Templates)
}

func TestParseConfig_DefaultServiceTriggers(t *testing.T) {
	cfg, err := ParseConfig(&corev1.ConfigMap{Data: map[string]string{
		"defaultTriggers.slack": `
- trigger-a
- trigger-b
`,
	}}, emptySecret)

	require.NoError(t, err)

	assert.Equal(t, map[string][]string{
		"slack": {
			"trigger-a",
			"trigger-b",
		},
	}, cfg.ServiceDefaultTriggers)
}

func TestReplaceStringSecret_KeyPresent(t *testing.T) {
	val := replaceStringSecret("hello $secret-value", map[string][]byte{
		"secret-value": []byte("world"),
	})

	assert.Equal(t, "hello world", val)
}

func TestReplaceStringSecret_KeyMissing(t *testing.T) {
	val := replaceStringSecret("hello $secret-value", map[string][]byte{
		"another-secret-value": []byte("world"),
	})

	assert.Equal(t, "hello $secret-value", val)
}

func TestReplaceServiceConfigSecrets_WithBasicWebhook_ReplacesSecrets(t *testing.T) {
	input := `url: $endpoint
headers:
  - name: Authorization
    value: Bearer $secret-value
`

	secrets := corev1.Secret{
		Data: map[string][]byte{
			"endpoint":     []byte("https://example.com"),
			"secret-value": []byte("token"),
		},
	}

	expected := `url: https://example.com
headers:
    - name: Authorization
      value: Bearer token
`

	result, err := replaceServiceConfigSecrets(input, &secrets)

	require.NoError(t, err)
	assert.Equal(t, expected, string(result))
}

func TestReplaceServiceConfigSecrets_WithMapOfSecrets_ReplacesSecrets(t *testing.T) {
	input := `apiUrl: $api-url
apiKeys:
  first-team: $first-team-secret
  second-team: $second-team-secret
`

	secrets := corev1.Secret{
		Data: map[string][]byte{
			"first-team-secret":  []byte("first-token"),
			"second-team-secret": []byte("second-token"),
		},
	}

	expected := `apiUrl: $api-url
apiKeys:
    first-team: first-token
    second-team: second-token
`

	result, err := replaceServiceConfigSecrets(input, &secrets)

	require.NoError(t, err)
	assert.Equal(t, expected, string(result))
}

func TestReplaceServiceConfigSecrets_WithMultilineSecret_ReplacesSecrets(t *testing.T) {
	input := `appID: 12345
privateKey: $github-privateKey
installationID: 67890
`

	secrets := corev1.Secret{
		Data: map[string][]byte{
			"github-privateKey": []byte("A\nValue\nOn\nMultiple\nLines"),
		},
	}

	expected := `appID: 12345
privateKey: |-
    A
    Value
    On
    Multiple
    Lines
installationID: 67890
`

	result, err := replaceServiceConfigSecrets(input, &secrets)

	require.NoError(t, err)
	assert.Equal(t, expected, string(result))
}

func TestParseConfig_DefaultTriggers(t *testing.T) {
	cfg, err := ParseConfig(&corev1.ConfigMap{
		Data: map[string]string{
			"defaultTriggers": `[trigger1, trigger2]`,
		},
	}, emptySecret)

	require.NoError(t, err)
	assert.Equal(t, []string{"trigger1", "trigger2"}, cfg.DefaultTriggers)
}

func TestParseConfig_Subscriptions(t *testing.T) {
	cfg, err := ParseConfig(&corev1.ConfigMap{
		Data: map[string]string{
			"subscriptions": `
- selector: test=true
  triggers:
  - my-trigger2`,
		},
	}, emptySecret)

	require.NoError(t, err)

	label, err := labels.Parse("test=true")
	require.NoError(t, err)
	assert.Equal(t, subscriptions.DefaultSubscriptions([]subscriptions.DefaultSubscription{
		{Triggers: []string{"my-trigger2"}, Selector: label},
	}), cfg.Subscriptions)
}

func TestParseConfig_RetiredTeams(t *testing.T) {
	for _, key := range []string{"service.teams", "service.teams.custom", "service.teams.shared"} {
		t.Run(key, func(t *testing.T) {
			cfg, err := ParseConfig(&corev1.ConfigMap{Data: map[string]string{
				key:                              "recipientUrls: {}",
				"service.teams-workflows.shared": "recipientUrls: {}",
				"service.webhook":                "url: https://example.com",
				"template.mixed": `message: hello
teams:
  title: '{{obsoleteFunction}}'
teams-workflows:
  title: hello
`,
			}}, emptySecret)
			require.NoError(t, err)
			require.Len(t, cfg.Services, 2)
			require.Contains(t, cfg.Services, "shared")
			require.Contains(t, cfg.Services, "webhook")
			instance, err := NewAPI(*cfg, nil)
			require.NoError(t, err)
			assert.Len(t, instance.GetNotificationServices(), 2)
			require.ErrorContains(t, instance.Send(nil, []string{"mixed"}, services.Destination{Service: "teams"}), "not supported")
		})
	}
}

func TestParseConfig_RetiredTeamsPreservesValidation(t *testing.T) {
	_, err := ParseConfig(&corev1.ConfigMap{Data: map[string]string{
		"service.teams": "recipientUrls: {}",
		// An undefined YAML alias must still be rejected for supported services.
		"service.webhook": "url: *undefined",
	}}, emptySecret)
	require.ErrorContains(t, err, "webhook")
}

func TestParseConfig_RetiredTeamsTemplateWarning(t *testing.T) {
	oldHooks := log.StandardLogger().ReplaceHooks(make(log.LevelHooks))
	t.Cleanup(func() { log.StandardLogger().ReplaceHooks(oldHooks) })
	hook := logtest.NewGlobal()
	for _, retired := range []string{`teams:
  title: '{{obsoleteFunction}}'
`, "teams: null\n", "teams: ignored\n"} {
		t.Run(retired, func(t *testing.T) {
			hook.Reset()
			cfg, err := ParseConfig(&corev1.ConfigMap{Data: map[string]string{
				"template.mixed": retired + `message: hello
teams-workflows:
  title: Supported
`,
			}}, emptySecret)
			require.NoError(t, err)
			entry := hook.LastEntry()
			require.NotNil(t, entry)
			assert.Equal(t, log.WarnLevel, entry.Level)
			assert.Contains(t, entry.Message, "template.mixed")
			assert.Contains(t, entry.Message, "teams-workflows")
			_, err = NewAPI(*cfg, nil)
			require.NoError(t, err)
			require.NotNil(t, cfg.Templates["mixed"].TeamsWorkflows)
			assert.Equal(t, "Supported", cfg.Templates["mixed"].TeamsWorkflows.Title)
		})
	}
	hook.Reset()
	_, err := ParseConfig(&corev1.ConfigMap{Data: map[string]string{
		"template.supported": `message: hello
teams-workflows:
  title: Supported
`,
	}}, emptySecret)
	require.NoError(t, err)
	assert.Empty(t, hook.AllEntries())
}
