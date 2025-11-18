package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	texttemplate "text/template"

	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"golang.org/x/time/rate"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
	slackutil "github.com/argoproj/notifications-engine/pkg/util/slack"
)

// No rate limit unless Slack requests it (allows for Slack to control bursting)
var slackState = slackutil.NewState(rate.NewLimiter(rate.Inf, 1))

// Cache for Slack API lookups to avoid repeated calls
type slackLookupCache struct {
	sync.RWMutex
	usersByEmail map[string]string
	channels     map[string]string
	userGroups   map[string]string
}

var globalLookupCache = &slackLookupCache{
	usersByEmail: make(map[string]string),
	channels:     make(map[string]string),
	userGroups:   make(map[string]string),
}

// Regex patterns to match special markers in messages
var (
	slackUserEmailPattern = regexp.MustCompile(`__SLACK_USER_EMAIL__(.+?)__`)
	slackChannelPattern   = regexp.MustCompile(`__SLACK_CHANNEL__(.+?)__`)
	slackUserGroupPattern = regexp.MustCompile(`__SLACK_USERGROUP__(.+?)__`)
)

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

	return func(notification *Notification, vars map[string]any) error {
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
	Username           string   `json:"username"`
	Icon               string   `json:"icon"`
	Token              string   `json:"token"`
	SigningSecret      string   `json:"signingSecret"`
	Channels           []string `json:"channels"`
	ApiURL             string   `json:"apiURL"`
	DisableUnfurl      bool     `json:"disableUnfurl"`
	InsecureSkipVerify bool     `json:"insecureSkipVerify"`
	httputil.TransportOptions
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
	client, err := newSlackClient(s.opts)
	if err != nil {
		return err
	}

	// Process Slack mentions in the message
	notification.Message = processSlackMentions(client, notification.Message)

	// Also process mentions in attachments and blocks if present
	if notification.Slack != nil {
		if notification.Slack.Attachments != "" {
			notification.Slack.Attachments = processSlackMentions(client, notification.Slack.Attachments)
		}
		if notification.Slack.Blocks != "" {
			notification.Slack.Blocks = processSlackMentions(client, notification.Slack.Blocks)
		}
	}

	slackNotification, msgOptions, err := buildMessageOptions(notification, s.opts)
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

	client, err := httputil.NewServiceHTTPClient(opts.TransportOptions, opts.InsecureSkipVerify, apiURL, "slack")
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

// lookupUserByEmail retrieves a Slack user ID by email address
func lookupUserByEmail(client *slack.Client, email string) (string, error) {
	// Check cache first
	globalLookupCache.RLock()
	if userID, ok := globalLookupCache.usersByEmail[email]; ok {
		globalLookupCache.RUnlock()
		return userID, nil
	}
	globalLookupCache.RUnlock()

	// Make API call
	user, err := client.GetUserByEmail(email)
	if err != nil {
		return "", fmt.Errorf("failed to lookup user by email %s: %w", email, err)
	}

	// Cache the result
	globalLookupCache.Lock()
	globalLookupCache.usersByEmail[email] = user.ID
	globalLookupCache.Unlock()

	return user.ID, nil
}

// lookupChannelByName retrieves a Slack channel ID by channel name
func lookupChannelByName(client *slack.Client, channelName string) (string, error) {
	// Normalize channel name (remove # if present)
	channelName = strings.TrimPrefix(channelName, "#")

	// Check cache first
	globalLookupCache.RLock()
	if channelID, ok := globalLookupCache.channels[channelName]; ok {
		globalLookupCache.RUnlock()
		return channelID, nil
	}
	globalLookupCache.RUnlock()

	// Make API call to get all channels
	// Implements pagination for workspaces with many channels
	var cursor string
	for {
		params := &slack.GetConversationsParameters{
			Cursor:          cursor,
			ExcludeArchived: true,
			Limit:           1000,
			Types:           []string{"public_channel", "private_channel"},
		}
		channels, nextCursor, err := client.GetConversations(params)
		if err != nil {
			return "", fmt.Errorf("failed to lookup channel %s: %w", channelName, err)
		}

		for _, channel := range channels {
			if channel.Name == channelName {
				// Cache the result
				globalLookupCache.Lock()
				globalLookupCache.channels[channelName] = channel.ID
				globalLookupCache.Unlock()
				return channel.ID, nil
			}
		}

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return "", fmt.Errorf("channel %s not found", channelName)
}

// lookupUserGroupByName retrieves a Slack user group ID by group name
func lookupUserGroupByName(client *slack.Client, groupName string) (string, error) {
	// Check cache first
	globalLookupCache.RLock()
	if groupID, ok := globalLookupCache.userGroups[groupName]; ok {
		globalLookupCache.RUnlock()
		return groupID, nil
	}
	globalLookupCache.RUnlock()

	// Make API call
	groups, err := client.GetUserGroups(slack.GetUserGroupsOptionIncludeDisabled(false))
	if err != nil {
		return "", fmt.Errorf("failed to lookup user group %s: %w", groupName, err)
	}

	for _, group := range groups {
		if group.Handle == groupName || group.Name == groupName {
			// Cache the result
			globalLookupCache.Lock()
			globalLookupCache.userGroups[groupName] = group.ID
			globalLookupCache.Unlock()
			return group.ID, nil
		}
	}

	return "", fmt.Errorf("user group %s not found", groupName)
}

// processSlackMentions processes the notification message and replaces special markers with actual Slack mentions
func processSlackMentions(client *slack.Client, message string) string {
	// Process user mentions by email
	message = slackUserEmailPattern.ReplaceAllStringFunc(message, func(match string) string {
		matches := slackUserEmailPattern.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}
		email := matches[1]
		userID, err := lookupUserByEmail(client, email)
		if err != nil {
			log.Warnf("Failed to lookup Slack user by email %s: %v", email, err)
			return match
		}
		return fmt.Sprintf("<@%s>", userID)
	})

	// Process channel mentions
	message = slackChannelPattern.ReplaceAllStringFunc(message, func(match string) string {
		matches := slackChannelPattern.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}
		channelName := matches[1]
		channelID, err := lookupChannelByName(client, channelName)
		if err != nil {
			log.Warnf("Failed to lookup Slack channel %s: %v", channelName, err)
			return match
		}
		return fmt.Sprintf("<#%s>", channelID)
	})

	// Process user group mentions
	message = slackUserGroupPattern.ReplaceAllStringFunc(message, func(match string) string {
		matches := slackUserGroupPattern.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}
		groupName := matches[1]
		groupID, err := lookupUserGroupByName(client, groupName)
		if err != nil {
			log.Warnf("Failed to lookup Slack user group %s: %v", groupName, err)
			return match
		}
		return fmt.Sprintf("<!subteam^%s>", groupID)
	})

	return message
}
