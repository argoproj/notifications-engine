package services

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

func TestGetTemplater_GitHub(t *testing.T) {
	n := Notification{
		GitHub: &GitHubNotification{
			Status: &GitHubStatus{
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
					"repoURL": "https://github.com/argoproj-labs/argocd-notifications.git",
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

	assert.Equal(t, "https://github.com/argoproj-labs/argocd-notifications.git", notification.GitHub.repoURL)
	assert.Equal(t, "0123456789", notification.GitHub.revision)
	assert.Equal(t, "success", notification.GitHub.Status.State)
	assert.Equal(t, "continuous-delivery/argocd-notifications", notification.GitHub.Status.Label)
	assert.Equal(t, "https://example.com/applications/argocd-notifications", notification.GitHub.Status.TargetURL)
}

func TestGetTemplater_GitHub_Custom_Resource(t *testing.T) {
	n := Notification{
		GitHub: &GitHubNotification{
			RepoURLPath:  "{{.sync.spec.git.repo}}",
			RevisionPath: "{{.sync.status.lastSyncedCommit}}",
			Status: &GitHubStatus{
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
					"repo": "https://github.com/argoproj-labs/argocd-notifications.git",
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

	assert.Equal(t, "{{.sync.spec.git.repo}}", notification.GitHub.RepoURLPath)
	assert.Equal(t, "{{.sync.status.lastSyncedCommit}}", notification.GitHub.RevisionPath)
	assert.Equal(t, "https://github.com/argoproj-labs/argocd-notifications.git", notification.GitHub.repoURL)
	assert.Equal(t, "0123456789", notification.GitHub.revision)
	assert.Equal(t, "synced", notification.GitHub.Status.State)
	assert.Equal(t, "continuous-delivery/root-sync-test", notification.GitHub.Status.Label)
	assert.Equal(t, "", notification.GitHub.Status.TargetURL)
}

func TestGetTemplater_GitHub_Deployment(t *testing.T) {
	f := false
	n := Notification{
		GitHub: &GitHubNotification{
			RepoURLPath:  "{{.sync.spec.git.repo}}",
			RevisionPath: "{{.sync.status.lastSyncedCommit}}",
			Deployment: &GitHubDeployment{
				State:            "success",
				Environment:      "production",
				EnvironmentURL:   "https://argoproj.github.io",
				LogURL:           "https://argoproj.github.io/log",
				RequiredContexts: []string{},
				AutoMerge:        &f,
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
					"repo": "https://github.com/argoproj-labs/argocd-notifications.git",
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

	assert.Equal(t, "{{.sync.spec.git.repo}}", notification.GitHub.RepoURLPath)
	assert.Equal(t, "{{.sync.status.lastSyncedCommit}}", notification.GitHub.RevisionPath)
	assert.Equal(t, "https://github.com/argoproj-labs/argocd-notifications.git", notification.GitHub.repoURL)
	assert.Equal(t, "0123456789", notification.GitHub.revision)
	assert.Equal(t, "success", notification.GitHub.Deployment.State)
	assert.Equal(t, "production", notification.GitHub.Deployment.Environment)
	assert.Equal(t, "https://argoproj.github.io", notification.GitHub.Deployment.EnvironmentURL)
	assert.Equal(t, "https://argoproj.github.io/log", notification.GitHub.Deployment.LogURL)
	assert.Len(t, notification.GitHub.Deployment.RequiredContexts, 0)
	assert.Equal(t, &f, notification.GitHub.Deployment.AutoMerge)
}

func TestNewGitHubService_GitHubOptions(t *testing.T) {
	tests := []struct {
		name                  string
		appID, installationID interface{}
	}{
		{
			name:           "nil",
			appID:          nil,
			installationID: nil,
		},
		{
			name:           "int",
			appID:          123456789,
			installationID: 123456789,
		},
		{
			name:           "string",
			appID:          "123456789",
			installationID: "123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGitHubService(GitHubOptions{
				AppID:          tt.appID,
				InstallationID: tt.installationID,
				PrivateKey:     "-----BEGIN RSA PRIVATE KEY-----\nMIICWgIBAAKBgFPm23ojxbC1wC8X73f3aE9JEUrNEGuuj9TXscgp8HEqCHEOSh2/\nlwiPckhcdxnvu23uHGL4jwSHJe5jj4IgOUDjl/KSplJFuZYYfegQYjsOR512s4zn\nNVFsstfCNH6w7SQKsT5jVe3WPsCCuVyCZMTgEpJF2cQ7VNDYMT6hZn0NAgMBAAEC\ngYAVL7V6STAxaCPIgI3KyGHBq5y/O7sKxgCx6WmONvDtUoThL4+NpYSY98gO97Jn\njT7SCo+Gemd66Dmu0ds6K7LpIsqdGOJwp/YxgGBSxAjhL1qFHnOjhPgzE80c0aMB\ngFUnfqrxl7OqpUisrQP8K4XOPzRC/ukhI4YPG23zRi9l4QJBAJPeuqu5P0Aiy8TV\nsyxNSEaLp5QSjhrV41ooF/7Yb41crGoDPHwT5dIKi9jLMpzERY2wtL0SomNN1Bv8\nBOJIHHkCQQCRQUWVqHHLpMhmWLk9b/8q3ZFZ7mNinTaZjekhtYP1CcuFG1b9UCJE\nuJeEUH+ijYUrRKv/y8mkzkB7l5VaZ1g1AkBmxhFcNV6+xvB1mEn16qjnTz1j7xmR\nkUN5cBBtciTmTZkP/bvWSUYcnHPidChzSP9GoaCdIQx4lKlt4dXLKG+RAkBoXNxR\nFdCE/2UY2+Bj+wb71mvrkHMJ1Gj5VNPO62re8OWwQh9zK1MjyvjaEThTI5ktqE5o\nIBRF/AaqhhPB+4SNAkBko5ygyfmdooxEeM2PSCIcL/Jjs8ogOk+kPYMRtVKzdaGU\naDbUQ7GRzo2mJEuq4pGhkAh3b00Zc5Eapy5EFQlu\n-----END RSA PRIVATE KEY-----",
			})

			if !assert.NoError(t, err) {
				return
			}
		})
	}
}

func TestGetTemplater_Github_PullRequestComment(t *testing.T) {
	n := Notification{
		GitHub: &GitHubNotification{
			RepoURLPath:  "{{.sync.spec.git.repo}}",
			RevisionPath: "{{.sync.status.lastSyncedCommit}}",
			PullRequestComment: &GitHubPullRequestComment{
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
					"repo": "https://github.com/argoproj-labs/argocd-notifications.git",
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

	assert.Equal(t, "{{.sync.spec.git.repo}}", notification.GitHub.RepoURLPath)
	assert.Equal(t, "{{.sync.status.lastSyncedCommit}}", notification.GitHub.RevisionPath)
	assert.Equal(t, "https://github.com/argoproj-labs/argocd-notifications.git", notification.GitHub.repoURL)
	assert.Equal(t, "0123456789", notification.GitHub.revision)
	assert.Equal(t, "This is a comment", notification.GitHub.PullRequestComment.Content)
}

func TestGetTemplater_Github_CheckRun(t *testing.T) {
	n := Notification{
		CheckRun: &GitHubCheckRun{
			Output:  &CheckRunOutput {
               Title: "{{.sync.status.lastSyncedCommit}}"
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
					"repo": "https://github.com/argoproj-labs/argocd-notifications.git",
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

	assert.Equal(t, "{{.sync.spec.git.repo}}", notification.GitHub.RepoURLPath)
	assert.Equal(t, "{{.sync.status.lastSyncedCommit}}", notification.GitHub.RevisionPath)
	assert.Equal(t, "https://github.com/argoproj-labs/argocd-notifications.git", notification.GitHub.repoURL)
	assert.Equal(t, "0123456789", notification.GitHub.revision)
	assert.Equal(t, "0123456789", notification.GitHub.CheckRun.Output.Title)
}