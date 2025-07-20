package services

import (
	"context"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
)

// TestSend_GcpPubsub tests sending a message to Google Cloud Pub/Sub
// you can start the PubSub emulator locally with:
// `gcloud beta emulators pubsub start --project=test-project`
func TestSend_GcpPubsub(t *testing.T) {

	ctx := context.Background()
	projectID := "test-project"
	topicID := "test-topic"

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("failed to create pubsub client: %v", err)
	}
	defer client.Close()

	// Create topic
	topic, err := client.CreateTopic(ctx, topicID)
	if err != nil {
		t.Fatalf("failed to create topic: %v", err)
	}
	defer topic.Stop()

	service := NewGcpPubsubService(GcpPubsubOptions{
		Project: projectID,
		Topic:   topicID,
	})

	notification := Notification{
		Message:   "hello world",
		GcpPubsub: &GcpPubsubNotification{Attributes: map[string]string{"foo": "bar"}},
	}
	dest := Destination{Recipient: ""}

	err = service.Send(notification, dest)
	assert.NoError(t, err)
}
