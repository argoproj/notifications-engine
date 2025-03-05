package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	texttemplate "text/template"

	log "github.com/sirupsen/logrus"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
)

type NewrelicOptions struct {
	ApiKey             string `json:"apiKey"`
	ApiURL             string `json:"apiURL"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	httputil.TransportOptions
}

type NewrelicNotification struct {
	Revision    string `json:"revision"`
	Changelog   string `json:"changelog,omitempty"`
	Description string `json:"description,omitempty"`
	User        string `json:"user,omitempty"`
}

var (
	ErrMissingConfig        = errors.New("config is missing")
	ErrMissingApiKey        = errors.New("apiKey is missing")
	ErrAppIdMultipleMatches = errors.New("multiple matches found for application name")
	ErrAppIdNoMatches       = errors.New("no matches found for application name")
)

func (n *NewrelicNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	revisionTemplate := n.Revision
	if revisionTemplate == "" {
		revisionTemplate = "{{.app.status.operationState.syncResult.revision}}"
	}

	revision, err := texttemplate.New(name).Funcs(f).Parse(revisionTemplate)
	if err != nil {
		return nil, err
	}
	description, err := texttemplate.New(name).Funcs(f).Parse(n.Description)
	if err != nil {
		return nil, err
	}

	changelogTemplate := n.Changelog
	if changelogTemplate == "" {
		changelogTemplate = "{{(call .repo.GetCommitMetadata .app.status.sync.revision).Message}}"
	}

	changelog, err := texttemplate.New(name).Funcs(f).Parse(changelogTemplate)
	if err != nil {
		return nil, err
	}

	commitAuthorTemplate := n.User
	if commitAuthorTemplate == "" {
		commitAuthorTemplate = "{{(call .repo.GetCommitMetadata .app.status.sync.revision).Author}}"
	}

	user, err := texttemplate.New(name).Funcs(f).Parse(commitAuthorTemplate)
	if err != nil {
		return nil, err
	}

	return func(notification *Notification, vars map[string]any) error {
		if notification.Newrelic == nil {
			notification.Newrelic = &NewrelicNotification{}
		}
		var revisionData bytes.Buffer
		if err := revision.Execute(&revisionData, vars); err != nil {
			return err
		}
		notification.Newrelic.Revision = revisionData.String()

		var changelogData bytes.Buffer
		if err := changelog.Execute(&changelogData, vars); err != nil {
			return err
		}
		notification.Newrelic.Changelog = changelogData.String()

		var descriptionData bytes.Buffer
		if err := description.Execute(&descriptionData, vars); err != nil {
			return err
		}
		notification.Newrelic.Description = descriptionData.String()

		var userData bytes.Buffer
		if err := user.Execute(&userData, vars); err != nil {
			return err
		}
		notification.Newrelic.User = userData.String()

		return nil
	}, nil
}

func NewNewrelicService(opts NewrelicOptions) NotificationService {
	if opts.ApiURL == "" {
		opts.ApiURL = "https://api.newrelic.com"
	} else {
		opts.ApiURL = strings.TrimSuffix(opts.ApiURL, "/")
	}

	return &newrelicService{opts: opts}
}

type newrelicService struct {
	opts NewrelicOptions
}
type newrelicDeploymentMarkerRequest struct {
	Deployment NewrelicNotification `json:"deployment"`
}
type newrelicApplicationsResponse struct {
	Applications []struct {
		ID json.Number `json:"id"`
	} `json:"applications"`
}

func (s newrelicService) getApplicationId(client *http.Client, appName string) (string, error) {
	applicationsApi := fmt.Sprintf("%s/v2/applications.json?filter[name]=%s", s.opts.ApiURL, appName)
	req, err := http.NewRequest(http.MethodGet, applicationsApi, nil)
	if err != nil {
		return "", fmt.Errorf("Failed to create filtered application request: %s", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", s.opts.ApiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data newrelicApplicationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("Failed to decode applications response: %s", err)
	}

	if len(data.Applications) == 0 {
		return "", ErrAppIdNoMatches
	}

	if len(data.Applications) > 1 {
		return "", ErrAppIdMultipleMatches
	}

	return data.Applications[0].ID.String(), nil
}

func (s newrelicService) Send(notification Notification, dest Destination) (err error) {
	if s.opts.ApiKey == "" {
		return ErrMissingApiKey
	}

	if notification.Newrelic == nil {
		return ErrMissingConfig
	}

	if notification.Newrelic.Description == "" {
		notification.Newrelic.Description = notification.Message
	}

	deploymentMarker := newrelicDeploymentMarkerRequest{
		Deployment: NewrelicNotification{
			notification.Newrelic.Revision,
			notification.Newrelic.Changelog,
			notification.Newrelic.Description,
			notification.Newrelic.User,
		},
	}

	client, err := httputil.NewServiceHTTPClient(s.opts.TransportOptions, s.opts.InsecureSkipVerify, s.opts.ApiURL, "newrelic")
	if err != nil {
		return err
	}

	jsonValue, err := json.Marshal(deploymentMarker)
	if err != nil {
		return err
	}

	var appId = dest.Recipient
	if dest.Recipient != "" {
		_, err := strconv.Atoi(dest.Recipient)
		if err != nil {
			log.Debugf(
				"Recipient was provided by application name. Looking up the application id for %s",
				dest.Recipient,
			)
			appId, err = s.getApplicationId(client, dest.Recipient)
			if err != nil {
				log.Errorf("Failed to lookup application %s by name: %s", dest.Recipient, err)
				return err
			}
		}
	}
	markerApi := fmt.Sprintf(s.opts.ApiURL+"/v2/applications/%s/deployments.json", appId)
	req, err := http.NewRequest(http.MethodPost, markerApi, bytes.NewBuffer(jsonValue))
	if err != nil {
		log.Errorf("Failed to create deployment marker request: %s", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", s.opts.ApiKey)

	_, err = client.Do(req)
	return err
}
