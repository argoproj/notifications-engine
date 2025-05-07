package services

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	texttemplate "text/template"

	"github.com/opsgenie/opsgenie-go-sdk-v2/alert"
	"github.com/opsgenie/opsgenie-go-sdk-v2/client"
	log "github.com/sirupsen/logrus"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
)

type OpsgenieOptions struct {
	ApiUrl    string                         `json:"apiUrl"`
	ApiKeys   map[string]string              `json:"apiKeys"`
	Transport httputil.HTTPTransportSettings `json:"transport"`
}

type OpsgenieNotification struct {
	Description string            `json:"description"`
	Priority    string            `json:"priority,omitempty"`
	Alias       string            `json:"alias,omitempty"`
	Note        string            `json:"note,omitempty"`
	Actions     []string          `json:"actions,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
	Entity      string            `json:"entity,omitempty"`
	VisibleTo   []alert.Responder `json:"visibleTo,omitempty"`
	User        string            `json:"user,omitempty"`
}

func (n *OpsgenieNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	desc, err := texttemplate.New(name).Funcs(f).Parse(n.Description)
	if err != nil {
		return nil, err
	}
	priority, err := texttemplate.New(name).Funcs(f).Parse(n.Priority)
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
	entity, err := texttemplate.New(name).Funcs(f).Parse(n.Entity)
	if err != nil {
		return nil, err
	}
	user, err := texttemplate.New(name).Funcs(f).Parse(n.User)
	if err != nil {
		return nil, err
	}

	details := make(map[string]*texttemplate.Template)
	for key, value := range n.Details {
		detailTemplate, err := texttemplate.New(fmt.Sprintf("%s_detail_%s", name, key)).Funcs(f).Parse(value)
		if err != nil {
			return nil, err
		}
		details[key] = detailTemplate
	}

	visibleTo := make([]struct {
		Type     *texttemplate.Template
		Name     *texttemplate.Template
		Id       *texttemplate.Template
		Username *texttemplate.Template
	}, len(n.VisibleTo))

	// Templating for VisibleTo Responder fields (Id, Type, Name, Username)
	for i, responder := range n.VisibleTo {
		idTemplate, err := texttemplate.New(fmt.Sprintf("%s_responder_%d_id", name, i)).Funcs(f).Parse(responder.Id)
		if err != nil {
			return nil, err
		}
		typeTemplate, err := texttemplate.New(fmt.Sprintf("%s_responder_%d_type", name, i)).Funcs(f).Parse(string(responder.Type))
		if err != nil {
			return nil, err
		}
		nameTemplate, err := texttemplate.New(fmt.Sprintf("%s_responder_%d_name", name, i)).Funcs(f).Parse(responder.Name)
		if err != nil {
			return nil, err
		}
		usernameTemplate, err := texttemplate.New(fmt.Sprintf("%s_responder_%d_username", name, i)).Funcs(f).Parse(responder.Username)
		if err != nil {
			return nil, err
		}

		visibleTo[i] = struct {
			Type     *texttemplate.Template
			Name     *texttemplate.Template
			Id       *texttemplate.Template
			Username *texttemplate.Template
		}{
			Type:     typeTemplate,
			Name:     nameTemplate,
			Id:       idTemplate,
			Username: usernameTemplate,
		}
	}

	var actionsTemplates []*texttemplate.Template
	if n.Actions != nil {
		actionsTemplates = make([]*texttemplate.Template, len(n.Actions))
		for i, action := range n.Actions {
			actionTemplate, err := texttemplate.New(fmt.Sprintf("%s_action_%d", name, i)).Funcs(f).Parse(action)
			if err != nil {
				return nil, err
			}
			actionsTemplates[i] = actionTemplate
		}
	}

	var tagsTemplates []*texttemplate.Template
	if n.Tags != nil {
		tagsTemplates = make([]*texttemplate.Template, len(n.Tags))
		for i, tag := range n.Tags {
			tagTemplate, err := texttemplate.New(fmt.Sprintf("%s_tag_%d", name, i)).Funcs(f).Parse(tag)
			if err != nil {
				return nil, err
			}
			tagsTemplates[i] = tagTemplate
		}
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

		var entityData bytes.Buffer
		if err := entity.Execute(&entityData, vars); err != nil {
			return err
		}
		notification.Opsgenie.Entity = entityData.String()

		var userData bytes.Buffer
		if err := user.Execute(&userData, vars); err != nil {
			return err
		}
		notification.Opsgenie.User = userData.String()

		var priorityData bytes.Buffer
		if err := priority.Execute(&priorityData, vars); err != nil {
			return err
		}
		notification.Opsgenie.Priority = priorityData.String()

		if n.Details != nil {
			notification.Opsgenie.Details = make(map[string]string, len(n.Details))
			for key, template := range details {
				var valueData bytes.Buffer
				if err := template.Execute(&valueData, vars); err != nil {
					return err
				}
				notification.Opsgenie.Details[key] = valueData.String()
			}
		}

		if n.VisibleTo != nil {
			notification.Opsgenie.VisibleTo = make([]alert.Responder, len(n.VisibleTo))
			for i, template := range visibleTo {
				var idData, typeData, nameData, usernameData bytes.Buffer

				// Execute each responder field template
				if err := template.Id.Execute(&idData, vars); err != nil {
					return err
				}
				if err := template.Type.Execute(&typeData, vars); err != nil {
					return err
				}
				if err := template.Name.Execute(&nameData, vars); err != nil {
					return err
				}
				if err := template.Username.Execute(&usernameData, vars); err != nil {
					return err
				}

				notification.Opsgenie.VisibleTo[i] = alert.Responder{
					Id:       idData.String(),
					Type:     alert.ResponderType(typeData.String()), // Convert the string to the ResponderType
					Name:     nameData.String(),
					Username: usernameData.String(),
				}
			}
		}

		if n.Actions != nil {
			notification.Opsgenie.Actions = make([]string, len(n.Actions))
			for i, template := range actionsTemplates {
				var actionData bytes.Buffer
				if err := template.Execute(&actionData, vars); err != nil {
					return err
				}
				notification.Opsgenie.Actions[i] = actionData.String()
			}
		}

		if n.Tags != nil {
			notification.Opsgenie.Tags = make([]string, len(n.Tags))
			for i, template := range tagsTemplates {
				var tagData bytes.Buffer
				if err := template.Execute(&tagData, vars); err != nil {
					return err
				}
				notification.Opsgenie.Tags[i] = tagData.String()
			}
		}

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
	s.opts.Transport.InsecureSkipVerify = false
	alertClient, _ := alert.NewClient(&client.Config{
		ApiKey:         apiKey,
		OpsGenieAPIURL: client.ApiUrl(s.opts.ApiUrl),
		HttpClient: &http.Client{
			Transport: httputil.NewLoggingRoundTripper(
				httputil.NewTransport(s.opts.ApiUrl, s.opts.Transport), log.WithField("service", "opsgenie")),
		},
	})

	var description, alias, note, entity, user string
	var priority alert.Priority
	var actions, tags []string
	var details map[string]string
	var visibleTo []alert.Responder

	if notification.Opsgenie != nil {
		if notification.Opsgenie.Description == "" {
			return fmt.Errorf("opsgenie notification description is missing")
		}

		description = notification.Opsgenie.Description

		if notification.Opsgenie.Alias != "" {
			alias = notification.Opsgenie.Alias
		}

		if notification.Opsgenie.Note != "" {
			note = notification.Opsgenie.Note
		}

		if notification.Opsgenie.Entity != "" {
			entity = notification.Opsgenie.Entity
		}

		if notification.Opsgenie.User != "" {
			user = notification.Opsgenie.User
		}

		if notification.Opsgenie.Priority != "" {
			priority = alert.Priority(notification.Opsgenie.Priority)
		}

		if len(notification.Opsgenie.Actions) > 0 {
			actions = notification.Opsgenie.Actions
		}

		if len(notification.Opsgenie.Tags) > 0 {
			tags = notification.Opsgenie.Tags
		}

		if len(notification.Opsgenie.Details) > 0 {
			details = notification.Opsgenie.Details
		}

		if len(notification.Opsgenie.VisibleTo) > 0 {
			visibleTo = notification.Opsgenie.VisibleTo
		}
	}

	_, err := alertClient.Create(context.TODO(), &alert.CreateAlertRequest{
		Message:     notification.Message,
		Description: description,
		Priority:    priority,
		Alias:       alias,
		Note:        note,
		Actions:     actions,
		Tags:        tags,
		Details:     details,
		Entity:      entity,
		VisibleTo:   visibleTo,
		User:        user,
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
