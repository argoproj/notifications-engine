package services

import (
	"bytes"
	"fmt"
	texttemplate "text/template"
)

type DatadogNotification struct {
	AlertType string `json:"alertType,omitempty"`
	Tags      string `json:"tags,omitempty"`
}

func (n *DatadogNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	tags, err := texttemplate.New(name).Funcs(f).Parse(n.Tags)
	if err != nil {
		return nil, err
	}

	return func(notification *Notification, vars map[string]interface{}) error {
		if notification.Datadog == nil {
			notification.Datadog = &DatadogNotification{}
		}

		notification.Datadog.AlertType = n.AlertType

		var tagsData bytes.Buffer
		if err := tags.Execute(&tagsData, vars); err != nil {
			return err
		}
		notification.Datadog.Tags = tagsData.String()

		return nil
	}, nil
}

type DatadogOptions struct {
	DDApiKey string `json:"ddApiKey"`
	DDAppKey string `json:"ddAppKey"`
}

type datadogService struct {
	opts DatadogOptions
}

func NewDatadogService(opts DatadogOptions) NotificationService {
	return &datadogService{opts: opts}
}

func (s *datadogService) Send(notification Notification, dest Destination) error {
	// print notificaiton.Datadog
	fmt.Printf("%+v", notification.Datadog)

	return nil
}
