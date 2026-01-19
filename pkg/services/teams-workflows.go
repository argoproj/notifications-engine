package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	texttemplate "text/template"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
)

// WorkflowsURLPattern matches Microsoft Teams Workflows webhook URLs (Power Automate)
// Matches: api.powerautomate.com, api.powerplatform.com, flow.microsoft.com, or webhook.office.com/workflows
// Also matches URLs with /powerautomate/ in the path (Power Platform URLs)
var WorkflowsURLPattern = regexp.MustCompile(`(?i)(api\.(powerautomate|powerplatform)\.com|flow\.microsoft\.com|webhook\.office\.com/workflows|/powerautomate/)`)

type TeamsWorkflowsNotification struct {
	Template        string `json:"template,omitempty"`
	Title           string `json:"title,omitempty"`
	Summary         string `json:"summary,omitempty"`
	Text            string `json:"text,omitempty"`
	ThemeColor      string `json:"themeColor,omitempty"`
	Facts           string `json:"facts,omitempty"`
	Sections        string `json:"sections,omitempty"`
	PotentialAction string `json:"potentialAction,omitempty"`
	// AdaptiveCard specifies the Adaptive Card JSON payload
	AdaptiveCard string `json:"adaptiveCard,omitempty"`
}

func (n *TeamsWorkflowsNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	template, err := texttemplate.New(name).Funcs(f).Parse(n.Template)
	if err != nil {
		return nil, fmt.Errorf("error in '%s' teams-workflows.template : %w", name, err)
	}

	title, err := texttemplate.New(name).Funcs(f).Parse(n.Title)
	if err != nil {
		return nil, fmt.Errorf("error in '%s' teams-workflows.title : %w", name, err)
	}

	summary, err := texttemplate.New(name).Funcs(f).Parse(n.Summary)
	if err != nil {
		return nil, fmt.Errorf("error in '%s' teams-workflows.summary : %w", name, err)
	}

	text, err := texttemplate.New(name).Funcs(f).Parse(n.Text)
	if err != nil {
		return nil, fmt.Errorf("error in '%s' teams-workflows.text : %w", name, err)
	}

	themeColor, err := texttemplate.New(name).Funcs(f).Parse(n.ThemeColor)
	if err != nil {
		return nil, fmt.Errorf("error in '%s' teams-workflows.themeColor: %w", name, err)
	}

	facts, err := texttemplate.New(name).Funcs(f).Parse(n.Facts)
	if err != nil {
		return nil, fmt.Errorf("error in '%s' teams-workflows.facts : %w", name, err)
	}

	sections, err := texttemplate.New(name).Funcs(f).Parse(n.Sections)
	if err != nil {
		return nil, fmt.Errorf("error in '%s' teams-workflows.sections : %w", name, err)
	}

	potentialActions, err := texttemplate.New(name).Funcs(f).Parse(n.PotentialAction)
	if err != nil {
		return nil, fmt.Errorf("error in '%s' teams-workflows.potentialAction: %w", name, err)
	}

	adaptiveCard, err := texttemplate.New(name).Funcs(f).Parse(n.AdaptiveCard)
	if err != nil {
		return nil, fmt.Errorf("error in '%s' teams-workflows.adaptiveCard: %w", name, err)
	}

	return func(notification *Notification, vars map[string]any) error {
		if notification.TeamsWorkflows == nil {
			notification.TeamsWorkflows = &TeamsWorkflowsNotification{}
		}

		var templateBuff bytes.Buffer
		if err := template.Execute(&templateBuff, vars); err != nil {
			return err
		}
		if val := templateBuff.String(); val != "" {
			notification.TeamsWorkflows.Template = val
		}

		var titleBuff bytes.Buffer
		if err := title.Execute(&titleBuff, vars); err != nil {
			return err
		}
		if val := titleBuff.String(); val != "" {
			notification.TeamsWorkflows.Title = val
		}

		var summaryBuff bytes.Buffer
		if err := summary.Execute(&summaryBuff, vars); err != nil {
			return err
		}
		if val := summaryBuff.String(); val != "" {
			notification.TeamsWorkflows.Summary = val
		}

		var textBuff bytes.Buffer
		if err := text.Execute(&textBuff, vars); err != nil {
			return err
		}
		if val := textBuff.String(); val != "" {
			notification.TeamsWorkflows.Text = val
		}

		var themeColorBuff bytes.Buffer
		if err := themeColor.Execute(&themeColorBuff, vars); err != nil {
			return err
		}
		if val := themeColorBuff.String(); val != "" {
			notification.TeamsWorkflows.ThemeColor = val
		}

		var factsData bytes.Buffer
		if err := facts.Execute(&factsData, vars); err != nil {
			return err
		}
		if val := factsData.String(); val != "" {
			notification.TeamsWorkflows.Facts = val
		}

		var sectionsBuff bytes.Buffer
		if err := sections.Execute(&sectionsBuff, vars); err != nil {
			return err
		}
		if val := sectionsBuff.String(); val != "" {
			notification.TeamsWorkflows.Sections = val
		}

		var actionsData bytes.Buffer
		if err := potentialActions.Execute(&actionsData, vars); err != nil {
			return err
		}
		if val := actionsData.String(); val != "" {
			notification.TeamsWorkflows.PotentialAction = val
		}

		var adaptiveCardBuff bytes.Buffer
		if err := adaptiveCard.Execute(&adaptiveCardBuff, vars); err != nil {
			return err
		}
		if val := adaptiveCardBuff.String(); val != "" {
			notification.TeamsWorkflows.AdaptiveCard = val
		}

		return nil
	}, nil
}

