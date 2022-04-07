package services

import (
	"bytes"
	"context"
	texttemplate "text/template"

	"github.com/PagerDuty/go-pagerduty"
	log "github.com/sirupsen/logrus"
)

type PagerDutyNotification struct {
	Title      string `json:"title"`
	Body       string `json:"body,omitempty"`
	Urgency    string `json:"urgency,omitempty"`
	PriorityId string `json:"priorityId,omitempty"`
}

type PagerdutyOptions struct {
	Token     string `json:"token"`
	From      string `json:"from,omitempty"`
	ServiceID string `json:"serviceId"`
}

func (p *PagerDutyNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	title, err := texttemplate.New(name).Funcs(f).Parse(p.Title)
	if err != nil {
		return nil, err
	}
	body, err := texttemplate.New(name).Funcs(f).Parse(p.Body)
	if err != nil {
		return nil, err
	}
	urgency, err := texttemplate.New(name).Funcs(f).Parse(p.Urgency)
	if err != nil {
		return nil, err
	}
	priorityId, err := texttemplate.New(name).Funcs(f).Parse(p.PriorityId)
	if err != nil {
		return nil, err
	}

	return func(notification *Notification, vars map[string]interface{}) error {
		if notification.Pagerduty == nil {
			notification.Pagerduty = &PagerDutyNotification{}
		}
		var titleData bytes.Buffer
		if err := title.Execute(&titleData, vars); err != nil {
			return err
		}
		notification.Pagerduty.Title = titleData.String()

		var pdBodyData bytes.Buffer
		if err := body.Execute(&pdBodyData, vars); err != nil {
			return err
		}
		notification.Pagerduty.Body = pdBodyData.String()

		var pdUrgencyData bytes.Buffer
		if err := urgency.Execute(&pdUrgencyData, vars); err != nil {
			return err
		}
		notification.Pagerduty.Urgency = pdUrgencyData.String()

		var pdPriorityIDData bytes.Buffer
		if err := priorityId.Execute(&pdPriorityIDData, vars); err != nil {
			return err
		}
		notification.Pagerduty.PriorityId = pdPriorityIDData.String()

		return nil
	}, nil
}

func NewPagerdutyService(opts PagerdutyOptions) NotificationService {
	return &pagerdutyService{opts: opts}
}

type pagerdutyService struct {
	opts PagerdutyOptions
}

func (p pagerdutyService) Send(notification Notification, dest Destination) error {
	title := notification.Pagerduty.Title
	body := notification.Pagerduty.Body
	urgency := notification.Pagerduty.Urgency
	priorityID := notification.Pagerduty.PriorityId

	pagerDutyClient := pagerduty.NewClient(p.opts.Token)
	input := &pagerduty.CreateIncidentOptions{
		Type:     "incident",
		Service:  &pagerduty.APIReference{ID: dest.Recipient, Type: "service_reference"},
		Priority: &pagerduty.APIReference{ID: priorityID, Type: "priority"},
		Title:    title,
		Urgency:  urgency,
		Body: &pagerduty.APIDetails{Type: "incident_details	", Details: body},
	}
	incident, err := pagerDutyClient.CreateIncidentWithContext(context.TODO(), p.opts.From, input)
	if err != nil {
		log.Errorf("Error: %v", err)
		return err
	}
	log.Debugf("Incident created Succesfully. Incident Number: %v, IncidentKey:%v, incident.ID: %v, incident.Title: %v", incident.IncidentNumber, incident.IncidentKey, incident.ID, incident.Title)
	return nil
}
