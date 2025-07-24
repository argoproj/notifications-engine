package services

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/option"
)

// Interfaces for testability
type pubsubClient interface {
	Topic(name string) pubsubTopic
	Close() error
}
type pubsubTopic interface {
	Publish(ctx context.Context, msg *pubsub.Message) pubsubPublishResult
	Stop()
}
type pubsubPublishResult interface {
	Get(ctx context.Context) (string, error)
}

// Real implementations
type realPubsubClient struct{ c *pubsub.Client }

func (r *realPubsubClient) Topic(name string) pubsubTopic {
	return &realPubsubTopic{t: r.c.Topic(name)}
}
func (r *realPubsubClient) Close() error { return r.c.Close() }

type realPubsubTopic struct{ t *pubsub.Topic }

func (r *realPubsubTopic) Publish(ctx context.Context, msg *pubsub.Message) pubsubPublishResult {
	return &realPubsubPublishResult{r.t.Publish(ctx, msg)}
}
func (r *realPubsubTopic) Stop() { r.t.Stop() }

type realPubsubPublishResult struct{ r *pubsub.PublishResult }

func (r *realPubsubPublishResult) Get(ctx context.Context) (string, error) { return r.r.Get(ctx) }

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
	opts   GcpPubsubOptions
	client pubsubClient // interface for testability
}

func (s *gcpPubsubService) Send(notif Notification, dest Destination) error {
	ctx := context.Background()
	var client pubsubClient
	var err error

	// Use injected client if present (for testing)
	if s.client != nil {
		client = s.client
	} else {
		// Use key file if provided, else default creds (enables GKE Workload Identity)
		var realClient *pubsub.Client
		if s.opts.KeyFile != "" {
			realClient, err = pubsub.NewClient(ctx, s.opts.Project, option.WithCredentialsFile(s.opts.KeyFile))
		} else {
			realClient, err = pubsub.NewClient(ctx, s.opts.Project)
		}
		if err != nil {
			log.Errorf("failed to create pubsub client: %v", err)
			return err
		}
		defer realClient.Close()
		client = &realPubsubClient{c: realClient}
	}

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
