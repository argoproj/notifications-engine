package services

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	log "github.com/sirupsen/logrus"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
	"github.com/argoproj/notifications-engine/pkg/util/text"
)

type WebhookNotification struct {
	Method string `json:"method"`
	Body   string `json:"body"`
	Path   string `json:"path"`
}

type WebhookNotifications map[string]WebhookNotification

type compiledWebhookTemplate struct {
	body   *texttemplate.Template
	path   *texttemplate.Template
	method string
}

func (n WebhookNotifications) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	webhooks := map[string]compiledWebhookTemplate{}
	for k, v := range n {
		body, err := texttemplate.New(name + k).Funcs(f).Parse(v.Body)
		if err != nil {
			return nil, err
		}
		path, err := texttemplate.New(name + k).Funcs(f).Parse(v.Path)
		if err != nil {
			return nil, err
		}
		webhooks[k] = compiledWebhookTemplate{body: body, method: v.Method, path: path}
	}
	return func(notification *Notification, vars map[string]interface{}) error {
		for k, v := range webhooks {
			if notification.Webhook == nil {
				notification.Webhook = map[string]WebhookNotification{}
			}
			var body bytes.Buffer
			err := webhooks[k].body.Execute(&body, vars)
			if err != nil {
				return err
			}
			var path bytes.Buffer
			err = webhooks[k].path.Execute(&path, vars)
			if err != nil {
				return err
			}
			notification.Webhook[k] = WebhookNotification{
				Method: v.method,
				Body:   body.String(),
				Path:   path.String(),
			}
		}
		return nil
	}, nil
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type BasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type WebhookOptions struct {
	URL                 string        `json:"url"`
	Headers             []Header      `json:"headers"`
	BasicAuth           *BasicAuth    `json:"basicAuth"`
	RetryWaitMin        time.Duration `json:"retryWaitMin"`
	RetryWaitMax        time.Duration `json:"retryWaitMax"`
	RetryMax            int           `json:"retryMax"`
	InsecureSkipVerify  bool          `json:"insecureSkipVerify"`
	MaxIdleConns        int           `json:"maxIdleConns"`
	MaxIdleConnsPerHost int           `json:"maxIdleConnsPerHost"`
	MaxConnsPerHost     int           `json:"maxConnsPerHost"`
	IdleConnTimeout     string        `json:"idleConnTimeout"`
}

func NewWebhookService(opts WebhookOptions) NotificationService {
	// Set default values if fields are zero
	if opts.RetryWaitMin == 0 {
		opts.RetryWaitMin = 1 * time.Second
	}
	if opts.RetryWaitMax == 0 {
		opts.RetryWaitMax = 5 * time.Second
	}
	if opts.RetryMax == 0 {
		opts.RetryMax = 3
	}
	return &webhookService{opts: opts}
}

type webhookService struct {
	opts WebhookOptions
}

func (s webhookService) Send(notification Notification, dest Destination) error {
	request := request{
		body:        notification.Message,
		method:      http.MethodGet,
		url:         s.opts.URL,
		destService: dest.Service,
	}

	if webhookNotification, ok := notification.Webhook[dest.Service]; ok {
		request.applyOverridesFrom(webhookNotification)
	}

	resp, err := request.execute(&s)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !(resp.StatusCode >= 200 && resp.StatusCode <= 299) {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			data = []byte(fmt.Sprintf("unable to read response data: %v", err))
		}
		return fmt.Errorf("request to %s has failed with error code %d : %s", request, resp.StatusCode, string(data))
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}

	return nil
}

type request struct {
	body        string
	method      string
	url         string
	destService string
}

func (r *request) applyOverridesFrom(notification WebhookNotification) {
	r.body = notification.Body
	r.method = text.Coalesce(notification.Method, r.method)
	if notification.Path != "" {
		r.url = strings.TrimRight(r.url, "/") + "/" + strings.TrimLeft(notification.Path, "/")
	}
}

func (r *request) intoRetryableHttpRequest(service *webhookService) (*retryablehttp.Request, error) {
	retryReq, err := retryablehttp.NewRequest(r.method, r.url, bytes.NewBufferString(r.body))
	if err != nil {
		return nil, err
	}
	for _, header := range service.opts.Headers {
		retryReq.Header.Set(header.Name, header.Value)
	}
	if service.opts.BasicAuth != nil {
		retryReq.SetBasicAuth(service.opts.BasicAuth.Username, service.opts.BasicAuth.Password)
	}
	return retryReq, nil
}

func (r *request) execute(service *webhookService) (*http.Response, error) {
	req, err := r.intoRetryableHttpRequest(service)
	if err != nil {
		return nil, err
	}

	idleConnTimeout, err := time.ParseDuration(service.opts.IdleConnTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to parse idle connection timeout")
	}
	transport := httputil.NewLoggingRoundTripper(
		httputil.NewTransport(r.url, service.opts.MaxIdleConns, service.opts.MaxIdleConnsPerHost, service.opts.MaxConnsPerHost, idleConnTimeout, service.opts.InsecureSkipVerify),
		log.WithField("service", r.destService))

	client := retryablehttp.NewClient()
	client.HTTPClient = &http.Client{
		Transport: transport,
	}
	client.RetryWaitMin = service.opts.RetryWaitMin
	client.RetryWaitMax = service.opts.RetryWaitMax
	client.RetryMax = service.opts.RetryMax

	return client.Do(req)
}
