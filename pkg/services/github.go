package services

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	texttemplate "text/template"
	"time"
	"unicode/utf8"

	"github.com/bradleyfalzon/ghinstallation/v2"
	giturls "github.com/chainguard-dev/git-urls"
	"github.com/google/go-github/v69/github"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
	"github.com/argoproj/notifications-engine/pkg/util/text"
)

var (
	gitSuffix = regexp.MustCompile(`\.git$`)
)

type GitHubOptions struct {
	AppID             interface{} `json:"appID"`
	InstallationID    interface{} `json:"installationID"`
	PrivateKey        string      `json:"privateKey"`
	EnterpriseBaseURL string      `json:"enterpriseBaseURL"`
}

type GitHubNotification struct {
	repoURL            string
	revision           string
	Status             *GitHubStatus             `json:"status,omitempty"`
	Deployment         *GitHubDeployment         `json:"deployment,omitempty"`
	PullRequestComment *GitHubPullRequestComment `json:"pullRequestComment,omitempty"`
	RepoURLPath        string                    `json:"repoURLPath,omitempty"`
	RevisionPath       string                    `json:"revisionPath,omitempty"`
	CheckRun           *GitHubCheckRun           `json:"checkRun,omitempty"`
}

type GitHubStatus struct {
	State     string `json:"state,omitempty"`
	Label     string `json:"label,omitempty"`
	TargetURL string `json:"targetURL,omitempty"`
}

type GitHubCheckRun struct {
	// head_sha  - this will be the revision path
	// external_id - this should have the details of argocd server
	Name        string                `json:"name,omitempty"`
	DetailsURL  string                `json:"details_url,omitempty"`
	Status      string                `json:"status,omitempty"`
	Conclusion  string                `json:"conclusion,omitempty"`
	StartedAt   string                `json:"started_at,omitempty"`
	CompletedAt string                `json:"completed_at,omitempty"`
	Output      *GitHubCheckRunOutput `json:"output,omitempty"`
}
type GitHubCheckRunOutput struct {
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
	Text    string `json:"text,omitempty"`
}

type GitHubDeployment struct {
	State                string   `json:"state,omitempty"`
	Environment          string   `json:"environment,omitempty"`
	EnvironmentURL       string   `json:"environmentURL,omitempty"`
	LogURL               string   `json:"logURL,omitempty"`
	RequiredContexts     []string `json:"requiredContexts"`
	AutoMerge            *bool    `json:"autoMerge,omitempty"`
	TransientEnvironment *bool    `json:"transientEnvironment,omitempty"`
	Reference            string   `json:"reference,omitempty"`
}

type GitHubPullRequestComment struct {
	Content    string `json:"content,omitempty"`
	CommentTag string `json:"commentTag,omitempty"`
}

const (
	repoURLtemplate  = "{{.app.spec.source.repoURL}}"
	revisionTemplate = "{{.app.status.operationState.syncResult.revision}}"
	commentTagFormat = "<!-- argocd-notifications %s -->"
	contentFormat    = "%s\n%s"
)

