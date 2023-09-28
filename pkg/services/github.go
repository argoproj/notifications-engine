package services

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"
	"strings"
	texttemplate "text/template"
	"unicode/utf8"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v41/github"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
	giturls "github.com/whilp/git-urls"

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
	CheckRun           *GitHubCheckRun           `json:"checkRun,omitempty"`
	RepoURLPath        string                    `json:"repoURLPath,omitempty"`
	RevisionPath       string                    `json:"revisionPath,omitempty"`
}

type GitHubStatus struct {
	State     string `json:"state,omitempty"`
	Label     string `json:"label,omitempty"`
	TargetURL string `json:"targetURL,omitempty"`
}

//copy of github:UpdateCheckRunOptions + id + timestamp as string
type GitHubCheckRun struct {
	Id          string            `json:"id"`                     // check_id, actually an int64, but string since we want it to be template'able. (Optional - create new check-run for revision if missing.)
	Name        string            `json:"name"`                   // The name of the check (e.g., "code-coverage"). (Required.)
	DetailsURL  string            `json:"details_url,omitempty"`  // The URL of the integrator's site that has the full details of the check. (Optional.)
	ExternalID  string            `json:"external_id,omitempty"`  // A reference for the run on the integrator's system. (Optional.)
	Status      string            `json:"status,omitempty"`       // The current status. Can be one of "queued", "in_progress", or "completed". Default: "queued". (Optional.)
	Conclusion  string            `json:"conclusion,omitempty"`   // Can be one of "success", "failure", "neutral", "cancelled", "skipped", "timed_out", or "action_required". (Optional. Required if you provide a status of "completed".)
	CompletedAt string            `json:"completed_at,omitempty"` // The time the check completed. (Optional. Required if you provide conclusion.)
	Output      *github.CheckRunOutput   `json:"output,omitempty"`       // Provide descriptive details about the run. (Optional)
	Actions     []*github.CheckRunAction `json:"actions,omitempty"`      // Possible further actions the integrator can perform, which a user may trigger. (Optional.)
}

type GitHubDeployment struct {
	State            string   `json:"state,omitempty"`
	Environment      string   `json:"environment,omitempty"`
	EnvironmentURL   string   `json:"environmentURL,omitempty"`
	LogURL           string   `json:"logURL,omitempty"`
	RequiredContexts []string `json:"requiredContexts"`
	AutoMerge        *bool    `json:"autoMerge,omitempty"`
}

type GitHubPullRequestComment struct {
	Content string `json:"content,omitempty"`
}

const (
	repoURLtemplate  = "{{.app.spec.source.repoURL}}"
	revisionTemplate = "{{.app.status.operationState.syncResult.revision}}"
)

