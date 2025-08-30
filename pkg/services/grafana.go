package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	texttemplate "text/template"
	"time"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"

	log "github.com/sirupsen/logrus"
)

type GrafanaNotification struct {
	Tags string `json:"tags,omitempty"`
}

func (n *GrafanaNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	grafanaTags, err := texttemplate.New(name).Funcs(f).Parse(n.Tags)
	if err != nil {
		return nil, err
	}

	return func(notification *Notification, vars map[string]interface{}) error {
		if notification.Grafana == nil {
			notification.Grafana = &GrafanaNotification{}
		}
		var grafanaTagsData bytes.Buffer
		if err := grafanaTags.Execute(&grafanaTagsData, vars); err != nil {
			return err
		}
		notification.Grafana.Tags = grafanaTagsData.String()

	
		return nil
	}, nil
}

type GrafanaOptions struct {
	ApiUrl             string `json:"apiUrl"`
	ApiKey             string `json:"apiKey"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	Tags               string `json:"tags"`
}

type grafanaService struct {
	opts GrafanaOptions
}

func NewGrafanaService(opts GrafanaOptions) NotificationService {
	return &grafanaService{opts: opts}
}

type GrafanaAnnotation struct {
	Time     int64    `json:"time"` // unix ts in ms
	IsRegion bool     `json:"isRegion"`
	Tags     []string `json:"tags"`
	Text     string   `json:"text"`
}

func (s *grafanaService) Send(notification Notification, dest Destination) error {
    tags := strings.Split(dest.Recipient, "|")

	// append tags from notification grafana.tags field .. 
    if notification.Grafana != nil && notification.Grafana.Tags != "" {
		notificationTags := strings.Split(notification.Grafana.Tags, "|")
        tags = append(tags, notificationTags...)
    }

	// append global tags from opts
	 if s.opts.Tags != "" {
		optsTags := strings.Split(s.opts.Tags, "|")
        tags = append(tags, optsTags...)
    }

	ga := GrafanaAnnotation{
		Time:     time.Now().Unix() * 1000, // unix ts in ms
		IsRegion: false,
		Tags:     tags,
		Text:     notification.Message,
	}

	if notification.Message == "" {
		log.Warnf("Message is an empty string or not provided in the notifications template")
	}

	client := &http.Client{
		Transport: httputil.NewLoggingRoundTripper(
			httputil.NewTransport(s.opts.ApiUrl, s.opts.InsecureSkipVerify), log.WithField("service", "grafana")),
	}

	jsonValue, _ := json.Marshal(ga)
	apiUrl, err := url.Parse(s.opts.ApiUrl)

	if err != nil {
		return err
	}
	annotationApi := *apiUrl
	annotationApi.Path = path.Join(apiUrl.Path, "annotations")
	req, err := http.NewRequest(http.MethodPost, annotationApi.String(), bytes.NewBuffer(jsonValue))
	if err != nil {
		log.Errorf("Failed to create grafana annotation request: %s", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.opts.ApiKey))

	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("unable to read response data: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("request to %s has failed with error code %d : %s", s.opts.ApiUrl, response.StatusCode, string(data))
	}

	return err
}
