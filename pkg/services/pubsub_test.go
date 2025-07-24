package services

import (
	"context"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
)

// --- Mocks ---
type mockTopic struct {
	publishedMsg  string
	publishedAttr map[string]string
	publishErr    error
}

func (m *mockTopic) Publish(ctx context.Context, msg *pubsub.Message) pubsubPublishResult {
	m.publishedMsg = string(msg.Data)
	m.publishedAttr = msg.Attributes
	return &mockPublishResult{err: m.publishErr}
}
func (m *mockTopic) Stop() {}

type mockPublishResult struct {
	err error
}

func (m *mockPublishResult) Get(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return "mock-id", nil
}

type pubSubMockClient struct {
	topics map[string]*mockTopic
}

func (m *pubSubMockClient) Topic(name string) pubsubTopic {
	if t, ok := m.topics[name]; ok {
		return t
	}
	return nil
}
func (m *pubSubMockClient) Close() error { return nil }

// --- Test ---
func TestSend_GcpPubsub(t *testing.T) {
	topic := &mockTopic{}
	client := &pubSubMockClient{topics: map[string]*mockTopic{"test-topic": topic}}

	service := &gcpPubsubService{
		opts: GcpPubsubOptions{
			Project: "test-project",
			Topic:   "test-topic",
		},
		client: client,
	}

	notification := Notification{
		Message:   "hello world",
		GcpPubsub: &GcpPubsubNotification{Attributes: map[string]string{"foo": "bar"}},
	}
	dest := Destination{Recipient: ""}

	err := service.Send(notification, dest)
	assert.NoError(t, err)
	assert.Equal(t, "hello world", topic.publishedMsg)
	assert.Equal(t, map[string]string{"foo": "bar"}, topic.publishedAttr)
}
