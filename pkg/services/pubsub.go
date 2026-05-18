package services

import (
	"bytes"
	"context"
	texttemplate "text/template"

	"cloud.google.com/go/pubsub"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/option"
)

type GcpPubsubOptions struct {
	Topic   string `json:"topic"`
	Project string `json:"project"`
	KeyFile string `json:"keyFile,omitempty"`
}

type GcpPubsubNotification struct {
	Attributes map[string]string `json:"attributes,omitempty"`
}

func NewGcpPubsubService(opts GcpPubsubOptions) NotificationService {
	return &gcpPubsubService{opts: opts}
}

type gcpPubsubService struct {
	opts GcpPubsubOptions
}

func (s *gcpPubsubService) Send(notif Notification, dest Destination) error {
	ctx := context.Background()

	topicName := s.opts.Topic
	if dest.Recipient != "" {
		topicName = dest.Recipient
	}

	msg := &pubsub.Message{
		Data: []byte(notif.Message),
	}
	if notif.GcpPubsub != nil && notif.GcpPubsub.Attributes != nil {
		msg.Attributes = notif.GcpPubsub.Attributes
	}

	id, err := PublishPubsubMessage(ctx, s.opts.Project, s.opts.KeyFile, topicName, msg)
	if err != nil {
		log.Errorf("failed to publish pubsub message: %v", err)
		return err
	}
	log.Debugf("Published message to Pub/Sub with ID: %s", id)
	return nil
}

func (n *GcpPubsubNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	return func(notification *Notification, vars map[string]any) error {
		if notification.GcpPubsub == nil {
			notification.GcpPubsub = &GcpPubsubNotification{}
		}

		if len(n.Attributes) > 0 {
			notification.GcpPubsub.Attributes = make(map[string]string, len(n.Attributes))
			for k, v := range n.Attributes {
				notification.GcpPubsub.Attributes[k] = v
			}
			if err := notification.GcpPubsub.parseAttributes(name, f, vars); err != nil {
				return err
			}
		}

		return nil
	}, nil
}

func (n *GcpPubsubNotification) parseAttributes(name string, f texttemplate.FuncMap, vars map[string]any) error {
	for k, v := range n.Attributes {
		var tempData bytes.Buffer

		tmpl, err := texttemplate.New(name).Funcs(f).Parse(v)
		if err != nil {
			return err
		}
		if err := tmpl.Execute(&tempData, vars); err != nil {
			return err
		}
		if val := tempData.String(); val != "" {
			n.Attributes[k] = val
		}
	}
	return nil
}

var PublishPubsubMessage = func(ctx context.Context, projectID, keyFile, topicName string, msg *pubsub.Message) (string, error) {
	var client *pubsub.Client
	var err error
	if keyFile != "" {
		client, err = pubsub.NewClient(ctx, projectID, option.WithCredentialsFile(keyFile))
	} else {
		client, err = pubsub.NewClient(ctx, projectID)
	}
	if err != nil {
		return "", err
	}
	defer client.Close()

	topic := client.Topic(topicName)
	defer topic.Stop()

	result := topic.Publish(ctx, msg)
	return result.Get(ctx)
}
