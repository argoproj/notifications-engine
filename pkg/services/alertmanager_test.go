package services

import (
	"fmt"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTemplater_Alertmanager(t *testing.T) {
	n := Notification{
		Alertmanager: &AlertmanagerNotification{
			Labels: map[string]string{
				"alertname": "App_Deployed",
			},
			Annotations: map[string]string{
				"appname": "{{.app.metadata.name}}",
			},
		},
	}

	vars := map[string]any{
		"app": map[string]any{
			"metadata": map[string]any{
				"name": "argocd-notifications",
			},
			"spec": map[string]any{
				"source": map[string]any{
					"repoURL": "https://github.com/argoproj-labs/argocd-notifications.git",
				},
			},
		},
	}

	t.Run("test_Labels_Annotations", func(t *testing.T) {
		templater, err := n.GetTemplater("", template.FuncMap{})
		require.NoError(t, err)

		var notification Notification
		err = templater(&notification, vars)
		require.NoError(t, err)

		assert.Equal(t, "App_Deployed", notification.Alertmanager.Labels["alertname"])
		assert.Equal(t, "argocd-notifications", notification.Alertmanager.Annotations["appname"])
	})

	t.Run("test_default_GeneratorURL", func(t *testing.T) {
		templater, err := n.GetTemplater("", template.FuncMap{})
		require.NoError(t, err)

		var notification Notification
		err = templater(&notification, vars)
		require.NoError(t, err)

		assert.Equal(t, "https://github.com/argoproj-labs/argocd-notifications.git", notification.Alertmanager.GeneratorURL)
	})

	t.Run("test_custom_GeneratorURL", func(t *testing.T) {
		n.Alertmanager.GeneratorURL = "{{.app.metadata.name}}"

		templater, err := n.GetTemplater("", template.FuncMap{})
		require.NoError(t, err)

		var notification Notification
		_ = templater(&notification, map[string]any{
			"app": map[string]any{
				"metadata": map[string]any{
					"name": "argocd-notifications",
				},
				"spec": map[string]any{
					"source": map[string]any{
						"repoURL": "https://github.com/argoproj-labs/argocd-notifications.git",
					},
				},
			},
		})

		assert.Equal(t, "argocd-notifications", notification.Alertmanager.GeneratorURL)
	})

	t.Run("test_git_format_GeneratorURL", func(t *testing.T) {
		n.Alertmanager.GeneratorURL = "{{.app.spec.source.repoURL}}"

		templater, err := n.GetTemplater("", template.FuncMap{})
		require.NoError(t, err)

		var notification Notification
		_ = templater(&notification, map[string]any{
			"app": map[string]any{
				"metadata": map[string]any{
					"name": "argocd-notifications",
				},
				"spec": map[string]any{
					"source": map[string]any{
						"repoURL": "git@github.com:argoproj-labs/argocd-notifications.git",
					},
				},
			},
		})

		assert.Equal(t, "https://github.com/argoproj-labs/argocd-notifications.git", notification.Alertmanager.GeneratorURL)
	})
}

func TestSend_AlertmanagerCluster(t *testing.T) {
	opt := AlertmanagerOptions{
		Targets: []string{
			"127.0.0.1:19093",
			"127.0.0.1:29093",
			"127.0.0.1:39093",
		},
		Scheme:  "http",
		APIPath: "/api/v2/alerts",
		BasicAuth: &BasicAuth{
			Username: "user",
			Password: "pass",
		},
		Timeout: 2,
	}

	notification := Notification{
		Alertmanager: &AlertmanagerNotification{
			Labels: map[string]string{
				"alertname": "TestSend",
			},
		},
	}

	s := NewAlertmanagerService(opt)
	if err := s.Send(notification, Destination{}); err != nil {
		assert.EqualError(t, err, "no events were successfully received by alertmanager")
	}
}

func Test_AlertManagerNotExist(t *testing.T) {
	n := Notification{}
	svc := NewAlertmanagerService(AlertmanagerOptions{})
	err := svc.Send(n, Destination{})
	assert.EqualError(t, err, "notification alertmanager no config")
}

func Test_AlertManagerNoLabels(t *testing.T) {
	n := Notification{
		Alertmanager: &AlertmanagerNotification{},
	}
	svc := NewAlertmanagerService(AlertmanagerOptions{})
	err := svc.Send(n, Destination{})
	assert.EqualError(t, err, "alertmanager at least one label pair required")
}

func Test_AlertManagerReusableTemplater(t *testing.T) {
	n := Notification{
		Alertmanager: &AlertmanagerNotification{
			Labels: map[string]string{
				"alertname": "App_Deployed",
				"appname":   "{{.app.metadata.name}}",
			},
			Annotations: map[string]string{
				"appname": "{{.app.metadata.name}}",
			},
		},
	}

	templater, err := n.GetTemplater("", template.FuncMap{})
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		name := fmt.Sprintf("argocd-notifications-%d", i)
		var notification Notification
		err = templater(&notification, map[string]any{
			"app": map[string]any{
				"metadata": map[string]any{
					"name": name,
				},
				"spec": map[string]any{
					"source": map[string]any{
						"repoURL": "https://github.com/argoproj-labs/argocd-notifications.git",
					},
				},
			},
		})
		require.NoError(t, err)

		assert.Equal(t, "App_Deployed", notification.Alertmanager.Labels["alertname"])
		assert.Equal(t, name, notification.Alertmanager.Labels["appname"])
		assert.Equal(t, name, notification.Alertmanager.Annotations["appname"])
	}
}