func (g *GitHubNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
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

	var deploymentState, environment, environmentURL, reference, logURL *texttemplate.Template
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

		reference, err = texttemplate.New(name).Funcs(f).Parse(g.Deployment.Reference)
		if err != nil {
			return nil, err
		}

		logURL, err = texttemplate.New(name).Funcs(f).Parse(g.Deployment.LogURL)
		if err != nil {
			return nil, err
		}
	}

	var pullRequestCommentContent *texttemplate.Template
	if g.PullRequestComment != nil {
		pullRequestCommentContent, err = texttemplate.New(name).Funcs(f).Parse(g.PullRequestComment.Content)
		if err != nil {
			return nil, err
		}
	}

	var checkRunName, detailsURL, status, conclusion, startedAt, completedAt *texttemplate.Template
	if g.CheckRun != nil {
		checkRunName, err = texttemplate.New(name).Funcs(f).Parse(g.CheckRun.Name)
		if err != nil {
			return nil, err
		}
		detailsURL, err = texttemplate.New(name).Funcs(f).Parse(g.CheckRun.DetailsURL)
		if err != nil {
			return nil, err
		}
		status, err = texttemplate.New(name).Funcs(f).Parse(g.CheckRun.Status)
		if err != nil {
			return nil, err
		}
		conclusion, err = texttemplate.New(name).Funcs(f).Parse(g.CheckRun.Conclusion)
		if err != nil {
			return nil, err
		}
		startedAt, err = texttemplate.New(name).Funcs(f).Parse(g.CheckRun.StartedAt)
		if err != nil {
			return nil, err
		}
		completedAt, err = texttemplate.New(name).Funcs(f).Parse(g.CheckRun.CompletedAt)
		if err != nil {
			return nil, err
		}
	}
	var checkRunTitle, summary, text *texttemplate.Template
	if g.CheckRun != nil && g.CheckRun.Output != nil {
		checkRunTitle, err = texttemplate.New(name).Funcs(f).Parse(g.CheckRun.Output.Title)
		if err != nil {
			return nil, err
		}
		summary, err = texttemplate.New(name).Funcs(f).Parse(g.CheckRun.Output.Summary)
		if err != nil {
			return nil, err
		}
		text, err = texttemplate.New(name).Funcs(f).Parse(g.CheckRun.Output.Text)
		if err != nil {
			return nil, err
		}
	}

	return func(notification *Notification, vars map[string]interface{}) error {
		if notification.GitHub == nil {
			notification.GitHub = &GitHubNotification{
				RepoURLPath:  g.RepoURLPath,
				RevisionPath: g.RevisionPath,
			}
		}

		var repoData bytes.Buffer
		if err := repoURL.Execute(&repoData, vars); err != nil {
			return err
		}
		notification.GitHub.repoURL = repoData.String()

		var revisionData bytes.Buffer
		if err := revision.Execute(&revisionData, vars); err != nil {
			return err
		}
		notification.GitHub.revision = revisionData.String()

		if g.Status != nil {
			if notification.GitHub.Status == nil {
				notification.GitHub.Status = &GitHubStatus{}
			}

			var stateData bytes.Buffer
			if err := statusState.Execute(&stateData, vars); err != nil {
				return err
			}
			notification.GitHub.Status.State = stateData.String()

			var labelData bytes.Buffer
			if err := label.Execute(&labelData, vars); err != nil {
				return err
			}
			notification.GitHub.Status.Label = labelData.String()

			var targetData bytes.Buffer
			if err := targetURL.Execute(&targetData, vars); err != nil {
				return err
			}
			notification.GitHub.Status.TargetURL = targetData.String()
		}

		if g.Deployment != nil {
			if notification.GitHub.Deployment == nil {
				notification.GitHub.Deployment = &GitHubDeployment{}
			}

			var stateData bytes.Buffer
			if err := deploymentState.Execute(&stateData, vars); err != nil {
				return err
			}
			notification.GitHub.Deployment.State = stateData.String()

			var environmentData bytes.Buffer
			if err := environment.Execute(&environmentData, vars); err != nil {
				return err
			}
			notification.GitHub.Deployment.Environment = environmentData.String()

			var environmentURLData bytes.Buffer
			if err := environmentURL.Execute(&environmentURLData, vars); err != nil {
				return err
			}
			notification.GitHub.Deployment.EnvironmentURL = environmentURLData.String()

			var logURLData bytes.Buffer
			if err := logURL.Execute(&logURLData, vars); err != nil {
				return err
			}
			notification.GitHub.Deployment.LogURL = logURLData.String()

			if g.Deployment.AutoMerge == nil {
				deploymentAutoMergeDefault := true
				notification.GitHub.Deployment.AutoMerge = &deploymentAutoMergeDefault
			} else {
				notification.GitHub.Deployment.AutoMerge = g.Deployment.AutoMerge
			}

			if g.Deployment.TransientEnvironment == nil {
				deploymentTransientEnvironmentDefault := false
				notification.GitHub.Deployment.TransientEnvironment = &deploymentTransientEnvironmentDefault
			} else {
				notification.GitHub.Deployment.TransientEnvironment = g.Deployment.TransientEnvironment
			}

			var referenceData bytes.Buffer
			if err := reference.Execute(&referenceData, vars); err != nil {
				return err
			}
			notification.GitHub.Deployment.Reference = referenceData.String()
			notification.GitHub.Deployment.RequiredContexts = g.Deployment.RequiredContexts
		}

		if g.PullRequestComment != nil {
			if notification.GitHub.PullRequestComment == nil {
				notification.GitHub.PullRequestComment = &GitHubPullRequestComment{}
			}

			var contentData bytes.Buffer
			if err := pullRequestCommentContent.Execute(&contentData, vars); err != nil {
				return err
			}
			notification.GitHub.PullRequestComment.Content = contentData.String()
			notification.GitHub.PullRequestComment.CommentTag = g.PullRequestComment.CommentTag

			if g.PullRequestComment.CommentTag != "" {
				notification.GitHub.PullRequestComment.Content = fmt.Sprintf(contentFormat,
					notification.GitHub.PullRequestComment.Content,
					fmt.Sprintf(commentTagFormat, g.PullRequestComment.CommentTag))
			}
		}

		if g.CheckRun != nil {
			if notification.GitHub.CheckRun == nil {
				notification.GitHub.CheckRun = &GitHubCheckRun{}
			}
			var checkRunNameData bytes.Buffer
			if err := checkRunName.Execute(&checkRunNameData, vars); err != nil {
				return err
			}
			notification.GitHub.CheckRun.Name = checkRunNameData.String()
			var detailsURLData bytes.Buffer
			if err := detailsURL.Execute(&detailsURLData, vars); err != nil {
				return err
			}
			notification.GitHub.CheckRun.DetailsURL = detailsURLData.String()

			var statusData bytes.Buffer
			if err := status.Execute(&statusData, vars); err != nil {
				return err
			}
			notification.GitHub.CheckRun.Status = statusData.String()
			var conclusionData bytes.Buffer
			if err := conclusion.Execute(&conclusionData, vars); err != nil {
				return err
			}
			notification.GitHub.CheckRun.Conclusion = conclusionData.String()
			var startedAtData bytes.Buffer
			if err := startedAt.Execute(&startedAtData, vars); err != nil {
				return err
			}
			notification.GitHub.CheckRun.StartedAt = startedAtData.String()
			var completedAtData bytes.Buffer
			if err := completedAt.Execute(&completedAtData, vars); err != nil {
				return err
			}
			notification.GitHub.CheckRun.CompletedAt = completedAtData.String()
		}
		if g.CheckRun != nil && g.CheckRun.Output != nil {
			if notification.GitHub.CheckRun.Output == nil {
				notification.GitHub.CheckRun.Output = &GitHubCheckRunOutput{}
			}
			var checkRunTitleData bytes.Buffer
			if err := checkRunTitle.Execute(&checkRunTitleData, vars); err != nil {
				return err
			}
			notification.GitHub.CheckRun.Output.Title = checkRunTitleData.String()
			var summaryData bytes.Buffer
			if err := summary.Execute(&summaryData, vars); err != nil {
				return err
			}
			notification.GitHub.CheckRun.Output.Summary = summaryData.String()
			var textData bytes.Buffer
			if err := text.Execute(&textData, vars); err != nil {
				return err
			}
			notification.GitHub.CheckRun.Output.Text = textData.String()
		}

		return nil
	}, nil
}

