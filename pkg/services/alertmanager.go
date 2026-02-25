package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	texttemplate "text/template"
	"time"

	log "github.com/sirupsen/logrus"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
)

const (
	alertNameLabel = "alertname"
)

// AlertmanagerNotification message body is similar to Prometheus alertmanager postableAlert model
type AlertmanagerNotification struct {
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	GeneratorURL string            `json:"generatorURL"`
	StartsAt     time.Time         `json:"startsAt"`
}

// AlertmanagerOptions cluster configuration
type AlertmanagerOptions struct {
	Targets            []string   `json:"targets"`
	Scheme             string     `json:"scheme"`
	APIPath            string     `json:"apiPath"`
	BasicAuth          *BasicAuth `json:"basicAuth"`
	BearerToken        string     `json:"bearerToken"`
	Timeout            int        `json:"timeout"`
	InsecureSkipVerify bool       `json:"insecureSkipVerify"`
	httputil.TransportOptions
}

// NewAlertmanagerService new service
func NewAlertmanagerService(opts AlertmanagerOptions) NotificationService {
	if len(opts.Targets) == 0 {
		opts.Targets = append(opts.Targets, "127.0.0.1:9093")
	}
	if opts.Scheme == "" {
		opts.Scheme = "http"
	}
	if opts.APIPath == "" {
		opts.APIPath = "/api/v2/alerts"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 3
	}

	return &alertmanagerService{entry: log.WithField("service", "alertmanager"), opts: opts}
}

type alertmanagerService struct {
	entry *log.Entry
	opts  AlertmanagerOptions
}

// GetTemplater parse text template
func (n AlertmanagerNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	return func(notification *Notification, vars map[string]any) error {
		if notification.Alertmanager == nil {
			notification.Alertmanager = &AlertmanagerNotification{}
		}
		notification.Alertmanager.StartsAt = time.Now()

		tmplGeneratorURL := n.GeneratorURL
		if tmplGeneratorURL == "" {
			tmplGeneratorURL = "{{.app.spec.source.repoURL}}"
		}
		tmpl, err := texttemplate.New(name).Funcs(f).Parse(tmplGeneratorURL)
		if err != nil {
			return err
		}
		var tempData bytes.Buffer
		if err := tmpl.Execute(&tempData, vars); err != nil {
			return err
		}
		if val := tempData.String(); val != "" {
			notification.Alertmanager.GeneratorURL = val
		}

		notification.Alertmanager.GeneratorURL = convertGitURLtoHTTP(notification.Alertmanager.GeneratorURL)

		// generatorURL must url format
		if _, err := url.Parse(notification.Alertmanager.GeneratorURL); err != nil {
			return err
		}

		if len(n.Labels) == 0 {
			return errors.New("at least one label pair required")
		}

		notification.Alertmanager.Labels = maps.Clone(n.Labels)
		if err := notification.Alertmanager.parseLabels(name, f, vars); err != nil {
			return err
		}
		if len(n.Annotations) > 0 {
			notification.Alertmanager.Annotations = maps.Clone(n.Annotations)
			if err := notification.Alertmanager.parseAnnotations(name, f, vars); err != nil {
				return err
			}
		}

		return nil
	}, nil
}

func (n *AlertmanagerNotification) parseAnnotations(name string, f texttemplate.FuncMap, vars map[string]any) error {
	for k, v := range n.Annotations {
		var tempData bytes.Buffer
		tmpl, err := texttemplate.New(name).Funcs(f).Parse(v)
		if err != nil {
			return err
		}
		if err := tmpl.Execute(&tempData, vars); err != nil {
			return err
		}
		if val := tempData.String(); val != "" {
			n.Annotations[k] = val
		}
	}

	return nil
}

func (n *AlertmanagerNotification) parseLabels(name string, f texttemplate.FuncMap, vars map[string]any) error {
	foundAlertname := false

	for k, v := range n.Labels {
		if k == alertNameLabel {
			foundAlertname = true
		}

		var tempData bytes.Buffer
		tmpl, err := texttemplate.New(name).Funcs(f).Parse(v)
		if err != nil {
			return err
		}
		if err := tmpl.Execute(&tempData, vars); err != nil {
			return err
		}
		if val := tempData.String(); val != "" {
			n.Labels[k] = val
		}
	}

	if !foundAlertname {
		n.Labels[alertNameLabel] = name
	}

	return nil
}

// Send using create alertmanager events
func (s alertmanagerService) Send(notification Notification, _ Destination) error {
	if notification.Alertmanager == nil {
		return errors.New("notification alertmanager no config")
	}
	if len(notification.Alertmanager.Labels) == 0 {
		return errors.New("alertmanager at least one label pair required")
	}

	rawBody, err := json.Marshal([]*AlertmanagerNotification{notification.Alertmanager})
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var numSuccess uint32

	for _, target := range s.opts.Targets {
		wg.Add(1)

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.opts.Timeout)*time.Second)
		defer cancel()

		go func(target string) {
			if err := s.sendOneTarget(ctx, target, rawBody); err != nil {
				s.entry.Errorf("alertmanager sent target: %v", err)
			} else {
				atomic.AddUint32(&numSuccess, 1)
			}

			wg.Done()
		}(target)
	}

	wg.Wait()

	if numSuccess == 0 {
		return errors.New("no events were successfully received by alertmanager")
	}

	return nil
}

func (s alertmanagerService) sendOneTarget(ctx context.Context, target string, rawBody []byte) (err error) {
	rawURL := fmt.Sprintf("%v://%v%v", s.opts.Scheme, target, s.opts.APIPath)

	client, err := httputil.NewServiceHTTPClient(s.opts.TransportOptions, s.opts.InsecureSkipVerify, rawURL, "alertmanager")
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(rawBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if s.opts.BasicAuth != nil {
		req.SetBasicAuth(s.opts.BasicAuth.Username, s.opts.BasicAuth.Password)
	} else if s.opts.BearerToken != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", s.opts.BearerToken))
	}

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
		return fmt.Errorf("request to %s has failed with error code %d : %s", rawURL, response.StatusCode, string(data))
	}

	return nil
}

func convertGitURLtoHTTP(url string) string {
	if !strings.HasPrefix(url, "git@") {
		return url
	}

	url = strings.TrimPrefix(url, "git@")

	// replace host:org/repo to host/org/repo
	url = strings.Replace(url, ":", "/", 1)

	return "https://" + url
}
