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
			GeneratorURL: "{{.app.spec.source.repoURL}}",
		},
	}
	templater, err := n.GetTemplater("", template.FuncMap{})

	if !assert.NoError(t, err) {
		return
	}

	var notification Notification
	err = templater(&notification, map[string]interface{}{
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

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "App_Deployed", notification.Alertmanager.Labels["alertname"])
	assert.Equal(t, "https://github.com/argoproj-labs/argocd-notifications.git", notification.Alertmanager.GeneratorURL)
	assert.Equal(t, "argocd-notifications", notification.Alertmanager.Annotations["appname"])
}
