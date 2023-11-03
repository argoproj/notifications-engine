package services

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	texttemplate "text/template"

	log "github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
)

type GitLabOptions struct {
	BaseURL string `json:"baseURL"`
	Token   string `json:"token"`
}

type GitLabNotification struct {
	repoURL             string
	revision            string
	revisionIsTag       bool
	Status              *GitLabStatus              `json:"status,omitempty"`
	Deployment          *GitLabDeployment          `json:"deployment,omitempty"`
	MergeRequestComment *GitLabMergeRequestComment `json:"mergeRequestComment,omitempty"`
	RepoURLPath         string                     `json:"repoURLPath,omitempty"`
	RevisionPath        string                     `json:"revisionPath,omitempty"`
	RevisionIsTagPath   string                     `json:"revisionIsTagPath,omitempty"`
}

type GitLabStatus struct {
	State     string `json:"state,omitempty"`
	Label     string `json:"label,omitempty"`
	TargetURL string `json:"targetURL,omitempty"`
}

type GitLabDeployment struct {
	State          string `json:"state,omitempty"`
	Environment    string `json:"environment,omitempty"`
	EnvironmentURL string `json:"environmentURL,omitempty"`
}

type GitLabMergeRequestComment struct {
	Content string `json:"content,omitempty"`
}

const (
	gitlabRepoURLtemplate       = "{{.app.spec.source.repoURL}}"
	gitlabRevisionTemplate      = "{{.app.status.operationState.syncResult.revision}}"
	gitlabRevisionIsTagTemplate = "{{gt (len (call .repo.GitCommitMetadata .app.status.operationState.syncResult.revision).Tags) 0}}"
)

func (g *GitLabNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	if g.RepoURLPath == "" {
		g.RepoURLPath = gitlabRepoURLtemplate
	}
	if g.RevisionPath == "" {
		g.RevisionPath = gitlabRevisionTemplate
	}
	if g.RevisionIsTagPath == "" {
		g.RevisionIsTagPath = gitlabRevisionIsTagTemplate
	}

	repoURL, err := texttemplate.New(name).Funcs(f).Parse(g.RepoURLPath)
	if err != nil {
		return nil, err
	}

	revision, err := texttemplate.New(name).Funcs(f).Parse(g.RevisionPath)
	if err != nil {
		return nil, err
	}

	revisionIsTag, err := texttemplate.New(name).Funcs(f).Parse(g.RevisionIsTagPath)
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

	var deploymentState, environment, environmentURL *texttemplate.Template
	if g.Deployment != nil {
		deploymentState, err = texttemplate.New(name).Funcs(f).Parse(g.Deployment.State)
		if err != nil {
			return nil, err
		}

		environment, err = texttemplate.New(name).Funcs(f).Parse(g.Deployment.Environment)
		if err != nil {
			return nil, err
		}

		environmentURL, err = texttemplate.New(name).Funcs(f).Parse(g.Deployment.EnvironmentURL)
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

	return func(notification *Notification, vars map[string]interface{}) error {
		if notification.GitLab == nil {
			notification.GitLab = &GitLabNotification{
				RepoURLPath:       g.RepoURLPath,
				RevisionPath:      g.RevisionPath,
				RevisionIsTagPath: g.RevisionIsTagPath,
			}
		}

		var repoURLData bytes.Buffer
		if err := repoURL.Execute(&repoURLData, vars); err != nil {
			return err
		}
		notification.GitLab.repoURL = repoURLData.String()

		var revisionData bytes.Buffer
		if err := revision.Execute(&revisionData, vars); err != nil {
			return err
		}
		notification.GitLab.revision = revisionData.String()

		var revisionIsTagData bytes.Buffer
		if err := revisionIsTag.Execute(&revisionIsTagData, vars); err != nil {
			return err
		}
		revisionIsTag, err := strconv.ParseBool(revisionIsTagData.String())
		if err != nil {
			return err
		}
		notification.GitLab.revisionIsTag = revisionIsTag

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

			var environmentURLData bytes.Buffer
			if err := environmentURL.Execute(&environmentURLData, vars); err != nil {
				return err
			}
			notification.GitLab.Deployment.EnvironmentURL = environmentURLData.String()
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
	url := "https://gitlab.com/api/v4"
	if opts.BaseURL != "" {
		url = opts.BaseURL
	}

	var client *gitlab.Client
	client, err := gitlab.NewClient(
		opts.Token,
		gitlab.WithBaseURL(url),
		gitlab.WithCustomLogger(log.WithField("service", "gitlab")),
	)
	if err != nil {
		return nil, err
	}
	return &GitLabService{
		opts:   opts,
		client: client,
	}, nil
}

type GitLabService struct {
	opts GitLabOptions

	client *gitlab.Client
}

func (g GitLabService) Send(notification Notification, _ Destination) error {
	if notification.GitLab == nil {
		return fmt.Errorf("config is empty")
	}

	repo := fullNameByRepoURL(notification.GitLab.repoURL)
	if len(strings.Split(repo, "/")) < 2 {
		return fmt.Errorf("GitLab.repoURL (%s) does not have a `/`", notification.GitLab.repoURL)
	}
	pid := url.QueryEscape(repo)

	if notification.GitLab.Status != nil {
		// TODO: find the max length for status to truncate to
		// maximum length for status description is 140 characters
		// description := trunc(notification.Message, 140)

		_, _, err := g.client.Commits.SetCommitStatus(
			pid,
			notification.GitLab.revision,
			&gitlab.SetCommitStatusOptions{
				State:       gitlab.BuildStateValue(notification.GitLab.Status.State),
				Ref:         gitlab.String(notification.GitLab.revision),
				Name:        gitlab.String(notification.GitLab.Status.Label),
				TargetURL:   &notification.GitLab.Status.TargetURL,
				Description: gitlab.String(notification.Message),
			},
			gitlab.WithContext(context.TODO()),
		)
		if err != nil {
			return err
		}
	}

	if notification.GitLab.Deployment != nil {
		// maximum length for environment name is 255 characters
		environmentName := trunc(notification.GitLab.Deployment.Environment, 255)

		// TODO: If deployment already exists, update it
		// find how to get deployment ID

		_, _, err := g.client.Deployments.CreateProjectDeployment(
			pid,
			&gitlab.CreateProjectDeploymentOptions{
				Environment: gitlab.String(environmentName),
				Ref:         gitlab.String(notification.GitLab.revision),
				SHA:         gitlab.String(notification.GitLab.revision),
				Tag:         gitlab.Bool(notification.GitLab.revisionIsTag),
				Status:      gitlab.DeploymentStatus(gitlab.DeploymentStatusValue(notification.GitLab.Deployment.State)),
			},
			gitlab.WithContext(context.TODO()),
		)
		if err != nil {
			return err
		}
	}

	if notification.GitLab.MergeRequestComment != nil {
		// maximum length for merge request comment body is 1000000 characters
		body := trunc(notification.GitLab.MergeRequestComment.Content, 1000000)

		mrs, _, err := g.client.Commits.ListMergeRequestsByCommit(
			pid,
			notification.GitLab.revision,
			gitlab.WithContext(context.TODO()),
		)
		if err != nil {
			return err
		}

		for _, mr := range mrs {
			_, _, err := g.client.Notes.CreateMergeRequestNote(
				pid,
				mr.IID,
				&gitlab.CreateMergeRequestNoteOptions{
					Body: gitlab.String(body),
				},
				gitlab.WithContext(context.TODO()),
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
