package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	texttemplate "text/template"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
)

type MattermostNotification struct {
	Attachments string `json:"attachments,omitempty"`
	GroupingKey string `json:"groupingKey,omitempty"`
}

func (n *MattermostNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	mattermostAttachments, err := texttemplate.New(name).Funcs(f).Parse(n.Attachments)
	if err != nil {
		return nil, err
	}
	groupingKey, err := texttemplate.New(name).Funcs(f).Parse(n.GroupingKey)
	if err != nil {
		return nil, err
	}
	return func(notification *Notification, vars map[string]any) error {
		if notification.Mattermost == nil {
			notification.Mattermost = &MattermostNotification{}
		}
		var mattermostAttachmentsData bytes.Buffer
		if err := mattermostAttachments.Execute(&mattermostAttachmentsData, vars); err != nil {
			return err
		}

		notification.Mattermost.Attachments = mattermostAttachmentsData.String()

		var groupingKeyData bytes.Buffer
		if err := groupingKey.Execute(&groupingKeyData, vars); err != nil {
			return err
		}
		notification.Mattermost.GroupingKey = groupingKeyData.String()
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
	opts        MattermostOptions
	threadState *mattermostThreadState
}

var globalMattermostThreadState = newMattermostThreadState()

func NewMattermostService(opts MattermostOptions) NotificationService {
	return newMattermostService(opts, globalMattermostThreadState)
}

func newMattermostService(opts MattermostOptions, threadState *mattermostThreadState) NotificationService {
	return &mattermostService{opts: opts, threadState: threadState}
}

type mattermostThread struct {
	mutex  sync.Mutex
	rootID string
}

type mattermostThreadState struct {
	mutex   sync.Mutex
	threads map[string]map[string]map[string]*mattermostThread
}

func newMattermostThreadState() *mattermostThreadState {
	return &mattermostThreadState{threads: map[string]map[string]map[string]*mattermostThread{}}
}

func (s *mattermostThreadState) get(apiURL, channelID, groupingKey string) *mattermostThread {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	channels, ok := s.threads[apiURL]
	if !ok {
		channels = map[string]map[string]*mattermostThread{}
		s.threads[apiURL] = channels
	}
	groupingKeys, ok := channels[channelID]
	if !ok {
		groupingKeys = map[string]*mattermostThread{}
		channels[channelID] = groupingKeys
	}
	thread, ok := groupingKeys[groupingKey]
	if !ok {
		thread = &mattermostThread{}
		groupingKeys[groupingKey] = thread
	}
	return thread
}

func (m *mattermostService) Send(notification Notification, dest Destination) error {
	client, err := httputil.NewServiceHTTPClient(m.opts.TransportOptions, m.opts.InsecureSkipVerify, m.opts.ApiURL, "mattermost")
	if err != nil {
		return err
	}

	attachments := []any{}
	if notification.Mattermost != nil {
		if notification.Mattermost.Attachments != "" {
			if err := json.Unmarshal([]byte(notification.Mattermost.Attachments), &attachments); err != nil {
				return fmt.Errorf("failed to unmarshal attachments '%s' : %w", notification.Mattermost.Attachments, err)
			}
		}
	}

	groupingKey := ""
	if notification.Mattermost != nil {
		groupingKey = notification.Mattermost.GroupingKey
	}
	if groupingKey == "" {
		_, err = m.post(client, notification.Message, attachments, dest.Recipient, "", false)
		return err
	}

	thread := m.threadState.get(m.opts.ApiURL, dest.Recipient, groupingKey)
	thread.mutex.Lock()
	defer thread.mutex.Unlock()

	rootID, err := m.post(client, notification.Message, attachments, dest.Recipient, thread.rootID, thread.rootID == "")
	if err != nil {
		return err
	}
	if thread.rootID == "" {
		thread.rootID = rootID
	}
	return nil
}

func (m *mattermostService) post(client *http.Client, message string, attachments []any, channelID, rootID string, requireID bool) (string, error) {
	body := map[string]any{
		"channel_id": channelID,
		"message":    message,
		"props": map[string]any{
			"attachments": attachments,
		},
	}
	if rootID != "" {
		body["root_id"] = rootID
	}
	b, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, m.opts.ApiURL+"/api/v4/posts", bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.opts.Token)

	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request: %w", err)
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	if res.StatusCode/100 != 2 {
		return "", fmt.Errorf("request to %s has failed with error code %d : %s", body, res.StatusCode, string(data))
	}

	if !requireID {
		return "", nil
	}
	var post struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &post); err != nil {
		return "", fmt.Errorf("failed to unmarshal Mattermost post response: %w", err)
	}
	if post.ID == "" {
		return "", errors.New("mattermost post response does not contain an id")
	}
	return post.ID, nil
}
