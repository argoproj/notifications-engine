package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	texttemplate "text/template"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
)

type MattermostNotification struct {
	Attachments string `json:"attachments,omitempty"`
}

func (n *MattermostNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	mattermostAttachments, err := texttemplate.New(name).Funcs(f).Parse(n.Attachments)
	if err != nil {
		return nil, err
	}
	return func(notification *Notification, vars map[string]interface{}) error {
		if notification.Mattermost == nil {
			notification.Mattermost = &MattermostNotification{}
		}
		var mattermostAttachmentsData bytes.Buffer
		if err := mattermostAttachments.Execute(&mattermostAttachmentsData, vars); err != nil {
			return err
		}

		notification.Mattermost.Attachments = mattermostAttachmentsData.String()
		return nil
	}, nil
}

type MattermostOptions struct {
	ApiURL             string `json:"apiURL"`
	Token              string `json:"token"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	httputil.TransportOptions
}

type mattermostService struct {
	opts MattermostOptions
}

func NewMattermostService(opts MattermostOptions) NotificationService {
	return &mattermostService{opts: opts}
}

func (m *mattermostService) Send(notification Notification, dest Destination) (err error) {
	client, err := httputil.NewServiceHTTPClient(m.opts.TransportOptions, m.opts.InsecureSkipVerify, m.opts.ApiURL, "mattermost")
	if err != nil {
		return err
	}

	attachments := []interface{}{}
	if notification.Mattermost != nil {
		if notification.Mattermost.Attachments != "" {
			if err := json.Unmarshal([]byte(notification.Mattermost.Attachments), &attachments); err != nil {
				return fmt.Errorf("failed to unmarshal attachments '%s' : %w", notification.Mattermost.Attachments, err)
			}
		}
	}

	body := map[string]interface{}{
		"channel_id": dest.Recipient,
		"message":    notification.Message,
		"props": map[string]interface{}{
			"attachments": attachments,
		},
	}
	b, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, m.opts.ApiURL+"/api/v4/posts", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.opts.Token))

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request: %w", err)
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	if res.StatusCode/100 != 2 {
		return fmt.Errorf("request to %s has failed with error code %d : %s", body, res.StatusCode, string(data))
	}

	return nil
}