type TeamsWorkflowsOptions struct {
	RecipientUrls      map[string]string `json:"recipientUrls"`
	InsecureSkipVerify bool              `json:"insecureSkipVerify"`
	httputil.TransportOptions
}

type teamsWorkflowsService struct {
	opts TeamsWorkflowsOptions
}

func NewTeamsWorkflowsService(opts TeamsWorkflowsOptions) NotificationService {
	return &teamsWorkflowsService{opts: opts}
}

// validateWorkflowsWebhookURL validates that the webhook URL is properly formatted for Workflows
func validateWorkflowsWebhookURL(webhookURL string) error {
	if webhookURL == "" {
		return errors.New("webhook URL cannot be empty")
	}

	parsedURL, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL format: %w", err)
	}

	// Allow HTTP only for localhost/test URLs, enforce HTTPS for production
	isLocalhost := parsedURL.Hostname() == "localhost" || parsedURL.Hostname() == "127.0.0.1" || strings.HasPrefix(parsedURL.Hostname(), "127.")
	if parsedURL.Scheme != "https" && !isLocalhost {
		return errors.New("webhook URL must use HTTPS scheme")
	}

	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return errors.New("webhook URL must use HTTP or HTTPS scheme")
	}

	if parsedURL.Host == "" {
		return errors.New("webhook URL must have a valid host")
	}

	// Validate it matches Workflows URL pattern
	if !WorkflowsURLPattern.MatchString(webhookURL) {
		return errors.New("webhook URL does not appear to be a valid Microsoft Teams Workflows URL (Power Automate). Expected patterns: api.powerautomate.com, api.powerplatform.com, flow.microsoft.com, or webhook.office.com/workflows")
	}

	return nil
}

func (s teamsWorkflowsService) Send(notification Notification, dest Destination) (err error) {
	webhookUrl, ok := s.opts.RecipientUrls[dest.Recipient]
	if !ok {
		return fmt.Errorf("no teams-workflows webhook configured for recipient %s", dest.Recipient)
	}

	// Validate webhook URL format for Workflows
	if err := validateWorkflowsWebhookURL(webhookUrl); err != nil {
		return fmt.Errorf("invalid webhook URL for recipient %s: %w", dest.Recipient, err)
	}

	client, err := httputil.NewServiceHTTPClient(s.opts.TransportOptions, s.opts.InsecureSkipVerify, webhookUrl, "teams-workflows")
	if err != nil {
		return fmt.Errorf("failed to create HTTP client for teams-workflows webhook: %w", err)
	}

	// Generate message payload
	message, err := teamsWorkflowsNotificationToReader(notification)
	if err != nil {
		return fmt.Errorf("failed to generate message payload for teams-workflows: %w", err)
	}

	response, err := client.Post(webhookUrl, "application/json", bytes.NewReader(message))
	if err != nil {
		return fmt.Errorf("failed to post message to teams-workflows webhook %s: %w", webhookUrl, err)
	}

	defer func() {
		_ = response.Body.Close()
	}()

	// Workflows webhooks return HTTP status codes for success/failure
	// 200-299 indicates success, anything else is an error
	if response.StatusCode < 200 || response.StatusCode > 299 {
		bodyBytes, err := io.ReadAll(response.Body)
		errorMsg := "no error details provided"
		if err == nil && len(bodyBytes) > 0 {
			errorMsg = string(bodyBytes)
		}
		return fmt.Errorf("teams workflows webhook post error (status %d): %s", response.StatusCode, errorMsg)
	}

	return nil
}

