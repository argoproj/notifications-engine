package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	texttemplate "text/template"

	"github.com/RocketChat/Rocket.Chat.Go.SDK/models"
	"github.com/RocketChat/Rocket.Chat.Go.SDK/rest"
	log "github.com/sirupsen/logrus"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
)

type RocketChatNotification struct {
	Attachments string `json:"attachments,omitempty"`
}

func (n *RocketChatNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	rocketChatAttachments, err := texttemplate.New(name).Funcs(f).Parse(n.Attachments)
	if err != nil {
		return nil, err
	}
	return func(notification *Notification, vars map[string]any) error {
		if notification.RocketChat == nil {
			notification.RocketChat = &RocketChatNotification{}
		}
		var rocketChatAttachmentsData bytes.Buffer
		if err := rocketChatAttachments.Execute(&rocketChatAttachmentsData, vars); err != nil {
			return err
		}

		notification.RocketChat.Attachments = rocketChatAttachmentsData.String()

		return nil
	}, nil
}

type RocketChatOptions struct {
	Alias      string `json:"alias"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	Icon       string `json:"icon"`
	Avatar     string `json:"avatar"`
	ServerUrl  string `json:"serverUrl"`
	WebhookUrl string `json:"webhookUrl"`

	InsecureSkipVerify bool `json:"insecureSkipVerify"`
	httputil.TransportOptions
}

type rocketChatService struct {
	opts RocketChatOptions
}

var validEmoji = regexp.MustCompile(`^:.+:$`)

func NewRocketChatService(opts RocketChatOptions) NotificationService {
	return &rocketChatService{opts: opts}
}

func (r *rocketChatService) Send(notification Notification, dest Destination) error {
	if r.opts.WebhookUrl != "" {
		return r.sendWebhook(notification, dest)
	}

	serverUrl, err := url.Parse(r.opts.ServerUrl)
	if err != nil {
		return err
	}

	rl := rest.NewClient(serverUrl, false)

	credentials := models.UserCredentials{Email: r.opts.Email, Password: r.opts.Password}
	err = rl.Login(&credentials)
	if err != nil {
		return err
	}

	message := models.PostMessage{Alias: r.opts.Alias, Text: notification.Message}
	// It's a channel
	if strings.HasPrefix(dest.Recipient, "#") || strings.HasPrefix(dest.Recipient, "@") {
		message.Channel = dest.Recipient
	} else {
		message.RoomID = dest.Recipient
	}
	if r.opts.Icon != "" {
		if validEmoji.MatchString(r.opts.Icon) {
			message.Emoji = r.opts.Icon
		} else {
			log.Warnf("Icon reference '%v' is not a valid emoji", r.opts.Icon)
		}
	}
	if r.opts.Avatar != "" {
		if isValidAvatarURL(r.opts.Avatar) {
			message.Avatar = r.opts.Avatar
		} else {
			log.Warnf("Avatar reference '%v' is not a valid URL", r.opts.Avatar)
		}
	}

	if notification.RocketChat != nil {
		attachments := make([]models.Attachment, 0)
		if notification.RocketChat.Attachments != "" {
			if err := json.Unmarshal([]byte(notification.RocketChat.Attachments), &attachments); err != nil {
				return fmt.Errorf("failed to unmarshal attachments '%s' : %w", notification.RocketChat.Attachments, err)
			}
		}

		message.Attachments = attachments
	}

	postMessage, err := rl.PostMessage(&message)
	if err != nil {
		return err
	}
	if !postMessage.Success {
		return errors.New(postMessage.Error)
	}

	return err
}

type rocketChatWebhookPayload struct {
	Text        string              `json:"text"`
	Alias       string              `json:"alias,omitempty"`
	Emoji       string              `json:"emoji,omitempty"`
	Avatar      string              `json:"avatar,omitempty"`
	Channel     string              `json:"channel,omitempty"`
	Attachments []models.Attachment `json:"attachments,omitempty"`
}

func (r *rocketChatService) sendWebhook(notification Notification, dest Destination) error {
	payload := rocketChatWebhookPayload{
		Text:  notification.Message,
		Alias: r.opts.Alias,
	}

	if strings.HasPrefix(dest.Recipient, "#") || strings.HasPrefix(dest.Recipient, "@") {
		payload.Channel = dest.Recipient
	} else if dest.Recipient != "" {
		payload.Channel = "#" + dest.Recipient
	}

	if r.opts.Icon != "" {
		if validEmoji.MatchString(r.opts.Icon) {
			payload.Emoji = r.opts.Icon
		} else {
			log.Warnf("Icon reference '%v' is not a valid emoji", r.opts.Icon)
		}
	}
	if r.opts.Avatar != "" {
		if isValidAvatarURL(r.opts.Avatar) {
			payload.Avatar = r.opts.Avatar
		} else {
			log.Warnf("Avatar reference '%v' is not a valid URL", r.opts.Avatar)
		}
	}

	if notification.RocketChat != nil && notification.RocketChat.Attachments != "" {
		attachments := make([]models.Attachment, 0)
		if err := json.Unmarshal([]byte(notification.RocketChat.Attachments), &attachments); err != nil {
			return fmt.Errorf("failed to unmarshal attachments '%s' : %w", notification.RocketChat.Attachments, err)
		}
		payload.Attachments = attachments
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client, err := httputil.NewServiceHTTPClient(r.opts.TransportOptions, r.opts.InsecureSkipVerify, r.opts.WebhookUrl, "rocketchat")
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, r.opts.WebhookUrl, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	if res.StatusCode/100 != 2 {
		return fmt.Errorf("rocketchat webhook post error %d : %s", res.StatusCode, string(data))
	}

	return nil
}

func isValidAvatarURL(iconURL string) bool {
	_, err := url.ParseRequestURI(iconURL)
	if err != nil {
		return false
	}

	u, err := url.Parse(iconURL)
	if err != nil || (u.Scheme == "" || (u.Scheme != "http" && u.Scheme != "https")) || u.Host == "" {
		return false
	}

	return true
}
