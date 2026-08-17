package services

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gitLabTestVars() map[string]any {
	return map[string]any{
		"context": map[string]any{
			"argocdUrl": "https://example.com",
			"state":     "success",
		},
		"app": map[string]any{
			"metadata": map[string]any{
				"name": "argocd-notifications",
			},
			"spec": map[string]any{
				"source": map[string]any{
					"repoURL": "https://gitlab.com/argoproj-labs/argocd-notifications.git",
				},
			},
			"status": map[string]any{
				"operationState": map[string]any{
					"syncResult": map[string]any{
						"revision": "0123456789",
					},
				},
			},
		},
	}
}

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
	require.NoError(t, err)

	var notification Notification
	err = templater(&notification, gitLabTestVars())
	require.NoError(t, err)

	assert.Equal(t, "https://gitlab.com/argoproj-labs/argocd-notifications.git", notification.GitLab.repoURL)
	assert.Equal(t, "0123456789", notification.GitLab.revision)
	assert.Equal(t, "success", notification.GitLab.Status.State)
	assert.Equal(t, "continuous-delivery/argocd-notifications", notification.GitLab.Status.Label)
	assert.Equal(t, "https://example.com/applications/argocd-notifications", notification.GitLab.Status.TargetURL)
}

func TestGetTemplater_GitLab_Deployment(t *testing.T) {
	tag := true
	n := Notification{
		GitLab: &GitLabNotification{
			Deployment: &GitLabDeployment{
				State:       "{{.context.state}}",
				Environment: "production",
				Reference:   "v1.0.0",
				Tag:         &tag,
			},
		},
	}
	templater, err := n.GetTemplater("", template.FuncMap{})
	require.NoError(t, err)

	var notification Notification
	err = templater(&notification, gitLabTestVars())
	require.NoError(t, err)

	assert.Equal(t, "success", notification.GitLab.Deployment.State)
	assert.Equal(t, "production", notification.GitLab.Deployment.Environment)
	assert.Equal(t, "v1.0.0", notification.GitLab.Deployment.Reference)
	assert.Equal(t, &tag, notification.GitLab.Deployment.Tag)
}

func TestGetTemplater_GitLab_MergeRequestComment(t *testing.T) {
	n := Notification{
		GitLab: &GitLabNotification{
			RepoURLPath:  "{{.sync.spec.git.repo}}",
			RevisionPath: "{{.sync.status.lastSyncedCommit}}",
			MergeRequestComment: &GitLabMergeRequestComment{
				Content: "Application {{.sync.metadata.name}} is now running",
			},
		},
	}
	templater, err := n.GetTemplater("", template.FuncMap{})
	require.NoError(t, err)

	var notification Notification
	err = templater(&notification, map[string]any{
		"sync": map[string]any{
			"metadata": map[string]any{
				"name": "root-sync-test",
			},
			"spec": map[string]any{
				"git": map[string]any{
					"repo": "https://gitlab.com/argoproj-labs/argocd-notifications.git",
				},
			},
			"status": map[string]any{
				"lastSyncedCommit": "0123456789",
			},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "{{.sync.spec.git.repo}}", notification.GitLab.RepoURLPath)
	assert.Equal(t, "{{.sync.status.lastSyncedCommit}}", notification.GitLab.RevisionPath)
	assert.Equal(t, "https://gitlab.com/argoproj-labs/argocd-notifications.git", notification.GitLab.repoURL)
	assert.Equal(t, "0123456789", notification.GitLab.revision)
	assert.Equal(t, "Application root-sync-test is now running", notification.GitLab.MergeRequestComment.Content)
}

func TestProjectPathByRepoURL_GitLab(t *testing.T) {
	tests := []struct {
		name    string
		repoURL string
		want    string
		wantErr bool
	}{
		{name: "https", repoURL: "https://gitlab.com/argoproj-labs/argocd-notifications.git", want: "argoproj-labs/argocd-notifications"},
		{name: "ssh", repoURL: "git@gitlab.com:argoproj-labs/argocd-notifications.git", want: "argoproj-labs/argocd-notifications"},
		{name: "subgroups", repoURL: "https://gitlab.com/group/subgroup/nested/argocd-notifications.git", want: "group/subgroup/nested/argocd-notifications"},
		{name: "no namespace", repoURL: "https://gitlab.com/argocd-notifications.git", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := projectPathByRepoURL(tt.repoURL)
			if tt.wantErr {
				require.ErrorContains(t, err, "is not a project path")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSend_GitLab_EmptyConfig(t *testing.T) {
	err := gitLabService{}.Send(Notification{}, Destination{})
	require.ErrorContains(t, err, "config is empty")
}

func TestSend_GitLab_Status(t *testing.T) {
	var path string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path = request.URL.EscapedPath()
		data, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &body))
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"id": 1}`))
	}))
	defer server.Close()

	service, err := NewGitLabService(GitLabOptions{BaseURL: server.URL, Token: "token"})
	require.NoError(t, err)

	err = service.Send(Notification{
		Message: "Application is now running",
		GitLab: &GitLabNotification{
			repoURL:  "https://gitlab.com/group/subgroup/argocd-notifications.git",
			revision: "0123456789",
			Status: &GitLabStatus{
				State:     "success",
				Label:     "continuous-delivery/argocd-notifications",
				TargetURL: "https://example.com/applications/argocd-notifications",
			},
		},
	}, Destination{})
	require.NoError(t, err)

	assert.Equal(t, "/api/v4/projects/group%2Fsubgroup%2Fargocd-notifications/statuses/0123456789", path)
	assert.Equal(t, "success", body["state"])
	assert.Equal(t, "continuous-delivery/argocd-notifications", body["name"])
	assert.Equal(t, "https://example.com/applications/argocd-notifications", body["target_url"])
	assert.Equal(t, "Application is now running", body["description"])
}

func TestSend_GitLab_Deployment(t *testing.T) {
	var path string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path = request.URL.EscapedPath()
		data, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &body))
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"id": 1}`))
	}))
	defer server.Close()

	service, err := NewGitLabService(GitLabOptions{BaseURL: server.URL, Token: "token"})
	require.NoError(t, err)

	err = service.Send(Notification{
		GitLab: &GitLabNotification{
			repoURL:  "https://gitlab.com/argoproj-labs/argocd-notifications.git",
			revision: "0123456789",
			Deployment: &GitLabDeployment{
				State:       "success",
				Environment: "production",
			},
		},
	}, Destination{})
	require.NoError(t, err)

	assert.Equal(t, "/api/v4/projects/argoproj-labs%2Fargocd-notifications/deployments", path)
	assert.Equal(t, map[string]any{
		"environment": "production",
		"ref":         "0123456789",
		"sha":         "0123456789",
		"status":      "success",
		"tag":         false,
	}, body)
}

