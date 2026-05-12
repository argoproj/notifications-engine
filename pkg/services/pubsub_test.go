package services

import (
	"context"
	"errors"
	"testing"
	"text/template"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSend_GcpPubsub(t *testing.T) {
	savePublishPubsubMessage := PublishPubsubMessage
	defer func() { PublishPubsubMessage = savePublishPubsubMessage }()

	var capturedProject, capturedTopic string
	var capturedMsg *pubsub.Message
	PublishPubsubMessage = func(_ context.Context, projectID, _, topicName string, msg *pubsub.Message) (string, error) {
		capturedProject = projectID
		capturedTopic = topicName
		capturedMsg = msg
		return "mock-message-id", nil
	}

	service := NewGcpPubsubService(GcpPubsubOptions{
		Project: "test-project",
		Topic:   "test-topic",
	})

	notification := Notification{
		Message:   "hello world",
		GcpPubsub: &GcpPubsubNotification{Attributes: map[string]string{"foo": "bar"}},
	}
	dest := Destination{Recipient: ""}

	err := service.Send(notification, dest)
	require.NoError(t, err)
	assert.Equal(t, "test-project", capturedProject)
	assert.Equal(t, "test-topic", capturedTopic)
	assert.Equal(t, "hello world", string(capturedMsg.Data))
	assert.Equal(t, map[string]string{"foo": "bar"}, capturedMsg.Attributes)
}

func TestSend_GcpPubsub_WithRecipient(t *testing.T) {
	savePublishPubsubMessage := PublishPubsubMessage
	defer func() { PublishPubsubMessage = savePublishPubsubMessage }()

	var capturedProject, capturedTopicName string
	PublishPubsubMessage = func(_ context.Context, projectID, _, topicName string, _ *pubsub.Message) (string, error) {
		capturedProject = projectID
		capturedTopicName = topicName
		return "mock-message-id", nil
	}

	service := NewGcpPubsubService(GcpPubsubOptions{
		Project: "test-project",
		Topic:   "default-topic",
	})

	notification := Notification{Message: "hello"}
	dest := Destination{Recipient: "override-topic"}

	err := service.Send(notification, dest)
	require.NoError(t, err)
	assert.Equal(t, "test-project", capturedProject)
	assert.Equal(t, "override-topic", capturedTopicName)
}

func TestSend_GcpPubsub_PublishError(t *testing.T) {
	savePublishPubsubMessage := PublishPubsubMessage
	defer func() { PublishPubsubMessage = savePublishPubsubMessage }()

	PublishPubsubMessage = func(_ context.Context, _, _, _ string, _ *pubsub.Message) (string, error) {
		return "", errors.New("publish error")
	}

	service := NewGcpPubsubService(GcpPubsubOptions{
		Project: "test-project",
		Topic:   "test-topic",
	})

	notification := Notification{Message: "hello"}
	dest := Destination{}

	err := service.Send(notification, dest)
	require.Error(t, err)
}

func TestGetTemplater_GcpPubsub(t *testing.T) {
	n := Notification{
		Message: "{{.message}}",
		GcpPubsub: &GcpPubsubNotification{
			Attributes: map[string]string{
				"app":       "{{.app}}",
				"namespace": "{{.namespace}}",
			},
		},
	}

	templater, err := n.GetTemplater("", template.FuncMap{})
	require.NoError(t, err)

	var notification Notification

	err = templater(&notification, map[string]any{
		"message":   "deployment ready",
		"app":       "my-app",
		"namespace": "production",
	})

	require.NoError(t, err)
	assert.Equal(t, "deployment ready", notification.Message)
	assert.Equal(t, map[string]string{
		"app":       "my-app",
		"namespace": "production",
	}, notification.GcpPubsub.Attributes)
}
