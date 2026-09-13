package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	texttemplate "text/template"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
)

type MattermostNotification struct {
	Attachments    string                   `json:"attachments,omitempty"`
	GroupingKey    string                   `json:"groupingKey,omitempty"`
	DeliveryPolicy MattermostDeliveryPolicy `json:"deliveryPolicy,omitempty"`
}

type MattermostDeliveryPolicy string

const (
	MattermostPost   MattermostDeliveryPolicy = "Post"
	MattermostUpdate MattermostDeliveryPolicy = "Update"
)

func (p *MattermostDeliveryPolicy) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	switch MattermostDeliveryPolicy(value) {
	case "", MattermostPost, MattermostUpdate:
		*p = MattermostDeliveryPolicy(value)
		return nil
	default:
		return fmt.Errorf("unsupported Mattermost delivery policy %q", value)
	}
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
		notification.Mattermost.DeliveryPolicy = n.DeliveryPolicy
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
	updateState *mattermostUpdateState
}

type mattermostUpdate struct {
	mutex  sync.Mutex
	postID string
}

type mattermostUpdateKey [3]string

type mattermostUpdateState struct {
	mutex   sync.Mutex
	updates map[mattermostUpdateKey]*mattermostUpdate
}

func newMattermostUpdateState() *mattermostUpdateState {
	return &mattermostUpdateState{updates: map[mattermostUpdateKey]*mattermostUpdate{}}
}

func (s *mattermostUpdateState) get(apiURL, channelID, groupingKey string) *mattermostUpdate {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	key := mattermostUpdateKey{apiURL, channelID, groupingKey}
	update, ok := s.updates[key]
	if !ok {
		update = &mattermostUpdate{}
		s.updates[key] = update
	}
	return update
}

var globalMattermostUpdateState = newMattermostUpdateState()

func NewMattermostService(opts MattermostOptions) NotificationService {
	return newMattermostService(opts, globalMattermostUpdateState)
}

func newMattermostService(opts MattermostOptions, updateState *mattermostUpdateState) NotificationService {
	return &mattermostService{opts: opts, updateState: updateState}
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

	deliveryPolicy := MattermostPost
	groupingKey := ""
	if notification.Mattermost != nil {
		groupingKey = notification.Mattermost.GroupingKey
		if notification.Mattermost.DeliveryPolicy != "" {
			deliveryPolicy = notification.Mattermost.DeliveryPolicy
		}
	}
	if deliveryPolicy != MattermostPost && deliveryPolicy != MattermostUpdate {
		return fmt.Errorf("unsupported Mattermost delivery policy %q", deliveryPolicy)
	}
	if deliveryPolicy == MattermostPost || groupingKey == "" {
		_, err = m.post(client, notification.Message, attachments, dest.Recipient, false)
		return err
	}

	update := m.updateState.get(m.opts.ApiURL, dest.Recipient, groupingKey)
	update.mutex.Lock()
	defer update.mutex.Unlock()
	if update.postID == "" {
		postID, err := m.post(client, notification.Message, attachments, dest.Recipient, true)
		if err != nil {
			return err
		}
		update.postID = postID
		return nil
	}
	return m.update(client, notification.Message, attachments, update.postID)
}

func (m *mattermostService) post(client *http.Client, message string, attachments []any, channelID string, requireID bool) (string, error) {
	body := map[string]any{
		"channel_id": channelID,
		"message":    message,
		"props": map[string]any{
			"attachments": attachments,
		},
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

func (m *mattermostService) update(client *http.Client, message string, attachments []any, postID string) error {
	postURL := m.opts.ApiURL + "/api/v4/posts/" + url.PathEscape(postID)
	req, err := http.NewRequest(http.MethodGet, postURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.opts.Token)
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request: %w", err)
	}
	data, readErr := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if readErr != nil {
		return fmt.Errorf("failed to read body: %w", readErr)
	}
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("request to Mattermost post %q has failed with error code %d : %s", postID, res.StatusCode, string(data))
	}
	var post struct {
		Props map[string]json.RawMessage `json:"props"`
	}
	if err := json.Unmarshal(data, &post); err != nil {
		return fmt.Errorf("failed to unmarshal Mattermost post response: %w", err)
	}
	if post.Props == nil {
		post.Props = map[string]json.RawMessage{}
	}
	attachmentsJSON, err := json.Marshal(attachments)
	if err != nil {
		return fmt.Errorf("failed to marshal Mattermost attachments: %w", err)
	}
	post.Props["attachments"] = attachmentsJSON
	patchBody, err := json.Marshal(map[string]any{"message": message, "props": post.Props})
	if err != nil {
		return fmt.Errorf("failed to marshal Mattermost post patch: %w", err)
	}
	patchReq, err := http.NewRequest(http.MethodPut, postURL+"/patch", bytes.NewReader(patchBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.Header.Set("Authorization", "Bearer "+m.opts.Token)
	patchRes, err := client.Do(patchReq)
	if err != nil {
		return fmt.Errorf("failed to request: %w", err)
	}
	defer patchRes.Body.Close()
	patchData, err := io.ReadAll(patchRes.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	if patchRes.StatusCode/100 != 2 {
		return fmt.Errorf("request to Mattermost post %q has failed with error code %d : %s", postID, patchRes.StatusCode, string(patchData))
	}
	return nil
}