func TestSend_GitLab_Deployment_TagAndReference(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &body))
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"id": 1}`))
	}))
	defer server.Close()

	service, err := NewGitLabService(GitLabOptions{BaseURL: server.URL, Token: "token"})
	require.NoError(t, err)

	tag := true
	err = service.Send(Notification{
		GitLab: &GitLabNotification{
			repoURL:  "https://gitlab.com/argoproj-labs/argocd-notifications.git",
			revision: "0123456789",
			Deployment: &GitLabDeployment{
				State:       "success",
				Environment: "production",
				Reference:   "v1.0.0",
				Tag:         &tag,
			},
		},
	}, Destination{})
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"environment": "production",
		"ref":         "v1.0.0",
		"sha":         "0123456789",
		"status":      "success",
		"tag":         true,
	}, body)
}

func TestSend_GitLab_MergeRequestComment(t *testing.T) {
	var notePaths []string
	var noteBodies []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			_, _ = writer.Write([]byte(`[{"iid": 11}, {"iid": 22}]`))
			return
		}
		data, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		var body map[string]any
		require.NoError(t, json.Unmarshal(data, &body))
		notePaths = append(notePaths, request.URL.EscapedPath())
		noteBodies = append(noteBodies, body["body"].(string))
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"id": 1}`))
	}))
	defer server.Close()

	service, err := NewGitLabService(GitLabOptions{BaseURL: server.URL, Token: "token"})
	require.NoError(t, err)

	err = service.Send(Notification{
		GitLab: &GitLabNotification{
			repoURL:             "https://gitlab.com/argoproj-labs/argocd-notifications.git",
			revision:            "0123456789",
			MergeRequestComment: &GitLabMergeRequestComment{Content: "Application is now running"},
		},
	}, Destination{})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"/api/v4/projects/argoproj-labs%2Fargocd-notifications/merge_requests/11/notes",
		"/api/v4/projects/argoproj-labs%2Fargocd-notifications/merge_requests/22/notes",
	}, notePaths)
	assert.Equal(t, []string{"Application is now running", "Application is now running"}, noteBodies)
}

func TestSend_GitLab_RejectedStatusStillComments(t *testing.T) {
	var notePaths []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/statuses/0123456789"):
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"message": "Cannot transition status via :run from :running"}`))
		case request.Method == http.MethodGet:
			_, _ = writer.Write([]byte(`[{"iid": 11}]`))
		default:
			notePaths = append(notePaths, request.URL.EscapedPath())
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte(`{"id": 1}`))
		}
	}))
	defer server.Close()

	service, err := NewGitLabService(GitLabOptions{BaseURL: server.URL, Token: "token"})
	require.NoError(t, err)

	err = service.Send(Notification{
		Message: "Application is now running",
		GitLab: &GitLabNotification{
			repoURL:             "https://gitlab.com/argoproj-labs/argocd-notifications.git",
			revision:            "0123456789",
			Status:              &GitLabStatus{State: "running"},
			MergeRequestComment: &GitLabMergeRequestComment{Content: "Application is now running"},
		},
	}, Destination{})
	require.ErrorContains(t, err, "Cannot transition status")

	assert.Equal(t, []string{
		"/api/v4/projects/argoproj-labs%2Fargocd-notifications/merge_requests/11/notes",
	}, notePaths)
}

func TestSend_GitLab_BadRepoURL(t *testing.T) {
	err := gitLabService{}.Send(Notification{
		GitLab: &GitLabNotification{repoURL: "hello"},
	}, Destination{})
	require.ErrorContains(t, err, "is not a project path")
}
