package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	texttemplate "text/template"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
)

var (
	// WorkflowsURLPattern matches Microsoft Teams Workflows webhook URLs (Power Automate)
	// Matches: api.powerautomate.com, api.powerplatform.com, flow.microsoft.com, or webhook.office.com/workflows
	// Also matches URLs with /powerautomate/ in the path (Power Platform URLs)
	WorkflowsURLPattern = regexp.MustCompile(`(?i)(api\.(powerautomate|powerplatform)\.com|flow\.microsoft\.com|webhook\.office\.com/workflows|/powerautomate/)`)
)

type TeamsWorkflowsNotification struct {
	Template        string `json:"template,omitempty"`
	Title           string `json:"title,omitempty"`
	Summary         string `json:"summary,omitempty"`
	Text            string `json:"text,omitempty"`
	ThemeColor      string `json:"themeColor,omitempty"`
	Facts           string `json:"facts,omitempty"`
	Sections        string `json:"sections,omitempty"`
	PotentialAction string `json:"potentialAction,omitempty"`
	// AdaptiveCard specifies the Adaptive Card JSON payload (alternative to messageCard)
	AdaptiveCard string `json:"adaptiveCard,omitempty"`
	// CardFormat specifies which card format to use: "messageCard" (default) or "adaptiveCard"
	CardFormat string `json:"cardFormat,omitempty"`
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

	cardFormat, err := texttemplate.New(name).Funcs(f).Parse(n.CardFormat)
	if err != nil {
		return nil, fmt.Errorf("error in '%s' teams-workflows.cardFormat: %w", name, err)
	}

	return func(notification *Notification, vars map[string]interface{}) error {
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

		var cardFormatBuff bytes.Buffer
		if err := cardFormat.Execute(&cardFormatBuff, vars); err != nil {
			return err
		}
		if val := cardFormatBuff.String(); val != "" {
			notification.TeamsWorkflows.CardFormat = val
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
		return fmt.Errorf("webhook URL cannot be empty")
	}

	parsedURL, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL format: %w", err)
	}

	// Allow HTTP only for localhost/test URLs, enforce HTTPS for production
	isLocalhost := parsedURL.Hostname() == "localhost" || parsedURL.Hostname() == "127.0.0.1" || strings.HasPrefix(parsedURL.Hostname(), "127.")
	if parsedURL.Scheme != "https" && !isLocalhost {
		return fmt.Errorf("webhook URL must use HTTPS scheme")
	}

	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return fmt.Errorf("webhook URL must use HTTP or HTTPS scheme")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("webhook URL must have a valid host")
	}

	// Validate it matches Workflows URL pattern
	if !WorkflowsURLPattern.MatchString(webhookURL) {
		return fmt.Errorf("webhook URL does not appear to be a valid Microsoft Teams Workflows URL (Power Automate). Expected patterns: api.powerautomate.com, api.powerplatform.com, flow.microsoft.com, or webhook.office.com/workflows")
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
		return err
	}

	// Generate message payload
	message, err := teamsWorkflowsNotificationToReader(notification)
	if err != nil {
		return err
	}

	response, err := client.Post(webhookUrl, "application/json", bytes.NewReader(message))
	if err != nil {
		return err
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

func teamsWorkflowsNotificationToMessage(n Notification) (*teamsMessage, error) {
	message := &teamsMessage{
		Type:    "MessageCard",
		Context: "https://schema.org/extensions",
		Text:    n.Message,
	}

	if n.TeamsWorkflows == nil {
		return message, nil
	}

	if n.TeamsWorkflows.Title != "" {
		message.Title = n.TeamsWorkflows.Title
	}

	if n.TeamsWorkflows.Summary != "" {
		message.Summary = n.TeamsWorkflows.Summary
	}

	if n.TeamsWorkflows.Text != "" {
		message.Text = n.TeamsWorkflows.Text
	}

	if n.TeamsWorkflows.ThemeColor != "" {
		message.ThemeColor = n.TeamsWorkflows.ThemeColor
	}

	if n.TeamsWorkflows.Sections != "" {
		unmarshalledSections := make([]teamsSection, 2)
		err := json.Unmarshal([]byte(n.TeamsWorkflows.Sections), &unmarshalledSections)
		if err != nil {
			return nil, fmt.Errorf("teams-workflows sections unmarshalling error %w", err)
		}
		message.Sections = unmarshalledSections
	}

	if n.TeamsWorkflows.Facts != "" {
		unmarshalledFacts := make([]map[string]interface{}, 2)
		err := json.Unmarshal([]byte(n.TeamsWorkflows.Facts), &unmarshalledFacts)
		if err != nil {
			return nil, fmt.Errorf("teams-workflows facts unmarshalling error %w", err)
		}
		message.Sections = append(message.Sections, teamsSection{
			"facts": unmarshalledFacts,
		})
	}

	if n.TeamsWorkflows.PotentialAction != "" {
		unmarshalledActions := make([]teamsAction, 2)
		err := json.Unmarshal([]byte(n.TeamsWorkflows.PotentialAction), &unmarshalledActions)
		if err != nil {
			return nil, fmt.Errorf("teams-workflows potential action unmarshalling error %w", err)
		}
		message.PotentialAction = unmarshalledActions
	}

	return message, nil
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

// teamsWorkflowsNotificationToAdaptiveCard converts a Notification to an Adaptive Card format
func teamsWorkflowsNotificationToAdaptiveCard(n Notification) *adaptiveCard {
	card := &adaptiveCard{
		Type:    "message",
		Version: "1.4",
		Body:    []adaptiveCardElement{},
	}

	// Add title if present
	if n.TeamsWorkflows != nil && n.TeamsWorkflows.Title != "" {
		card.Body = append(card.Body, adaptiveCardElement{
			Type:   "TextBlock",
			Text:   n.TeamsWorkflows.Title,
			Size:   "Large",
			Weight: "Bolder",
			Wrap:   true,
		})
	}

	// Add summary/text if present
	textContent := n.Message
	if n.TeamsWorkflows != nil && n.TeamsWorkflows.Text != "" {
		textContent = n.TeamsWorkflows.Text
	}
	if textContent != "" {
		card.Body = append(card.Body, adaptiveCardElement{
			Type: "TextBlock",
			Text: textContent,
			Wrap: true,
		})
	}

	// Add facts if present
	if n.TeamsWorkflows != nil && n.TeamsWorkflows.Facts != "" {
		var facts []map[string]interface{}
		if err := json.Unmarshal([]byte(n.TeamsWorkflows.Facts), &facts); err == nil {
			factSet := adaptiveCardElement{
				Type:  "FactSet",
				Facts: []adaptiveCardFact{},
			}
			for _, fact := range facts {
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
	}

	// Add sections if present (convert to Adaptive Card format)
	if n.TeamsWorkflows != nil && n.TeamsWorkflows.Sections != "" {
		var sections []teamsSection
		if err := json.Unmarshal([]byte(n.TeamsWorkflows.Sections), &sections); err == nil {
			for _, section := range sections {
				// Convert section facts to Adaptive Card facts
				if facts, ok := section["facts"].([]interface{}); ok {
					factSet := adaptiveCardElement{
						Type:  "FactSet",
						Facts: []adaptiveCardFact{},
					}
					for _, fact := range facts {
						if factMap, ok := fact.(map[string]interface{}); ok {
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
		}
	}

	// Add actions if present
	if n.TeamsWorkflows != nil && n.TeamsWorkflows.PotentialAction != "" {
		var actions []teamsAction
		if err := json.Unmarshal([]byte(n.TeamsWorkflows.PotentialAction), &actions); err == nil {
			card.Actions = []adaptiveCardAction{}
			for _, action := range actions {
				if actionType, ok := action["@type"].(string); ok {
					if actionType == "OpenUri" {
						acAction := adaptiveCardAction{
							Type: "Action.OpenUrl",
						}
						if name, ok := action["name"].(string); ok {
							acAction.Title = name
						}
						if targets, ok := action["targets"].([]interface{}); ok && len(targets) > 0 {
							if target, ok := targets[0].(map[string]interface{}); ok {
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
	}

	return card
}

func teamsWorkflowsNotificationToReader(n Notification) ([]byte, error) {
	if n.TeamsWorkflows != nil && n.TeamsWorkflows.Template != "" {
		return []byte(n.TeamsWorkflows.Template), nil
	}

	// Check if Adaptive Card is explicitly requested
	if n.TeamsWorkflows != nil && n.TeamsWorkflows.AdaptiveCard != "" {
		return []byte(n.TeamsWorkflows.AdaptiveCard), nil
	}

	// Determine card format based on notification settings
	cardFormat := "messageCard" // default
	if n.TeamsWorkflows != nil && n.TeamsWorkflows.CardFormat != "" {
		cardFormat = strings.ToLower(n.TeamsWorkflows.CardFormat)
	}

	// Generate message based on card format
	if cardFormat == "adaptivecard" {
		adaptiveCard := teamsWorkflowsNotificationToAdaptiveCard(n)
		return json.Marshal(adaptiveCard)
	}

	// Default to messageCard format
	message, err := teamsWorkflowsNotificationToMessage(n)
	if err != nil {
		return nil, err
	}

	marshal, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}

	return marshal, nil
}
