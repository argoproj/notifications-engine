package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	texttemplate "text/template"

	giturls "github.com/chainguard-dev/git-urls"
	gitlab "gitlab.com/gitlab-org/api/client-go"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
	"github.com/argoproj/notifications-engine/pkg/util/text"
)

type GitLabOptions struct {
	BaseURL            string `json:"baseURL"`
	Token              string `json:"token"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	httputil.TransportOptions
}

type GitLabNotification struct {
	repoURL             string
	revision            string
	Status              *GitLabStatus              `json:"status,omitempty"`
	Deployment          *GitLabDeployment          `json:"deployment,omitempty"`
	MergeRequestComment *GitLabMergeRequestComment `json:"mergeRequestComment,omitempty"`
	RepoURLPath         string                     `json:"repoURLPath,omitempty"`
	RevisionPath        string                     `json:"revisionPath,omitempty"`
}

type GitLabStatus struct {
	State     string `json:"state,omitempty"`
	Label     string `json:"label,omitempty"`
	TargetURL string `json:"targetURL,omitempty"`
}

type GitLabDeployment struct {
	State       string `json:"state,omitempty"`
	Environment string `json:"environment,omitempty"`
	Reference   string `json:"reference,omitempty"`
	Tag         *bool  `json:"tag,omitempty"`
}

type GitLabMergeRequestComment struct {
	Content string `json:"content,omitempty"`
}

// GitLab rejects a commit status whose description exceeds this.
const gitLabDescriptionMaxLength = 255

func (g *GitLabNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	if g.RepoURLPath == "" {
		g.RepoURLPath = repoURLtemplate
	}
	if g.RevisionPath == "" {
		g.RevisionPath = revisionTemplate
	}

	repoURL, err := texttemplate.New(name).Funcs(f).Parse(g.RepoURLPath)
	if err != nil {
		return nil, err
	}

	revision, err := texttemplate.New(name).Funcs(f).Parse(g.RevisionPath)
	if err != nil {
		return nil, err
	}

	var statusState, label, targetURL *texttemplate.Template
	if g.Status != nil {
		statusState, err = texttemplate.New(name).Funcs(f).Parse(g.Status.State)
		if err != nil {
			return nil, err
		}

		label, err = texttemplate.New(name).Funcs(f).Parse(g.Status.Label)
		if err != nil {
			return nil, err
		}

		targetURL, err = texttemplate.New(name).Funcs(f).Parse(g.Status.TargetURL)
		if err != nil {
			return nil, err
		}
	}

	var deploymentState, environment, reference *texttemplate.Template
	if g.Deployment != nil {
		deploymentState, err = texttemplate.New(name).Funcs(f).Parse(g.Deployment.State)
		if err != nil {
			return nil, err
		}

		environment, err = texttemplate.New(name).Funcs(f).Parse(g.Deployment.Environment)
		if err != nil {
			return nil, err
		}

		reference, err = texttemplate.New(name).Funcs(f).Parse(g.Deployment.Reference)
		if err != nil {
			return nil, err
		}
	}

	var mergeRequestCommentContent *texttemplate.Template
	if g.MergeRequestComment != nil {
		mergeRequestCommentContent, err = texttemplate.New(name).Funcs(f).Parse(g.MergeRequestComment.Content)
		if err != nil {
			return nil, err
		}
	}

	return func(notification *Notification, vars map[string]any) error {
		if notification.GitLab == nil {
			notification.GitLab = &GitLabNotification{
				RepoURLPath:  g.RepoURLPath,
				RevisionPath: g.RevisionPath,
			}
		}

		var repoData bytes.Buffer
		if err := repoURL.Execute(&repoData, vars); err != nil {
			return err
		}
		notification.GitLab.repoURL = repoData.String()

		var revisionData bytes.Buffer
		if err := revision.Execute(&revisionData, vars); err != nil {
			return err
		}
		notification.GitLab.revision = revisionData.String()

		if g.Status != nil {
			if notification.GitLab.Status == nil {
				notification.GitLab.Status = &GitLabStatus{}
			}

			var stateData bytes.Buffer
			if err := statusState.Execute(&stateData, vars); err != nil {
				return err
			}
			notification.GitLab.Status.State = stateData.String()

			var labelData bytes.Buffer
			if err := label.Execute(&labelData, vars); err != nil {
				return err
			}
			notification.GitLab.Status.Label = labelData.String()

			var targetData bytes.Buffer
			if err := targetURL.Execute(&targetData, vars); err != nil {
				return err
			}
			notification.GitLab.Status.TargetURL = targetData.String()
		}

		if g.Deployment != nil {
			if notification.GitLab.Deployment == nil {
				notification.GitLab.Deployment = &GitLabDeployment{}
			}

			var stateData bytes.Buffer
			if err := deploymentState.Execute(&stateData, vars); err != nil {
				return err
			}
			notification.GitLab.Deployment.State = stateData.String()

			var environmentData bytes.Buffer
			if err := environment.Execute(&environmentData, vars); err != nil {
				return err
			}
			notification.GitLab.Deployment.Environment = environmentData.String()

			var referenceData bytes.Buffer
			if err := reference.Execute(&referenceData, vars); err != nil {
				return err
			}
			notification.GitLab.Deployment.Reference = referenceData.String()
			notification.GitLab.Deployment.Tag = g.Deployment.Tag
		}

		if g.MergeRequestComment != nil {
			if notification.GitLab.MergeRequestComment == nil {
				notification.GitLab.MergeRequestComment = &GitLabMergeRequestComment{}
			}

			var contentData bytes.Buffer
			if err := mergeRequestCommentContent.Execute(&contentData, vars); err != nil {
				return err
			}
			notification.GitLab.MergeRequestComment.Content = contentData.String()
		}

		return nil
	}, nil
}

func NewGitLabService(opts GitLabOptions) (NotificationService, error) {
	url := "https://gitlab.com/"
	if opts.BaseURL != "" {
		url = opts.BaseURL
	}

	client, err := httputil.NewServiceHTTPClient(opts.TransportOptions, opts.InsecureSkipVerify, url, "gitlab")
	if err != nil {
		return nil, err
	}

	glclient, err := gitlab.NewClient(opts.Token, gitlab.WithBaseURL(url), gitlab.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return &gitLabService{client: glclient}, nil
}

type gitLabService struct {
	client *gitlab.Client
}

// GitLab addresses a project by its full namespaced path, which unlike GitHub may contain subgroups.
func projectPathByRepoURL(rawURL string) (string, error) {
	parsed, err := giturls.Parse(rawURL)
	if err != nil {
		return "", err
	}

	pathParts := text.SplitRemoveEmpty(gitSuffix.ReplaceAllString(parsed.Path, ""), "/")
	if len(pathParts) < 2 {
		return "", fmt.Errorf("GitLab.repoURL (%s) is not a project path, expected at least <namespace>/<project>", rawURL)
	}

	return strings.Join(pathParts, "/"), nil
}

func (g gitLabService) Send(notification Notification, _ Destination) error {
	if notification.GitLab == nil {
		return errors.New("config is empty")
	}

	pid, err := projectPathByRepoURL(notification.GitLab.repoURL)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Each action reports independently: a status GitLab refuses must not stop the
	// merge request note, which the trigger's oncePer would never retry.
	var errs []error

	if notification.GitLab.Status != nil {
		opts := &gitlab.SetCommitStatusOptions{
			State:       gitlab.BuildStateValue(notification.GitLab.Status.State),
			Description: gitlab.Ptr(trunc(notification.Message, gitLabDescriptionMaxLength)),
		}
		if notification.GitLab.Status.Label != "" {
			opts.Name = gitlab.Ptr(notification.GitLab.Status.Label)
		}
		if notification.GitLab.Status.TargetURL != "" {
			opts.TargetURL = gitlab.Ptr(notification.GitLab.Status.TargetURL)
		}

		if _, _, err := g.client.Commits.SetCommitStatus(pid, notification.GitLab.revision, opts, gitlab.WithContext(ctx)); err != nil {
			errs = append(errs, err)
		}
	}

	if notification.GitLab.Deployment != nil {
		ref := notification.GitLab.Deployment.Reference
		if ref == "" {
			ref = notification.GitLab.revision
		}

		tag := false
		if notification.GitLab.Deployment.Tag != nil {
			tag = *notification.GitLab.Deployment.Tag
		}

		if _, _, err := g.client.Deployments.CreateProjectDeployment(pid, &gitlab.CreateProjectDeploymentOptions{
			Environment: gitlab.Ptr(notification.GitLab.Deployment.Environment),
			Ref:         gitlab.Ptr(ref),
			SHA:         gitlab.Ptr(notification.GitLab.revision),
			Tag:         gitlab.Ptr(tag),
			Status:      gitlab.Ptr(gitlab.DeploymentStatusValue(notification.GitLab.Deployment.State)),
		}, gitlab.WithContext(ctx)); err != nil {
			errs = append(errs, err)
		}
	}

	if notification.GitLab.MergeRequestComment != nil {
		mrs, _, err := g.client.Commits.ListMergeRequestsByCommit(pid, notification.GitLab.revision, gitlab.WithContext(ctx))
		if err != nil {
			errs = append(errs, err)
		}

		for _, mr := range mrs {
			if _, _, err := g.client.Notes.CreateMergeRequestNote(pid, mr.IID, &gitlab.CreateMergeRequestNoteOptions{
				Body: gitlab.Ptr(notification.GitLab.MergeRequestComment.Content),
			}, gitlab.WithContext(ctx)); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}
