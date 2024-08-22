package services

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	texttemplate "text/template"

	"github.com/opsgenie/opsgenie-go-sdk-v2/alert"
	"github.com/opsgenie/opsgenie-go-sdk-v2/client"
	log "github.com/sirupsen/logrus"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
)

type OpsgenieOptions struct {
	ApiUrl  string            `json:"apiUrl"`
	ApiKeys map[string]string `json:"apiKeys"`
}

type OpsgenieNotification struct {
	Description string `json:"description"`
	Priority    string `json:"priority,omitempty"`
	Alias       string `json:"alias,omitempty"`
	Note        string `json:"note,omitempty"`
}

func (n *OpsgenieNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	desc, err := texttemplate.New(name).Funcs(f).Parse(n.Description)
	if err != nil {
		return nil, err
	}
	alias, err := texttemplate.New(name).Funcs(f).Parse(n.Alias)
	if err != nil {
		return nil, err
	}
	note, err := texttemplate.New(name).Funcs(f).Parse(n.Note)
	if err != nil {
		return nil, err
	}

	priority := strings.ToUpper(n.Priority)

	if err := alert.ValidatePriority(alert.Priority(priority)); err != nil {
		return nil, err
	}

	return func(notification *Notification, vars map[string]interface{}) error {
		if notification.Opsgenie == nil {
			notification.Opsgenie = &OpsgenieNotification{}
		}

		var descData bytes.Buffer
		if err := desc.Execute(&descData, vars); err != nil {
			return err
		}
		notification.Opsgenie.Description = descData.String()
		var aliasData bytes.Buffer
		if err := alias.Execute(&aliasData, vars); err != nil {
			return err
		}
		notification.Opsgenie.Alias = aliasData.String()
		var noteData bytes.Buffer
		if err := note.Execute(&noteData, vars); err != nil {
			return err
		}
		notification.Opsgenie.Note = noteData.String()

		notification.Opsgenie.Priority = priority

		return nil
	}, nil
}

type opsgenieService struct {
	opts OpsgenieOptions
}

func NewOpsgenieService(opts OpsgenieOptions) NotificationService {
	return &opsgenieService{opts: opts}
}

func (s *opsgenieService) Send(notification Notification, dest Destination) error {
	apiKey, ok := s.opts.ApiKeys[dest.Recipient]
	if !ok {
		return fmt.Errorf("no API key configured for recipient %s", dest.Recipient)
	}
	alertClient, _ := alert.NewClient(&client.Config{
		ApiKey:         apiKey,
		OpsGenieAPIURL: client.ApiUrl(s.opts.ApiUrl),
		HttpClient: &http.Client{
			Transport: httputil.NewLoggingRoundTripper(
				httputil.NewTransport(s.opts.ApiUrl, false), log.WithField("service", "opsgenie")),
		},
	})

	var description, priority, alias, note string

	if notification.Opsgenie != nil {
		if notification.Opsgenie.Description == "" {
			return fmt.Errorf("Opsgenie notification description is missing")
		}

		description = notification.Opsgenie.Description

		if notification.Opsgenie.Priority != "" {
			priority = notification.Opsgenie.Priority
		}

		if notification.Opsgenie.Alias != "" {
			alias = notification.Opsgenie.Alias
		}

		if notification.Opsgenie.Note != "" {
			note = notification.Opsgenie.Note
		}
	}

	alertPriority := alert.Priority(priority)

	_, err := alertClient.Create(context.TODO(), &alert.CreateAlertRequest{
		Message:     notification.Message,
		Description: description,
		Priority:    alertPriority,
		Alias:       alias,
		Note:        note,
		Responders: []alert.Responder{
			{
				Type: "team",
				Id:   dest.Recipient,
			},
		},
		Source: "Argo CD",
	})
	return err
}
