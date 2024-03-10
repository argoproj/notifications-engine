package services

import (
	"bytes"
	"context"
	texttemplate "text/template"

	slackutil "github.com/argoproj/notifications-engine/pkg/util/slack"
)

type DatadogNotification struct {
	Attachments     string                   `json:"attachments,omitempty"`
	Blocks          string                   `json:"blocks,omitempty"`
	GroupingKey     string                   `json:"groupingKey"`
	NotifyBroadcast bool                     `json:"notifyBroadcast"`
	DeliveryPolicy  slackutil.DeliveryPolicy `json:"deliveryPolicy"`
}

func (n *DatadogNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
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

type DatadogOptions struct {
	Username           string   `json:"username"`
	Icon               string   `json:"icon"`
	Token              string   `json:"token"`
	SigningSecret      string   `json:"signingSecret"`
	Channels           []string `json:"channels"`
	InsecureSkipVerify bool     `json:"insecureSkipVerify"`
	ApiURL             string   `json:"apiURL"`
	DisableUnfurl      bool     `json:"disableUnfurl"`
}

type datadogService struct {
	opts DatadogOptions
}

func NewDatadogService(opts DatadogOptions) NotificationService {
	return &datadogService{opts: opts}
}

func (s *datadogService) Send(notification Notification, dest Destination) error {
	slackNotification, msgOptions, err := buildMessageOptions(notification, dest, s.opts)
	if err != nil {
		return err
	}
	return slackutil.NewThreadedClient(
		newSlackClient(s.opts),
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