func (g *GitHubNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	if g.RepoURLPath == "" {
		g.RepoURLPath = repoURLtemplate
	}
	if g.RevisionPath == "" {
		g.RevisionPath = revisionTemplate
	}

	type GetterSetter struct {
		G func(*GitHubNotification) *string
		S func(*GitHubNotification, string)
	}
	createField := "create"

	//list of template'able fields
	fields := []GetterSetter{
		{G: func(x *GitHubNotification) *string { return &x.RepoURLPath  }, S: func(x *GitHubNotification, val string) { x.repoURL  = val }},
		{G: func(x *GitHubNotification) *string { return &x.RevisionPath }, S: func(x *GitHubNotification, val string) { x.revision = val }},

		//Status support
		{G: func(x *GitHubNotification) *string { if x.Status != nil { return &createField        } else { return nil } }, S: func(x *GitHubNotification, val string) { if x.Status == nil { x.Status = &GitHubStatus{} } }},
		{G: func(x *GitHubNotification) *string { if x.Status != nil { return &x.Status.State     } else { return nil } }, S: func(x *GitHubNotification, val string) { x.Status.State     = val }},
		{G: func(x *GitHubNotification) *string { if x.Status != nil { return &x.Status.Label     } else { return nil } }, S: func(x *GitHubNotification, val string) { x.Status.Label     = val }},
		{G: func(x *GitHubNotification) *string { if x.Status != nil { return &x.Status.TargetURL } else { return nil } }, S: func(x *GitHubNotification, val string) { x.Status.TargetURL = val }},

		//Deployment support
		{G: func(x *GitHubNotification) *string { if x.Deployment != nil { return &createField                } else { return nil } }, S: func(x *GitHubNotification, val string) { if x.Deployment == nil { x.Deployment = &GitHubDeployment{} } }},
		{G: func(x *GitHubNotification) *string { if x.Deployment != nil { return &x.Deployment.State         } else { return nil } }, S: func(x *GitHubNotification, val string) { x.Deployment.State           = val }},
		{G: func(x *GitHubNotification) *string { if x.Deployment != nil { return &x.Deployment.Environment   } else { return nil } }, S: func(x *GitHubNotification, val string) { x.Deployment.Environment     = val }},
		{G: func(x *GitHubNotification) *string { if x.Deployment != nil { return &x.Deployment.EnvironmentURL} else { return nil } }, S: func(x *GitHubNotification, val string) { x.Deployment.EnvironmentURL  = val }},
		{G: func(x *GitHubNotification) *string { if x.Deployment != nil { return &x.Deployment.LogURL        } else { return nil } }, S: func(x *GitHubNotification, val string) { x.Deployment.LogURL          = val }},

		//PullRequestComment support
		{G: func(x *GitHubNotification) *string { if x.PullRequestComment != nil { return &createField                  } else { return nil } }, S: func(x *GitHubNotification, val string) { if x.PullRequestComment == nil { x.PullRequestComment = &GitHubPullRequestComment{} } }},
		{G: func(x *GitHubNotification) *string { if x.PullRequestComment != nil { return &x.PullRequestComment.Content } else { return nil } }, S: func(x *GitHubNotification, val string) { x.PullRequestComment.Content = val }},

		//CheckRunUpdate support
		{G: func(x *GitHubNotification) *string { if x.CheckRun != nil { return &createField           } else { return nil } }, S: func(x *GitHubNotification, val string) { if x.CheckRun == nil { x.CheckRun = &GitHubCheckRun{} } }},
		{G: func(x *GitHubNotification) *string { if x.CheckRun != nil { return &x.CheckRun.Id         } else { return nil } }, S: func(x *GitHubNotification, val string) { x.CheckRun.Id          = val }},
		{G: func(x *GitHubNotification) *string { if x.CheckRun != nil { return &x.CheckRun.Name       } else { return nil } }, S: func(x *GitHubNotification, val string) { x.CheckRun.Name        = val }},
		{G: func(x *GitHubNotification) *string { if x.CheckRun != nil { return &x.CheckRun.DetailsURL } else { return nil } }, S: func(x *GitHubNotification, val string) { x.CheckRun.DetailsURL  = val }},
		{G: func(x *GitHubNotification) *string { if x.CheckRun != nil { return &x.CheckRun.ExternalID } else { return nil } }, S: func(x *GitHubNotification, val string) { x.CheckRun.ExternalID  = val }},
		{G: func(x *GitHubNotification) *string { if x.CheckRun != nil { return &x.CheckRun.Status     } else { return nil } }, S: func(x *GitHubNotification, val string) { x.CheckRun.Status      = val }},
		{G: func(x *GitHubNotification) *string { if x.CheckRun != nil { return &x.CheckRun.Conclusion } else { return nil } }, S: func(x *GitHubNotification, val string) { x.CheckRun.Conclusion  = val }},
		{G: func(x *GitHubNotification) *string { if x.CheckRun != nil { return &x.CheckRun.CompletedAt} else { return nil } }, S: func(x *GitHubNotification, val string) { x.CheckRun.CompletedAt = val }},

		//CheckRunUpdate.Output support
		{G: func(x *GitHubNotification) *string { if x.CheckRun != nil && x.CheckRun.Output != nil { return &createField                     } else { return nil } }, S: func(x *GitHubNotification, val string) { if x.CheckRun.Output == nil { x.CheckRun.Output = &github.CheckRunOutput{} } }},
		{G: func(x *GitHubNotification) *string { if x.CheckRun != nil && x.CheckRun.Output != nil { return x.CheckRun.Output.Title          } else { return nil } }, S: func(x *GitHubNotification, val string) { x.CheckRun.Output.Title          = &val }},
		{G: func(x *GitHubNotification) *string { if x.CheckRun != nil && x.CheckRun.Output != nil { return x.CheckRun.Output.Summary        } else { return nil } }, S: func(x *GitHubNotification, val string) { x.CheckRun.Output.Summary        = &val }},
		{G: func(x *GitHubNotification) *string { if x.CheckRun != nil && x.CheckRun.Output != nil { return x.CheckRun.Output.Text           } else { return nil } }, S: func(x *GitHubNotification, val string) { x.CheckRun.Output.Text           = &val }},
		{G: func(x *GitHubNotification) *string { if x.CheckRun != nil && x.CheckRun.Output != nil { return x.CheckRun.Output.AnnotationsURL } else { return nil } }, S: func(x *GitHubNotification, val string) { x.CheckRun.Output.AnnotationsURL = &val }},
	};

	type TmplSetter struct {
		S func(*GitHubNotification, string)
		T *texttemplate.Template
	}

	templates := []TmplSetter{}

	for _, field := range fields {
		templateStr := field.G(g)
		if templateStr == nil {
			continue
		}
		tmpl, err := texttemplate.New(name).Funcs(f).Parse(*templateStr)
		if err != nil {
			return nil, err
		}
		templates = append(templates, TmplSetter{S: field.S, T: tmpl})
	}

	return func(notification *Notification, vars map[string]interface{}) error {
		if notification.GitHub == nil {
			notification.GitHub = &GitHubNotification{
				RepoURLPath:  g.RepoURLPath,
				RevisionPath: g.RevisionPath,
			}
		}

		for _, tmplFunc := range templates {
			var data bytes.Buffer
			if err := tmplFunc.T.Execute(&data, vars); err != nil {
				return err
			}
			tmplFunc.S(notification.GitHub, data.String())
		}

		//non-template'able props
		if g.Deployment != nil {
			if g.Deployment.AutoMerge == nil {
				deploymentAutoMergeDefault := true
				notification.GitHub.Deployment.AutoMerge = &deploymentAutoMergeDefault
			} else {
				notification.GitHub.Deployment.AutoMerge = g.Deployment.AutoMerge
			}
			notification.GitHub.Deployment.RequiredContexts = g.Deployment.RequiredContexts
		}
		if g.CheckRun != nil {
			notification.GitHub.CheckRun.Actions = g.CheckRun.Actions
			if g.CheckRun.Output != nil {
				notification.GitHub.CheckRun.Output.Annotations = g.CheckRun.Output.Annotations
				notification.GitHub.CheckRun.Output.Images      = g.CheckRun.Output.Images
			}
		}
		return nil
	}, nil
}

