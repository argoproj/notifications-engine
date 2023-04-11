package services

import (
	"bytes"
	"context"
	"fmt"
	texttemplate "text/template"

	"github.com/PagerDuty/go-pagerduty"
	log "github.com/sirupsen/logrus"
)

type PagerDutyV2Notification struct {
	Summary   string `json:"summary"`
	Severity  string `json:"severity"`
	Source    string `json:"source"`
	Component string `json:"component,omitempty"`
	Group     string `json:"group,omitempty"`
	Class     string `json:"class,omitempty"`
	URL       string `json:"url"`
}

type PagerdutyV2Options struct {
	ServiceKeys map[string]string `json:"serviceKeys"`
}

func (p *PagerDutyV2Notification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	summary, err := texttemplate.New(name).Funcs(f).Parse(p.Summary)
	if err != nil {
		return nil, err
	}
	severity, err := texttemplate.New(name).Funcs(f).Parse(p.Severity)
	if err != nil {
		return nil, err
	}
	source, err := texttemplate.New(name).Funcs(f).Parse(p.Source)
	if err != nil {
		return nil, err
	}
	component, err := texttemplate.New(name).Funcs(f).Parse(p.Component)
	if err != nil {
		return nil, err
	}
	group, err := texttemplate.New(name).Funcs(f).Parse(p.Group)
	if err != nil {
		return nil, err
	}
	class, err := texttemplate.New(name).Funcs(f).Parse(p.Class)
	if err != nil {
		return nil, err
	}
	url, err := texttemplate.New(name).Funcs(f).Parse(p.URL)
	if err != nil {
		return nil, err
	}

	return func(notification *Notification, vars map[string]interface{}) error {
		if notification.PagerdutyV2 == nil {
			notification.PagerdutyV2 = &PagerDutyV2Notification{}
		}
		var summaryData bytes.Buffer
		if err := summary.Execute(&summaryData, vars); err != nil {
			return err
		}
		notification.PagerdutyV2.Summary = summaryData.String()

		var severityData bytes.Buffer
		if err := severity.Execute(&severityData, vars); err != nil {
			return err
		}
		notification.PagerdutyV2.Severity = severityData.String()

		var sourceData bytes.Buffer
		if err := source.Execute(&sourceData, vars); err != nil {
			return err
		}
		notification.PagerdutyV2.Source = sourceData.String()

		var componentData bytes.Buffer
		if err := component.Execute(&componentData, vars); err != nil {
			return err
		}
		notification.PagerdutyV2.Component = componentData.String()

		var groupData bytes.Buffer
		if err := group.Execute(&groupData, vars); err != nil {
			return err
		}
		notification.PagerdutyV2.Group = groupData.String()

		var classData bytes.Buffer
		if err := class.Execute(&classData, vars); err != nil {
			return err
		}
		notification.PagerdutyV2.Class = classData.String()

		var urlData bytes.Buffer
		if err := url.Execute(&urlData, vars); err != nil {
			return err
		}
		notification.PagerdutyV2.URL = urlData.String()

		return nil
	}, nil
}

func NewPagerdutyV2Service(opts PagerdutyV2Options) NotificationService {
	return &pagerdutyV2Service{opts: opts}
}

type pagerdutyV2Service struct {
	opts PagerdutyV2Options
}

func (p pagerdutyV2Service) Send(notification Notification, dest Destination) error {
	routingKey, ok := p.opts.ServiceKeys[dest.Recipient]
	if !ok {
		return fmt.Errorf("no API key configured for recipient %s", dest.Recipient)
	}

	if notification.PagerdutyV2 == nil {
		return fmt.Errorf("no config found for pagerdutyv2")
	}

	event := buildEvent(routingKey, notification)

	response, err := pagerduty.ManageEventWithContext(context.TODO(), event)
	if err != nil {
		log.Errorf("Error: %v", err)
		return err
	}
	log.Debugf("PagerDuty event triggered succesfully. Status: %v, Message: %v", response.Status, response.Message)
	return nil
}

func buildEvent(routingKey string, notification Notification) pagerduty.V2Event {
	payload := pagerduty.V2Payload{
		Summary:  notification.PagerdutyV2.Summary,
		Severity: notification.PagerdutyV2.Severity,
		Source:   notification.PagerdutyV2.Source,
	}

	if len(notification.PagerdutyV2.Component) > 0 {
		payload.Component = notification.PagerdutyV2.Component
	}
	if len(notification.PagerdutyV2.Group) > 0 {
		payload.Group = notification.PagerdutyV2.Group
	}
	if len(notification.PagerdutyV2.Class) > 0 {
		payload.Class = notification.PagerdutyV2.Class
	}

	event := pagerduty.V2Event{
		RoutingKey: routingKey,
		Action:     "trigger",
		Payload:    &payload,
		Client:     "ArgoCD",
	}

	if len(notification.PagerdutyV2.URL) > 0 {
		event.ClientURL = notification.PagerdutyV2.URL
	}

	return event
}
