package services

import (
	"context"
	"testing"
	"text/template"
	"time"

	"github.com/google/go-github/v69/github"
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

func TestSend_GitHubService_BadURL(t *testing.T) {
	e := gitHubService{}.Send(
		Notification{
			GitHub: &GitHubNotification{
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

func TestGetTemplater_GitHub_Deployment(t *testing.T) {
	f := false
	tr := true
	n := Notification{
		GitHub: &GitHubNotification{
			RepoURLPath:  "{{.sync.spec.git.repo}}",
			RevisionPath: "{{.sync.status.lastSyncedCommit}}",
			Deployment: &GitHubDeployment{
				Reference:            "v0.0.1",
				State:                "success",
				Environment:          "production",
				EnvironmentURL:       "https://argoproj.github.io",
				LogURL:               "https://argoproj.github.io/log",
				RequiredContexts:     []string{},
				AutoMerge:            &f,
				TransientEnvironment: &tr,
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
	assert.Equal(t, &tr, notification.GitHub.Deployment.TransientEnvironment)
	assert.Equal(t, "v0.0.1", notification.GitHub.Deployment.Reference)
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

func TestGetTemplater_Github_PullRequestCommentWithTag(t *testing.T) {
	n := Notification{
		GitHub: &GitHubNotification{
			RepoURLPath:  "{{.sync.spec.git.repo}}",
			RevisionPath: "{{.sync.status.lastSyncedCommit}}",
			PullRequestComment: &GitHubPullRequestComment{
				Content:    "This is a comment",
				CommentTag: "test-tag",
			},
		},
	}
	templater, err := n.GetTemplater("", template.FuncMap{})
	assert.NoError(t, err)

	var notification Notification
	err = templater(&notification, map[string]interface{}{
		"sync": map[string]interface{}{
			"spec": map[string]interface{}{
				"git": map[string]interface{}{
					"repo": "https://github.com/owner/repo",
				},
			},
			"status": map[string]interface{}{
				"lastSyncedCommit": "abc123",
			},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "This is a comment\n<!-- argocd-notifications test-tag -->", notification.GitHub.PullRequestComment.Content)
}

func TestGitHubCheckRunNotification(t *testing.T) {
	checkRun := &GitHubCheckRun{
		Name:        "ArgoCD GitHub Check Run",
		DetailsURL:  "http://example.com/build/status",
		Status:      "completed",
		Conclusion:  "success",
		StartedAt:   time.Now().Format(time.RFC3339),
		CompletedAt: time.Now().Add(5 * time.Minute).Format(time.RFC3339),
		Output: &GitHubCheckRunOutput{
			Title:   "Test Check Run",
			Summary: "All tests passed.",
			Text:    "All unit tests and integration tests passed successfully.",
		},
	}

	githubNotification := &GitHubNotification{
		CheckRun: checkRun,
	}

	vars := map[string]interface{}{
		"app": map[string]interface{}{
			"spec": map[string]interface{}{
				"source": map[string]interface{}{
					"repoURL": "https://github.com/argoproj/argo-cd.git",
				},
			},
			"status": map[string]interface{}{
				"operationState": map[string]interface{}{
					"syncResult": map[string]interface{}{
						"revision": "abc123",
					},
				},
			},
		},
	}

	templater, err := githubNotification.GetTemplater("checkRun", nil)
	assert.NoError(t, err)

	notification := &Notification{}

	err = templater(notification, vars)
	assert.NoError(t, err)

	assert.NotNil(t, notification.GitHub)
	assert.NotNil(t, notification.GitHub.CheckRun)
	assert.Equal(t, "ArgoCD GitHub Check Run", notification.GitHub.CheckRun.Name)
	assert.Equal(t, "completed", notification.GitHub.CheckRun.Status)
	assert.Equal(t, "success", notification.GitHub.CheckRun.Conclusion)
	assert.Equal(t, "All tests passed.", notification.GitHub.CheckRun.Output.Summary)
}

// Mock implementations
type mockIssuesService struct {
	comments []*github.IssueComment
}

type mockPullRequestsService struct {
	prs []*github.PullRequest
}

type mockRepositoriesService struct{}

func (m *mockRepositoriesService) CreateStatus(ctx context.Context, owner, repo, ref string, status *github.RepoStatus) (*github.RepoStatus, *github.Response, error) {
	return status, nil, nil
}

func (m *mockRepositoriesService) ListDeployments(ctx context.Context, owner, repo string, opts *github.DeploymentsListOptions) ([]*github.Deployment, *github.Response, error) {
	return nil, nil, nil
}

func (m *mockRepositoriesService) CreateDeployment(ctx context.Context, owner, repo string, request *github.DeploymentRequest) (*github.Deployment, *github.Response, error) {
	return &github.Deployment{ID: github.Int64(1)}, nil, nil
}

func (m *mockRepositoriesService) CreateDeploymentStatus(ctx context.Context, owner, repo string, deploymentID int64, request *github.DeploymentStatusRequest) (*github.DeploymentStatus, *github.Response, error) {
	return &github.DeploymentStatus{}, nil, nil
}

type mockChecksService struct{}

func (m *mockChecksService) CreateCheckRun(ctx context.Context, owner, repo string, opts github.CreateCheckRunOptions) (*github.CheckRun, *github.Response, error) {
	return &github.CheckRun{}, nil, nil
}

// Mock client implementation
type mockGitHubClientImpl struct {
	issues *mockIssuesService
	prs    *mockPullRequestsService
	repos  *mockRepositoriesService
	checks *mockChecksService
}

func (m *mockGitHubClientImpl) GetIssues() issuesService             { return m.issues }
func (m *mockGitHubClientImpl) GetPullRequests() pullRequestsService { return m.prs }
func (m *mockGitHubClientImpl) GetRepositories() repositoriesService { return m.repos }
func (m *mockGitHubClientImpl) GetChecks() checksService             { return m.checks }

func setupMockServices() (*mockIssuesService, *mockPullRequestsService, githubClient) {
	issues := &mockIssuesService{comments: []*github.IssueComment{}}
	pulls := &mockPullRequestsService{prs: []*github.PullRequest{{Number: github.Int(1)}}}
	client := &mockGitHubClientImpl{
		issues: issues,
		prs:    pulls,
		repos:  &mockRepositoriesService{},
		checks: &mockChecksService{},
	}
	return issues, pulls, client
}

func TestGitHubService_Send_PullRequestCommentWithTag(t *testing.T) {
	issues, _, client := setupMockServices()

	service := &gitHubService{client: client}

	err := service.Send(Notification{
		GitHub: &GitHubNotification{
			repoURL:  "https://github.com/owner/repo",
			revision: "abc123",
			PullRequestComment: &GitHubPullRequestComment{
				Content:    "test comment",
				CommentTag: "test-tag",
			},
		},
	}, Destination{})

	assert.NoError(t, err)
	assert.Len(t, issues.comments, 1)
	assert.Contains(t, *issues.comments[0].Body, "test comment")
	assert.Contains(t, *issues.comments[0].Body, "<!-- argocd-notifications test-tag -->")
}

// Update mock implementation to match the interface from github.go
func (m *mockPullRequestsService) ListPullRequestsWithCommit(ctx context.Context, owner, repo, sha string, opts *github.ListOptions) ([]*github.PullRequest, *github.Response, error) {
	return m.prs, nil, nil
}

// Add these methods back
func (m *mockIssuesService) ListComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, *github.Response, error) {
	return m.comments, nil, nil
}

func (m *mockIssuesService) CreateComment(ctx context.Context, owner, repo string, number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
	m.comments = append(m.comments, comment)
	return comment, nil, nil
}

func (m *mockIssuesService) EditComment(ctx context.Context, owner, repo string, commentID int64, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
	for i, c := range m.comments {
		if c.ID != nil && *c.ID == commentID {
			m.comments[i] = comment
			return comment, nil, nil
		}
	}
	return nil, nil, nil
}

func TestGitHubService_Send_UpdateExistingComment(t *testing.T) {
	issues := &mockIssuesService{
		comments: []*github.IssueComment{
			{
				ID:   github.Int64(1),
				Body: github.String("old comment\n<!-- argocd-notifications test-tag -->"),
			},
		},
	}
	pulls := &mockPullRequestsService{
		prs: []*github.PullRequest{{Number: github.Int(1)}},
	}
	client := &mockGitHubClientImpl{
		issues: issues,
		prs:    pulls,
		repos:  &mockRepositoriesService{},
		checks: &mockChecksService{},
	}

	service := &gitHubService{client: client}

	err := service.Send(Notification{
		GitHub: &GitHubNotification{
			repoURL:  "https://github.com/owner/repo",
			revision: "abc123",
			PullRequestComment: &GitHubPullRequestComment{
				Content:    "updated comment",
				CommentTag: "test-tag",
			},
		},
	}, Destination{})

	assert.NoError(t, err)
	assert.Len(t, issues.comments, 1)
	assert.Equal(t, "updated comment\n<!-- argocd-notifications test-tag -->", *issues.comments[0].Body)
}