func NewGitHubService(opts GitHubOptions) (NotificationService, error) {
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
		client, err = github.NewEnterpriseClient(opts.EnterpriseBaseURL, "", &http.Client{Transport: itr})
		if err != nil {
			return nil, err
		}
	}

	return &gitHubService{
		opts:   opts,
		client: client,
	}, nil
}

type gitHubService struct {
	opts GitHubOptions

	client *github.Client
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

	if notification.GitHub.Status != nil {
		u := strings.Split(fullNameByRepoURL(notification.GitHub.repoURL), "/")
		// maximum is 140 characters
		description := trunc(notification.Message, 140)
		_, _, err := g.client.Repositories.CreateStatus(
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
		u := strings.Split(fullNameByRepoURL(notification.GitHub.repoURL), "/")
		// maximum is 140 characters
		description := trunc(notification.Message, 140)
		deployment, _, err := g.client.Repositories.CreateDeployment(
			context.Background(),
			u[0],
			u[1],
			&github.DeploymentRequest{
				Ref:              &notification.GitHub.revision,
				Environment:      &notification.GitHub.Deployment.Environment,
				RequiredContexts: &notification.GitHub.Deployment.RequiredContexts,
				AutoMerge:        notification.GitHub.Deployment.AutoMerge,
			},
		)
		if err != nil {
			return err
		}
		_, _, err = g.client.Repositories.CreateDeploymentStatus(
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
		u := strings.Split(fullNameByRepoURL(notification.GitHub.repoURL), "/")
		// maximum is 65536 characters
		body := trunc(notification.GitHub.PullRequestComment.Content, 65536)
		comment := &github.IssueComment{
			Body: &body,
		}

		prs, _, err := g.client.PullRequests.ListPullRequestsWithCommit(
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
			_, _, err = g.client.Issues.CreateComment(
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
		u := strings.Split(fullNameByRepoURL(notification.GitHub.repoURL), "/")
		var id int64
		if notification.GitHub.CheckRun.Id != "" {
			parsedId, err := strconv.ParseInt(notification.GitHub.CheckRun.Id, 10, 64)
			if err != nil {
				return err
			}
			id := parsedId
		}
		if id == 0 {
			checkrun, _, err := g.client.Checks.CreateCheckRun(
				context.Background(),
				u[0],
				u[1],
				github.CreateCheckRunOptions{
					Name:    notification.GitHub.CheckRun.Name,
					HeadSHA: notification.GitHub.revision,
				},
			)
			if err != nil {
				return err
			}
			id := checkrun.ID
		}
		var timestamp *github.Timestamp
		if notification.GitHub.CheckRun.CompletedAt != "" {
			parsedTime, err := time.Parse("2006-01-02T15:04:05Z07:00", notification.GitHub.CheckRun.CompletedAt)
			if err != nil {
				return err
			}
			timestamp = &github.Timestamp{parsedTime}
		}
		_, _, err := g.client.Checks.UpdateCheckRun(
			context.Background(),
			u[0],
			u[1],
			id,
			github.UpdateCheckRunOptions{
				Name       : notification.GitHub.CheckRun.Name,       
				DetailsURL : &notification.GitHub.CheckRun.DetailsURL, 
				ExternalID : &notification.GitHub.CheckRun.ExternalID, 
				Status     : &notification.GitHub.CheckRun.Status,     
				Conclusion : &notification.GitHub.CheckRun.Conclusion, 
				CompletedAt: timestamp,
				Output     : notification.GitHub.CheckRun.Output,     
				Actions    : notification.GitHub.CheckRun.Actions,    
			},
		)
		if err != nil {
			return err
		}
	}

	return nil
}

