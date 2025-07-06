package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	Targets             []string   `json:"targets"`
	Scheme              string     `json:"scheme"`
	APIPath             string     `json:"apiPath"`
	BasicAuth           *BasicAuth `json:"basicAuth"`
	BearerToken         string     `json:"bearerToken"`
	Timeout             int        `json:"timeout"`
	InsecureSkipVerify  bool       `json:"insecureSkipVerify"`
	MaxIdleConns        int        `json:"maxIdleConns"`
	MaxIdleConnsPerHost int        `json:"maxIdleConnsPerHost"`
	MaxConnsPerHost     int        `json:"maxConnsPerHost"`
	IdleConnTimeout     string     `json:"idleConnTimeout"`
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
	return func(notification *Notification, vars map[string]interface{}) error {
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

		if len(n.Labels) <= 0 {
			return fmt.Errorf("at least one label pair required")
		}

		notification.Alertmanager.Labels = n.Labels
		if err := notification.Alertmanager.parseLabels(name, f, vars); err != nil {
			return err
		}
		if len(n.Annotations) > 0 {
			notification.Alertmanager.Annotations = n.Annotations
			if err := notification.Alertmanager.parseAnnotations(name, f, vars); err != nil {
				return err
			}
		}

		return nil
	}, nil
}

func (n *AlertmanagerNotification) parseAnnotations(name string, f texttemplate.FuncMap, vars map[string]interface{}) error {
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

func (n *AlertmanagerNotification) parseLabels(name string, f texttemplate.FuncMap, vars map[string]interface{}) error {
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
func (s alertmanagerService) Send(notification Notification, dest Destination) error {
	if notification.Alertmanager == nil {
		return fmt.Errorf("notification alertmanager no config")
	}
	if len(notification.Alertmanager.Labels) == 0 {
		return fmt.Errorf("alertmanager at least one label pair required")
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
		return fmt.Errorf("no events were successfully received by alertmanager")
	}

	return nil
}

func (s alertmanagerService) sendOneTarget(ctx context.Context, target string, rawBody []byte) error {
	rawURL := fmt.Sprintf("%v://%v%v", s.opts.Scheme, target, s.opts.APIPath)

	idleConnTimeout, err := time.ParseDuration(s.opts.IdleConnTimeout)
	if err != nil {
		return fmt.Errorf("failed to parse idle connection timeout")
	}
	transport := httputil.NewTransport(rawURL, s.opts.MaxIdleConns, s.opts.MaxIdleConnsPerHost, s.opts.MaxConnsPerHost, idleConnTimeout, s.opts.InsecureSkipVerify)
	client := &http.Client{
		Transport: httputil.NewLoggingRoundTripper(transport, s.entry),
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
		return fmt.Errorf("unable to read response data: %v", err)
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