// notificationData holds the unified data structure extracted from a Notification
// This is used as an intermediate format that is converted to AdaptiveCard format
type notificationData struct {
	Title           string
	Summary         string
	Text            string
	ThemeColor      string
	Sections        []teamsSection
	Facts           []map[string]any
	PotentialAction []teamsAction
}

// extractNotificationData extracts common data from a Notification into a unified structure
// This data is then converted to AdaptiveCard format
func extractNotificationData(n Notification) (*notificationData, error) {
	data := &notificationData{
		Text: n.Message,
	}

	if n.TeamsWorkflows == nil {
		return data, nil
	}

	if n.TeamsWorkflows.Title != "" {
		data.Title = n.TeamsWorkflows.Title
	}

	if n.TeamsWorkflows.Summary != "" {
		data.Summary = n.TeamsWorkflows.Summary
	}

	if n.TeamsWorkflows.Text != "" {
		data.Text = n.TeamsWorkflows.Text
	}

	if n.TeamsWorkflows.ThemeColor != "" {
		data.ThemeColor = n.TeamsWorkflows.ThemeColor
	}

	if n.TeamsWorkflows.Sections != "" {
		var sections []teamsSection
		err := json.Unmarshal([]byte(n.TeamsWorkflows.Sections), &sections)
		if err != nil {
			return nil, fmt.Errorf("teams-workflows sections unmarshalling error %w", err)
		}
		data.Sections = sections
	}

	if n.TeamsWorkflows.Facts != "" {
		var facts []map[string]any
		err := json.Unmarshal([]byte(n.TeamsWorkflows.Facts), &facts)
		if err != nil {
			return nil, fmt.Errorf("teams-workflows facts unmarshalling error %w", err)
		}
		data.Facts = facts
	}

	if n.TeamsWorkflows.PotentialAction != "" {
		var actions []teamsAction
		err := json.Unmarshal([]byte(n.TeamsWorkflows.PotentialAction), &actions)
		if err != nil {
			return nil, fmt.Errorf("teams-workflows potential action unmarshalling error %w", err)
		}
		data.PotentialAction = actions
	}

	return data, nil
}

// Adaptive Card structures for Teams Workflows
type adaptiveCard struct {
	Type    string                `json:"type"`
	Version string                `json:"version"`
	Body    []adaptiveCardElement `json:"body"`
	Actions []adaptiveCardAction  `json:"actions,omitempty"`
}

type adaptiveCardElement struct {
	Type   string             `json:"type"`
	Text   string             `json:"text,omitempty"`
	Size   string             `json:"size,omitempty"`
	Weight string             `json:"weight,omitempty"`
	Color  string             `json:"color,omitempty"`
	Wrap   bool               `json:"wrap,omitempty"`
	Facts  []adaptiveCardFact `json:"facts,omitempty"`
}

type adaptiveCardFact struct {
	Title string `json:"title"`
	Value string `json:"value"`
}

type adaptiveCardAction struct {
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
	URL   string `json:"url,omitempty"`
}

// adaptiveMessage is the outer wrapper for Teams Workflows Adaptive Card messages
// It matches the expected structure:
//
//	{
//	  "type": "message",
//	  "attachments": [
//	    {
//	      "contentType": "application/vnd.microsoft.card.adaptive",
//	      "content": { ... AdaptiveCard ... }
//	    }
//	  ]
//	}
type adaptiveMessage struct {
	Type        string               `json:"type"`
	Attachments []adaptiveAttachment `json:"attachments"`
}

type adaptiveAttachment struct {
	ContentType string        `json:"contentType"`
	Content     *adaptiveCard `json:"content"`
}

