package services

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

func TestGetTemplater_GitLab(t *testing.T) {
	n := Notification{
		GitLab: &GitLabNotification{
			Status: &GitLabStatus{
				State:     "{{.context.state}}",
				Label:     "continuous-delivery/{{.app.metadata.name}}",
				TargetURL: "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
			},
		},
	}
	templater, err := n.GetTemplater("", template.FuncMap{})

	if !assert.NoError(t, err) {
		return
	}

	var notification Notification
	err = templater(&notification, map[string]interface{}{
		"context": map[string]interface{}{
			"argocdUrl": "https://example.com",
			"state":     "success",
		},
		"app": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "argocd-notifications",
			},
			"spec": map[string]interface{}{
				"source": map[string]interface{}{
					"repoURL": "https://gitlab.com/argoproj-labs/argocd-notifications.git",
				},
			},
			"status": map[string]interface{}{
				"operationState": map[string]interface{}{
					"syncResult": map[string]interface{}{
						"revision": "0123456789",
					},
				},
			},
		},
	})

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "https://gitlab.com/argoproj-labs/argocd-notifications.git", notification.GitLab.repoURL)
	assert.Equal(t, "0123456789", notification.GitLab.revision)
	assert.Equal(t, "success", notification.GitLab.Status.State)
	assert.Equal(t, "continuous-delivery/argocd-notifications", notification.GitLab.Status.Label)
	assert.Equal(t, "https://example.com/applications/argocd-notifications", notification.GitLab.Status.TargetURL)
}

func TestGetTemplater_GitLab_Custom_Resource(t *testing.T) {
	n := Notification{
		GitLab: &GitLabNotification{
			RepoURLPath:       "{{.sync.spec.git.repo}}",
			RevisionPath:      "{{.sync.status.lastSyncedCommit}}",
			RevisionIsTagPath: "true",
			Status: &GitLabStatus{
				State: "synced",
				Label: "continuous-delivery/{{.sync.metadata.name}}",
			},
		},
	}
	templater, err := n.GetTemplater("", template.FuncMap{})

	if !assert.NoError(t, err) {
		return
	}

	var notification Notification
	err = templater(&notification, map[string]interface{}{
		"sync": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "root-sync-test",
			},
			"spec": map[string]interface{}{
				"git": map[string]interface{}{
					"repo": "https://gitlab.com/argoproj-labs/argocd-notifications.git",
				},
			},
			"status": map[string]interface{}{
				"lastSyncedCommit": "0123456789",
			},
		},
	})

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "{{.sync.spec.git.repo}}", notification.GitLab.RepoURLPath)
	assert.Equal(t, "{{.sync.status.lastSyncedCommit}}", notification.GitLab.RevisionPath)
	assert.Equal(t, "true", notification.GitLab.RevisionIsTagPath)
	assert.Equal(t, "https://gitlab.com/argoproj-labs/argocd-notifications.git", notification.GitLab.repoURL)
	assert.Equal(t, "0123456789", notification.GitLab.revision)
	assert.Equal(t, "synced", notification.GitLab.Status.State)
	assert.Equal(t, "continuous-delivery/root-sync-test", notification.GitLab.Status.Label)
	assert.Equal(t, "", notification.GitLab.Status.TargetURL)
}

func TestSend_GitLabService_BadURL(t *testing.T) {
	e := gitHubService{}.Send(
		Notification{
			GitLab: &GitLabNotification{
				repoURL: "hello",
			},
		},
		Destination{
			Service:   "",
			Recipient: "",
		},
	)
	assert.ErrorContains(t, e, "does not have a `/`")
}

func TestGetTemplater_GitLab_Deployment(t *testing.T) {
	n := Notification{
		GitLab: &GitLabNotification{
			RepoURLPath:       "{{.sync.spec.git.repo}}",
			RevisionPath:      "{{.sync.status.lastSyncedCommit}}",
			RevisionIsTagPath: "true",
			Deployment: &GitLabDeployment{
				State:          "success",
				Environment:    "production",
				EnvironmentURL: "https://argoproj.gitlab.io",
			},
		},
	}
	templater, err := n.GetTemplater("", template.FuncMap{})

	if !assert.NoError(t, err) {
		return
	}

	var notification Notification
	err = templater(&notification, map[string]interface{}{
		"sync": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "root-sync-test",
			},
			"spec": map[string]interface{}{
				"git": map[string]interface{}{
					"repo": "https://gitlab.com/argoproj-labs/argocd-notifications.git",
				},
			},
			"status": map[string]interface{}{
				"lastSyncedCommit": "0123456789",
			},
		},
	})

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "{{.sync.spec.git.repo}}", notification.GitLab.RepoURLPath)
	assert.Equal(t, "{{.sync.status.lastSyncedCommit}}", notification.GitLab.RevisionPath)
	assert.Equal(t, "true", notification.GitLab.RevisionIsTagPath)
	assert.Equal(t, "https://gitlab.com/argoproj-labs/argocd-notifications.git", notification.GitLab.repoURL)
	assert.Equal(t, "0123456789", notification.GitLab.revision)
	assert.Equal(t, "success", notification.GitLab.Deployment.State)
	assert.Equal(t, "production", notification.GitLab.Deployment.Environment)
	assert.Equal(t, "https://argoproj.gitlab.io", notification.GitLab.Deployment.EnvironmentURL)
}

func TestNewGitLabService_GitLabOptions(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "empty",
			token: "",
		},
		{
			name:  "string",
			token: "123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGitLabService(GitLabOptions{
				Token: tt.token,
			})

			if !assert.NoError(t, err) {
				return
			}
		})
	}
}

func TestGetTemplater_GitLab_MergeRequestComment(t *testing.T) {
	n := Notification{
		GitLab: &GitLabNotification{
			RepoURLPath:       "{{.sync.spec.git.repo}}",
			RevisionPath:      "{{.sync.status.lastSyncedCommit}}",
			RevisionIsTagPath: "true",
			MergeRequestComment: &GitLabMergeRequestComment{
				Content: "This is a comment",
			},
		},
	}
	templater, err := n.GetTemplater("", template.FuncMap{})

	if !assert.NoError(t, err) {
		return
	}

	var notification Notification
	err = templater(&notification, map[string]interface{}{
		"sync": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "root-sync-test",
			},
			"spec": map[string]interface{}{
				"git": map[string]interface{}{
					"repo": "https://gitlab.com/argoproj-labs/argocd-notifications.git",
				},
			},
			"status": map[string]interface{}{
				"lastSyncedCommit": "0123456789",
			},
		},
	})

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "{{.sync.spec.git.repo}}", notification.GitLab.RepoURLPath)
	assert.Equal(t, "{{.sync.status.lastSyncedCommit}}", notification.GitLab.RevisionPath)
	assert.Equal(t, "true", notification.GitLab.RevisionIsTagPath)
	assert.Equal(t, "https://gitlab.com/argoproj-labs/argocd-notifications.git", notification.GitLab.repoURL)
	assert.Equal(t, "0123456789", notification.GitLab.revision)
	assert.Equal(t, "This is a comment", notification.GitLab.MergeRequestComment.Content)
}
