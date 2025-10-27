package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	texttemplate "text/template"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
	slackutil "github.com/argoproj/notifications-engine/pkg/util/slack"

	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"golang.org/x/time/rate"
)

// No rate limit unless Slack requests it (allows for Slack to control bursting)
var slackState = slackutil.NewState(rate.NewLimiter(rate.Inf, 1))

type SlackNotification struct {
	Username        string                   `json:"username,omitempty"`
	Icon            string                   `json:"icon,omitempty"`
	Attachments     string                   `json:"attachments,omitempty"`
	Blocks          string                   `json:"blocks,omitempty"`
	GroupingKey     string                   `json:"groupingKey"`
	NotifyBroadcast bool                     `json:"notifyBroadcast"`
	DeliveryPolicy  slackutil.DeliveryPolicy `json:"deliveryPolicy"`
}

func (n *SlackNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	slackUsername, err := texttemplate.New(name).Funcs(f).Parse(n.Username)
	if err != nil {
		return nil, err
	}

	slackIcon, err := texttemplate.New(name).Funcs(f).Parse(n.Icon)
	if err != nil {
		return nil, err
	}

	slackAttachments, err := texttemplate.New(name).Funcs(f).Parse(n.Attachments)
	if err != nil {
		return nil, err
	}
	slackBlocks, err := texttemplate.New(name).Funcs(f).Parse(n.Blocks)
	if err != nil {
		return nil, err
	}
	groupingKey, err := texttemplate.New(name).Funcs(f).Parse(n.GroupingKey)
	if err != nil {
		return nil, err
	}

	return func(notification *Notification, vars map[string]interface{}) error {
		if notification.Slack == nil {
			notification.Slack = &SlackNotification{}
		}
		var slackUsernameData bytes.Buffer
		if err := slackUsername.Execute(&slackUsernameData, vars); err != nil {
			return err
		}
		notification.Slack.Username = slackUsernameData.String()

		var slackIconData bytes.Buffer
		if err := slackIcon.Execute(&slackIconData, vars); err != nil {
			return err
		}
		notification.Slack.Icon = slackIconData.String()

		var slackAttachmentsData bytes.Buffer
		if err := slackAttachments.Execute(&slackAttachmentsData, vars); err != nil {
			return err
		}
		notification.Slack.Attachments = slackAttachmentsData.String()

		var slackBlocksData bytes.Buffer
		if err := slackBlocks.Execute(&slackBlocksData, vars); err != nil {
			return err
		}
		notification.Slack.Blocks = slackBlocksData.String()

		var groupingKeyData bytes.Buffer
		if err := groupingKey.Execute(&groupingKeyData, vars); err != nil {
			return err
		}
		notification.Slack.GroupingKey = groupingKeyData.String()

		notification.Slack.NotifyBroadcast = n.NotifyBroadcast
		notification.Slack.DeliveryPolicy = n.DeliveryPolicy
		return nil
	}, nil
}

type SlackOptions struct {
	Username            string   `json:"username"`
	Icon                string   `json:"icon"`
	Token               string   `json:"token"`
	SigningSecret       string   `json:"signingSecret"`
	Channels            []string `json:"channels"`
	ApiURL              string   `json:"apiURL"`
	DisableUnfurl       bool     `json:"disableUnfurl"`
	InsecureSkipVerify  bool     `json:"insecureSkipVerify"`
	MaxIdleConns        int      `json:"maxIdleConns"`
	MaxIdleConnsPerHost int      `json:"maxIdleConnsPerHost"`
	MaxConnsPerHost     int      `json:"maxConnsPerHost"`
	IdleConnTimeout     string   `json:"idleConnTimeout"`
}

type slackService struct {
	opts SlackOptions
}

var validIconEmoji = regexp.MustCompile(`^:.+:$`)

func NewSlackService(opts SlackOptions) NotificationService {
	return &slackService{opts: opts}
}

func buildMessageOptions(notification Notification, opts SlackOptions) (*SlackNotification, []slack.MsgOption, error) {
	msgOptions := []slack.MsgOption{slack.MsgOptionText(notification.Message, false)}
	slackNotification := &SlackNotification{}

	if notification.Slack != nil && notification.Slack.Username != "" {
		msgOptions = append(msgOptions, slack.MsgOptionUsername(notification.Slack.Username))
	} else if opts.Username != "" {
		msgOptions = append(msgOptions, slack.MsgOptionUsername(opts.Username))
	}

	if opts.Icon != "" || (notification.Slack != nil && notification.Slack.Icon != "") {
		var icon string
		if notification.Slack != nil && notification.Slack.Icon != "" {
			icon = notification.Slack.Icon
		} else {
			icon = opts.Icon
		}

		switch {
		case validIconEmoji.MatchString(icon):
			msgOptions = append(msgOptions, slack.MsgOptionIconEmoji(icon))
		case isValidIconURL(icon):
			msgOptions = append(msgOptions, slack.MsgOptionIconURL(icon))
		default:
			log.Warnf("Icon reference '%v' is not a valid emoji or url", icon)
		}
	}

	if notification.Slack != nil {
		attachments := make([]slack.Attachment, 0)
		if notification.Slack.Attachments != "" {
			if err := json.Unmarshal([]byte(notification.Slack.Attachments), &attachments); err != nil {
				return nil, nil, fmt.Errorf("failed to unmarshal attachments '%s' : %w", notification.Slack.Attachments, err)
			}
		}

		blocks := slack.Blocks{}
		if notification.Slack.Blocks != "" {
			if err := json.Unmarshal([]byte(notification.Slack.Blocks), &blocks); err != nil {
				return nil, nil, fmt.Errorf("failed to unmarshal blocks '%s' : %w", notification.Slack.Blocks, err)
			}
		}
		msgOptions = append(msgOptions, slack.MsgOptionAttachments(attachments...), slack.MsgOptionBlocks(blocks.BlockSet...))
		slackNotification = notification.Slack
	}

	if opts.DisableUnfurl {
		msgOptions = append(msgOptions, slack.MsgOptionDisableLinkUnfurl(), slack.MsgOptionDisableMediaUnfurl())
	}

	return slackNotification, msgOptions, nil
}

func (s *slackService) Send(notification Notification, dest Destination) error {
	slackNotification, msgOptions, err := buildMessageOptions(notification, s.opts)
	if err != nil {
		return err
	}
	client, err := newSlackClient(s.opts)
	if err != nil {
		return err
	}
	return slackutil.NewThreadedClient(
		client,
		slackState,
	).SendMessage(
		context.TODO(),
		dest.Recipient,
		slackNotification.GroupingKey,
		slackNotification.NotifyBroadcast,
		slackNotification.DeliveryPolicy,
		msgOptions,
	)
}

// GetSigningSecret exposes signing secret for slack bot
func (s *slackService) GetSigningSecret() string {
	return s.opts.SigningSecret
}

func newSlackClient(opts SlackOptions) (slackclient *slack.Client, err error) {
	apiURL := slack.APIURL
	if opts.ApiURL != "" {
		apiURL = opts.ApiURL
	}
	client, err := httputil.NewServiceHTTPClient(opts.MaxIdleConns, opts.MaxIdleConnsPerHost, opts.MaxConnsPerHost, opts.IdleConnTimeout, opts.InsecureSkipVerify, apiURL, "slack")
	if err != nil {
		return nil, err
	}
	return slack.New(opts.Token, slack.OptionHTTPClient(client), slack.OptionAPIURL(apiURL)), nil
}

func isValidIconURL(iconURL string) bool {
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
