package services

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
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

	vars := map[string]interface{}{
		"app": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "argocd-notifications",
			},
			"spec": map[string]interface{}{
				"source": map[string]interface{}{
					"repoURL": "https://github.com/argoproj-labs/argocd-notifications.git",
				},
			},
		},
	}

	t.Run("test_Labels_Annotations", func(t *testing.T) {
		templater, err := n.GetTemplater("", template.FuncMap{})
		if !assert.NoError(t, err) {
			return
		}

		var notification Notification
		err = templater(&notification, vars)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "App_Deployed", notification.Alertmanager.Labels["alertname"])
		assert.Equal(t, "argocd-notifications", notification.Alertmanager.Annotations["appname"])
	})

	t.Run("test_default_GeneratorURL", func(t *testing.T) {
		templater, err := n.GetTemplater("", template.FuncMap{})
		if !assert.NoError(t, err) {
			return
		}

		var notification Notification
		err = templater(&notification, vars)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "https://github.com/argoproj-labs/argocd-notifications.git", notification.Alertmanager.GeneratorURL)
	})

	t.Run("test_custom_GeneratorURL", func(t *testing.T) {
		n.Alertmanager.GeneratorURL = "{{.app.metadata.name}}"

		templater, err := n.GetTemplater("", template.FuncMap{})
		if !assert.NoError(t, err) {
			return
		}

		var notification Notification
		_ = templater(&notification, map[string]interface{}{
			"app": map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "argocd-notifications",
				},
				"spec": map[string]interface{}{
					"source": map[string]interface{}{
						"repoURL": "https://github.com/argoproj-labs/argocd-notifications.git",
					},
				},
			},
		})

		assert.Equal(t, "argocd-notifications", notification.Alertmanager.GeneratorURL)
	})
}