func NewGitHubService(opts GitHubOptions) (*gitHubService, error) {
	url := "https://api.github.com"
	if opts.EnterpriseBaseURL != "" {
		url = opts.EnterpriseBaseURL
	}

	appID, err := cast.ToInt64E(opts.AppID)
	if err != nil {
		return nil, err
	}

	installationID, err := cast.ToInt64E(opts.InstallationID)
	if err != nil {
		return nil, err
	}

	tr := httputil.NewLoggingRoundTripper(
		httputil.NewTransport(url, false), log.WithField("service", "github"))
	itr, err := ghinstallation.New(tr, appID, installationID, []byte(opts.PrivateKey))
	if err != nil {
		return nil, err
	}

	var client *github.Client
	if opts.EnterpriseBaseURL == "" {
		client = github.NewClient(&http.Client{Transport: itr})
	} else {
		itr.BaseURL = opts.EnterpriseBaseURL
		client, err = github.NewClient(&http.Client{Transport: itr}).WithEnterpriseURLs(opts.EnterpriseBaseURL, "")
		if err != nil {
			return nil, err
		}
	}

	return &gitHubService{
		opts:   opts,
		client: &githubClientAdapter{client: client},
	}, nil
}

type gitHubService struct {
	opts   GitHubOptions
	client githubClient
}

// Define interfaces for the GitHub client
type githubClient interface {
	GetIssues() issuesService
	GetPullRequests() pullRequestsService
	GetRepositories() repositoriesService
	GetChecks() checksService
}

type issuesService interface {
	ListComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, *github.Response, error)
	CreateComment(ctx context.Context, owner, repo string, number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error)
	EditComment(ctx context.Context, owner, repo string, commentID int64, comment *github.IssueComment) (*github.IssueComment, *github.Response, error)
}

