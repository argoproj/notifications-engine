package services

import (
	"context"
	"fmt"

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
	var client *pubsub.Client
	var err error

	// Use key file if provided, else default creds (enables GKE Workload Identity)
	if s.opts.KeyFile != "" {
		client, err = pubsub.NewClient(ctx, s.opts.Project, option.WithCredentialsFile(s.opts.KeyFile))
	} else {
		client, err = pubsub.NewClient(ctx, s.opts.Project)
	}
	if err != nil {
		log.Errorf("failed to create pubsub client: %v", err)
		return err
	}
	defer client.Close()

	// Determine topic name, annotations takes precedence over the default topic
	topicName := s.opts.Topic
	if dest.Recipient != "" {
		topicName = dest.Recipient
	}
	topic := client.Topic(topicName)
	if topic == nil {
		return fmt.Errorf("pubsub topic '%s' not found", topicName)
	}

	msg := &pubsub.Message{
		Data: []byte(notif.Message),
	}
	if notif.GcpPubsub.Attributes != nil {
		msg.Attributes = notif.GcpPubsub.Attributes
	}

	res := topic.Publish(ctx, msg)
	id, err := res.Get(ctx)
	if err != nil {
		log.Errorf("failed to publish pubsub message: %v", err)
		return err
	}
	log.Debugf("Published message to Pub/Sub with ID: %s", id)
	topic.Stop()
	return nil
}