// buildAdaptiveCard converts notificationData to Adaptive Card format
func buildAdaptiveCard(data *notificationData) *adaptiveCard {
	card := &adaptiveCard{
		Type:    "AdaptiveCard",
		Version: "1.4",
		Body:    []adaptiveCardElement{},
	}

	// Add title if present
	if data.Title != "" {
		card.Body = append(card.Body, adaptiveCardElement{
			Type:   "TextBlock",
			Text:   data.Title,
			Size:   "Large",
			Weight: "Bolder",
			Color:  data.ThemeColor, // ThemeColor can contain values like "Good", "Warning", etc.
			Wrap:   true,
		})
	}

	// Add text if present
	if data.Text != "" {
		card.Body = append(card.Body, adaptiveCardElement{
			Type: "TextBlock",
			Text: data.Text,
			Wrap: true,
		})
	}

	// Add facts if present (from Facts field)
	if len(data.Facts) > 0 {
		factSet := adaptiveCardElement{
			Type:  "FactSet",
			Facts: []adaptiveCardFact{},
		}
		for _, fact := range data.Facts {
			if name, ok := fact["name"].(string); ok {
				value := ""
				if v, ok := fact["value"].(string); ok {
					value = v
				} else if v, ok := fact["value"]; ok {
					value = fmt.Sprintf("%v", v)
				}
				factSet.Facts = append(factSet.Facts, adaptiveCardFact{
					Title: name,
					Value: value,
				})
			}
		}
		if len(factSet.Facts) > 0 {
			card.Body = append(card.Body, factSet)
		}
	}

	// Add sections if present (convert section facts to Adaptive Card facts)
	for _, section := range data.Sections {
		if facts, ok := section["facts"].([]any); ok {
			factSet := adaptiveCardElement{
				Type:  "FactSet",
				Facts: []adaptiveCardFact{},
			}
			for _, fact := range facts {
				if factMap, ok := fact.(map[string]any); ok {
					if name, ok := factMap["name"].(string); ok {
						value := ""
						if v, ok := factMap["value"].(string); ok {
							value = v
						} else if v, ok := factMap["value"]; ok {
							value = fmt.Sprintf("%v", v)
						}
						factSet.Facts = append(factSet.Facts, adaptiveCardFact{
							Title: name,
							Value: value,
						})
					}
				}
			}
			if len(factSet.Facts) > 0 {
				card.Body = append(card.Body, factSet)
			}
		}
	}

	// Add actions if present
	if len(data.PotentialAction) > 0 {
		card.Actions = []adaptiveCardAction{}
		for _, action := range data.PotentialAction {
			if actionType, ok := action["@type"].(string); ok {
				if actionType == "OpenUri" {
					acAction := adaptiveCardAction{
						Type: "Action.OpenUrl",
					}
					if name, ok := action["name"].(string); ok {
						acAction.Title = name
					}
					if targets, ok := action["targets"].([]any); ok && len(targets) > 0 {
						if target, ok := targets[0].(map[string]any); ok {
							if uri, ok := target["uri"].(string); ok {
								acAction.URL = uri
							}
						}
					}
					if acAction.URL != "" {
						card.Actions = append(card.Actions, acAction)
					}
				}
			}
		}
	}

	return card
}

func teamsWorkflowsNotificationToReader(n Notification) ([]byte, error) {
	// Check if a custom AdaptiveCard template is provided
	if n.TeamsWorkflows != nil && n.TeamsWorkflows.AdaptiveCard != "" {
		// Use the custom AdaptiveCard template directly, wrapped in message envelope
		var card adaptiveCard
		if err := json.Unmarshal([]byte(n.TeamsWorkflows.AdaptiveCard), &card); err != nil {
			return nil, fmt.Errorf("teams-workflows adaptiveCard unmarshalling error %w", err)
		}

		payload := adaptiveMessage{
			Type: "message",
			Attachments: []adaptiveAttachment{
				{
					ContentType: "application/vnd.microsoft.card.adaptive",
					Content:     &card,
				},
			},
		}
		return json.Marshal(payload)
	}

	// Extract unified notification data
	data, err := extractNotificationData(n)
	if err != nil {
		return nil, err
	}

	// Build AdaptiveCard and wrap it in the message envelope
	card := buildAdaptiveCard(data)
	payload := adaptiveMessage{
		Type: "message",
		Attachments: []adaptiveAttachment{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				Content:     card,
			},
		},
	}

	return json.Marshal(payload)
}