type pullRequestsService interface {
	ListPullRequestsWithCommit(ctx context.Context, owner string, repo string, sha string, opts *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error)
}

type repositoriesService interface {
	CreateStatus(ctx context.Context, owner, repo, ref string, status *github.RepoStatus) (*github.RepoStatus, *github.Response, error)
	ListDeployments(ctx context.Context, owner, repo string, opts *github.DeploymentsListOptions) ([]*github.Deployment, *github.Response, error)
	CreateDeployment(ctx context.Context, owner, repo string, request *github.DeploymentRequest) (*github.Deployment, *github.Response, error)
	CreateDeploymentStatus(ctx context.Context, owner, repo string, deploymentID int64, request *github.DeploymentStatusRequest) (*github.DeploymentStatus, *github.Response, error)
}

type checksService interface {
	CreateCheckRun(ctx context.Context, owner, repo string, opts github.CreateCheckRunOptions) (*github.CheckRun, *github.Response, error)
}

// Adapter implementation
type githubClientAdapter struct {
	client *github.Client
}

func (g *githubClientAdapter) GetIssues() issuesService {
	return g.client.Issues
}

func (g *githubClientAdapter) GetPullRequests() pullRequestsService {
	return &pullRequestsServiceAdapter{service: g.client.PullRequests}
}

func (g *githubClientAdapter) GetRepositories() repositoriesService {
	return g.client.Repositories
}

func (g *githubClientAdapter) GetChecks() checksService {
	return g.client.Checks
}

func trunc(message string, n int) string {
	if utf8.RuneCountInString(message) > n {
		return string([]rune(message)[0:n-3]) + "..."
	}
	return message
}

func fullNameByRepoURL(rawURL string) string {
	parsed, err := giturls.Parse(rawURL)
	if err != nil {
		panic(err)
	}

	path := gitSuffix.ReplaceAllString(parsed.Path, "")
	if pathParts := text.SplitRemoveEmpty(path, "/"); len(pathParts) >= 2 {
		return strings.Join(pathParts[:2], "/")
	}

	return path
}

