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
	PriorityID string `json:"priorityID,omitempty"`
}

type PagerdutyOptions struct {
	Token              string   `json:"token"`
	From               string   `json:"from,omitempty"`
	ServiceID          string   `json:"serviceID"`
	EscalationPolicyID string   `json:"escalationPolicyID,omitempty"`
	Assignments        []string `json:"assignments,omitempty"`
	ConferenceNumber   string   `json:"conferenceBridgeNumber,omitempty"`
	ConferenceUrl      string   `json:"conferenceBridgeUrl,omitempty"`
}

func (p *PagerDutyNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	Title, err := texttemplate.New(name).Funcs(f).Parse(p.Title)
	if err != nil {
		return nil, err
	}
	Body, err := texttemplate.New(name).Funcs(f).Parse(p.Body)
	if err != nil {
		return nil, err
	}
	Urgency, err := texttemplate.New(name).Funcs(f).Parse(p.Urgency)
	if err != nil {
		return nil, err
	}
	PriorityID, err := texttemplate.New(name).Funcs(f).Parse(p.PriorityID)
	if err != nil {
		return nil, err
	}

	return func(notification *Notification, vars map[string]interface{}) error {
		if notification.Pagerduty == nil {
			notification.Pagerduty = &PagerDutyNotification{}
		}
		var titleData bytes.Buffer
		if err := Title.Execute(&titleData, vars); err != nil {
			return err
		}
		notification.Pagerduty.Title = titleData.String()

		var pdBodyData bytes.Buffer
		if err := Body.Execute(&pdBodyData, vars); err != nil {
			return err
		}
		notification.Pagerduty.Body = pdBodyData.String()

		var pdUrgencyData bytes.Buffer
		if err := Urgency.Execute(&pdUrgencyData, vars); err != nil {
			return err
		}
		notification.Pagerduty.Urgency = pdUrgencyData.String()

		var pdPriorityIDData bytes.Buffer
		if err := PriorityID.Execute(&pdPriorityIDData, vars); err != nil {
			return err
		}
		notification.Pagerduty.PriorityID = pdPriorityIDData.String()

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
	Title := notification.Pagerduty.Title
	Body := notification.Pagerduty.Body
	Urgency := notification.Pagerduty.Urgency
	PriorityID := notification.Pagerduty.PriorityID

	pagerDutyClient := pagerduty.NewClient(p.opts.Token)
	var input1 *pagerduty.CreateIncidentOptions

	if p.opts.EscalationPolicyID != "" {
		input1 = &pagerduty.CreateIncidentOptions{
			Type:     "incident",
			Service:  &pagerduty.APIReference{ID: p.opts.ServiceID, Type: "service_reference"},
			Priority: &pagerduty.APIReference{ID: PriorityID, Type: "priority"},
			Title:    Title,
			Urgency:  Urgency,
			Body: &pagerduty.APIDetails{Type: "incident_details	", Details: Body},
			EscalationPolicy: &pagerduty.APIReference{Type: "escalation_policy_reference", ID: p.opts.EscalationPolicyID},
			ConferenceBridge: &pagerduty.ConferenceBridge{ConferenceNumber: p.opts.ConferenceNumber, ConferenceURL: p.opts.ConferenceUrl},
		}
	} else {
		var Assignees []pagerduty.Assignee
		for _, i := range p.opts.Assignments {
			Assignees = append(Assignees, pagerduty.Assignee{Assignee: pagerduty.APIObject{
				ID:   i,
				Type: "user_reference",
			}})
		}
		input1 = &pagerduty.CreateIncidentOptions{
			Type:             "incident",
			Service:          &pagerduty.APIReference{ID: p.opts.ServiceID, Type: "service_reference"},
			Priority:         &pagerduty.APIReference{ID: PriorityID, Type: "priority"},
			Title:            Title,
			Urgency:          Urgency,
			Body:             &pagerduty.APIDetails{Type: "incident_details", Details: Body},
			Assignments:      Assignees,
			ConferenceBridge: &pagerduty.ConferenceBridge{ConferenceNumber: p.opts.ConferenceNumber, ConferenceURL: p.opts.ConferenceUrl},
		}
	}
	incident, err := pagerDutyClient.CreateIncidentWithContext(context.TODO(), p.opts.From, input1)
	if err != nil {
		log.Errorf("Error: \n%v", err)
		return err
	}
	log.Infof("Incident: \n%v", incident)
	log.Infof("Incident Number: %v, IncidentKey:%v, incident.ID: %v, incident.Title: %v", incident.IncidentNumber, incident.IncidentKey, incident.ID, incident.Title)
	return nil
}
