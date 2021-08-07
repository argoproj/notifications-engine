package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	texttemplate "text/template"
	"time"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
	log "github.com/sirupsen/logrus"
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
	EndsAt       time.Time         `json:"endsAt"`
}

// AlertmanagerOptions cluster configuration
type AlertmanagerOptions struct {
	Targets            []string   `json:"targets"`
	Scheme             string     `json:"scheme"`
	APIPath            string     `json:"apiPath"`
	BasicAuth          *BasicAuth `json:"basicAuth"`
	BearerToken        string     `json:"bearerToken"`
	InsecureSkipVerify bool       `json:"insecureSkipVerify"`
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
		// generatorURL must url format
		if _, err := url.Parse(notification.Alertmanager.GeneratorURL); err != nil {
			return err
		}

		if len(n.Labels) > 0 {
			notification.Alertmanager.Labels = n.Labels
			if err := notification.Alertmanager.parseLabels(name, f, vars); err != nil {
				return err
			}
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
	rawBody, err := json.Marshal([]*AlertmanagerNotification{notification.Alertmanager})
	if err != nil {
		return err
	}

	for _, target := range s.opts.Targets {
		go func(target string) {
			if err := s.sendOneTarget(target, rawBody); err != nil {
				s.entry.Errorf("alertmanager sent target: %v", err)
			}
		}(target)
	}

	return nil
}

func (s alertmanagerService) sendOneTarget(target string, rawBody []byte) error {
	rawURL := fmt.Sprintf("%v://%v%v", s.opts.Scheme, target, s.opts.APIPath)

	transport := httputil.NewTransport(rawURL, s.opts.InsecureSkipVerify)
	client := &http.Client{
		Transport: httputil.NewLoggingRoundTripper(transport, s.entry),
	}

	req, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewReader(rawBody))
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

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("unable to read response data: %v", err)
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("request to %s has failed with error code %d : %s", rawURL, response.StatusCode, string(data))
	}

	return nil
}