func (g gitHubService) Send(notification Notification, _ Destination) error {
	if notification.GitHub == nil {
		return fmt.Errorf("config is empty")
	}

	u := strings.Split(fullNameByRepoURL(notification.GitHub.repoURL), "/")
	if len(u) < 2 {
		return fmt.Errorf("GitHub.repoURL (%s) does not have a `/`", notification.GitHub.repoURL)
	}
	if notification.GitHub.Status != nil {
		// maximum is 140 characters
		description := trunc(notification.Message, 140)
		_, _, err := g.client.GetRepositories().CreateStatus(
			context.Background(),
			u[0],
			u[1],
			notification.GitHub.revision,
			&github.RepoStatus{
				State:       &notification.GitHub.Status.State,
				Description: &description,
				Context:     &notification.GitHub.Status.Label,
				TargetURL:   &notification.GitHub.Status.TargetURL,
			},
		)
		if err != nil {
			return err
		}
	}

	if notification.GitHub.Deployment != nil {
		// maximum is 140 characters
		description := trunc(notification.Message, 140)
		deployments, _, err := g.client.GetRepositories().ListDeployments(
			context.Background(),
			u[0],
			u[1],
			&github.DeploymentsListOptions{
				Ref:         notification.GitHub.revision,
				Environment: notification.GitHub.Deployment.Environment,
			},
		)
		if err != nil {
			return err
		}

		// if no reference is provided, use the revision
		ref := notification.GitHub.Deployment.Reference
		if ref == "" {
			ref = notification.GitHub.revision
		}

		var deployment *github.Deployment
		if len(deployments) != 0 {
			deployment = deployments[0]
		} else {
			deployment, _, err = g.client.GetRepositories().CreateDeployment(
				context.Background(),
				u[0],
				u[1],
				&github.DeploymentRequest{
					Ref:                  &ref,
					Environment:          &notification.GitHub.Deployment.Environment,
					RequiredContexts:     &notification.GitHub.Deployment.RequiredContexts,
					AutoMerge:            notification.GitHub.Deployment.AutoMerge,
					TransientEnvironment: notification.GitHub.Deployment.TransientEnvironment,
				},
			)
			if err != nil {
				return err
			}
		}
		_, _, err = g.client.GetRepositories().CreateDeploymentStatus(
			context.Background(),
			u[0],
			u[1],
			*deployment.ID,
			&github.DeploymentStatusRequest{
				State:          &notification.GitHub.Deployment.State,
				LogURL:         &notification.GitHub.Deployment.LogURL,
				Description:    &description,
				Environment:    &notification.GitHub.Deployment.Environment,
				EnvironmentURL: &notification.GitHub.Deployment.EnvironmentURL,
			},
		)
		if err != nil {
			return err
		}
	}

	if notification.GitHub.PullRequestComment != nil {
		// maximum is 65536 characters
		body := trunc(notification.GitHub.PullRequestComment.Content, 65536)
		commentTag := notification.GitHub.PullRequestComment.CommentTag

		prs, _, err := g.client.GetPullRequests().ListPullRequestsWithCommit(
			context.Background(),
			u[0],
			u[1],
			notification.GitHub.revision,
			nil,
		)
		if err != nil {
			return err
		}

		for _, pr := range prs {
			if commentTag != "" {
				// If comment tag is provided, try to find and update existing comment
				tagPattern := fmt.Sprintf(commentTagFormat, commentTag)
				comments, _, err := g.client.GetIssues().ListComments(
					context.Background(),
					u[0],
					u[1],
					pr.GetNumber(),
					nil,
				)
				if err != nil {
					return err
				}

				var existingComment *github.IssueComment
				for _, comment := range comments {
					if strings.Contains(comment.GetBody(), tagPattern) {
						existingComment = comment
						break
					}
				}

				if existingComment != nil {
					// Update existing comment
					updatedBody := fmt.Sprintf(contentFormat, body, tagPattern)
					existingComment.Body = &updatedBody
					_, _, err = g.client.GetIssues().EditComment(
						context.Background(),
						u[0],
						u[1],
						existingComment.GetID(),
						existingComment,
					)
					if err != nil {
						return err
					}
					continue
				}

				// If no existing comment found, create new one with tag
				body = fmt.Sprintf(contentFormat, body, tagPattern)
			}

			// Create new comment
			comment := &github.IssueComment{
				Body: &body,
			}
			_, _, err = g.client.GetIssues().CreateComment(
				context.Background(),
				u[0],
				u[1],
				pr.GetNumber(),
				comment,
			)
			if err != nil {
				return err
			}
		}
	}

	if notification.GitHub.CheckRun != nil {
		startedTime, err := time.Parse(time.RFC3339, notification.GitHub.CheckRun.StartedAt)
		if err != nil {
			return err
		}
		completedTime, err := time.Parse(time.RFC3339, notification.GitHub.CheckRun.CompletedAt)
		if err != nil {
			return err
		}
		externalID := "argocd-notifications"
		checkRunOutput := &github.CheckRunOutput{}
		if notification.GitHub.CheckRun.Output != nil {
			checkRunOutput = &github.CheckRunOutput{
				Title:   &notification.GitHub.CheckRun.Output.Title,
				Text:    &notification.GitHub.CheckRun.Output.Text,
				Summary: &notification.GitHub.CheckRun.Output.Summary,
			}
		}

		_, _, err = g.client.GetChecks().CreateCheckRun(
			context.Background(),
			u[0],
			u[1],
			github.CreateCheckRunOptions{
				HeadSHA:     notification.GitHub.revision,
				ExternalID:  &externalID,
				Name:        notification.GitHub.CheckRun.Name,
				DetailsURL:  &notification.GitHub.CheckRun.DetailsURL,
				Status: &notification.GitHub.CheckRun.Status,
				Conclusion: &notification.GitHub.CheckRun.Conclusion,
				StartedAt:   &github.Timestamp{Time: startedTime},
				CompletedAt: &github.Timestamp{Time: completedTime},
				Output:      checkRunOutput,
			},
		)

		if err != nil {
			return err
		}
	}

	return nil
}

// PullRequestsServiceAdapter adapts GitHub's PullRequestsService to our interface
type pullRequestsServiceAdapter struct {
	service *github.PullRequestsService
}

func (a *pullRequestsServiceAdapter) ListPullRequestsWithCommit(ctx context.Context, owner string, repo string, sha string, opts *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error) {
	// Convert PullRequestListOptions to ListOptions
	listOpts := &github.ListOptions{
		Page:    opts.Page,
		PerPage: opts.PerPage,
	}
	return a.service.ListPullRequestsWithCommit(ctx, owner, repo, sha, listOpts)
}
