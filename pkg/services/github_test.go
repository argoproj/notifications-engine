package services

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

var templateContext = map[string]interface{}{
	"context": map[string]interface{}{
		"argocdUrl": "https://example.com",
		"state":     "success",
	},
	"app": map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "argocd-notifications",
			"annotations": map[string]interface{}{
				"org.example/repoURL": "https://github.com/argoproj/argo-cd.git",
			},
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
}

func TestGetTemplater_GitHub(t *testing.T) {
	cases := []struct {
		name  string
		input Notification
		want  GitHubNotification
		err   bool
	}{
		{
			name: "DefaultTemplate",
			input: Notification{
				GitHub: &GitHubNotification{
					State:     "{{.context.state}}",
					Label:     "continuous-delivery/{{.app.metadata.name}}",
					TargetURL: "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
				}},
			want: GitHubNotification{
				RepoURL:   "https://github.com/argoproj-labs/argocd-notifications.git",
				Revision:  "0123456789",
				State:     "success",
				Label:     "continuous-delivery/argocd-notifications",
				TargetURL: "https://example.com/applications/argocd-notifications",
			},
			err: false,
		},
		{
			name: "SpecifiedRepoURL",
			input: Notification{
				GitHub: &GitHubNotification{
					RepoURL:   "{{index .app.metadata.annotations \"org.example/repoURL\"}}",
					State:     "{{.context.state}}",
					Label:     "continuous-delivery/{{.app.metadata.name}}",
					TargetURL: "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
				}},
			want: GitHubNotification{
				RepoURL:   "https://github.com/argoproj/argo-cd.git",
				Revision:  "0123456789",
				State:     "success",
				Label:     "continuous-delivery/argocd-notifications",
				TargetURL: "https://example.com/applications/argocd-notifications",
			},
			err: false,
		},
		{
			name: "InvalidRepoURLTemplate",
			input: Notification{
				GitHub: &GitHubNotification{
					RepoURL:   "{{.app/}}",
					State:     "{{.context.state}}",
					Label:     "continuous-delivery/{{.app.metadata.name}}",
					TargetURL: "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
				}},
			want: GitHubNotification{},
			err:  true,
		},
		{
			name: "InvalidRevisionTemplate",
			input: Notification{
				GitHub: &GitHubNotification{
					Revision:  "{{.app/}}",
					State:     "{{.context.state}}",
					Label:     "continuous-delivery/{{.app.metadata.name}}",
					TargetURL: "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
				}},
			want: GitHubNotification{},
			err:  true,
		},
		{
			name: "InvalidStateTemplate",
			input: Notification{
				GitHub: &GitHubNotification{
					State:     "{{.app/}}",
					Label:     "continuous-delivery/{{.app.metadata.name}}",
					TargetURL: "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
				}},
			want: GitHubNotification{},
			err:  true,
		},
		{
			name: "InvalidLabelTemplate",
			input: Notification{
				GitHub: &GitHubNotification{
					State:     "{{.context.state}}",
					Label:     "{{.app/}}",
					TargetURL: "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
				}},
			want: GitHubNotification{},
			err:  true,
		},
		{
			name: "InvalidTargetURL",
			input: Notification{
				GitHub: &GitHubNotification{
					State:     "{{.context.state}}",
					Label:     "continuous-delivery/{{.app.metadata.name}}",
					TargetURL: "{{.app/}}",
				}},
			want: GitHubNotification{},
			err:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			templater, err := tc.input.GetTemplater("", template.FuncMap{})

			if tc.err {
				if assert.Error(t, err) {
					return
				}
			} else {
				if !assert.NoError(t, err) {
					t.FailNow()
				}
			}

			var resultNotification Notification
			err = templater(&resultNotification, templateContext)

			if !assert.NoError(t, err) {
				t.FailNow()
			}

			result := resultNotification.GitHub
			assert.EqualValuesf(t, tc.want, *result, "%v failed", tc.name)
		})
	}
}
