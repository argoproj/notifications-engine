package services

import (
	"context"
	"errors"
	"testing"
	"text/template"

	kafka "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSend_Kafka(t *testing.T) {
	save := PublishKafkaMessage
	defer func() { PublishKafkaMessage = save }()

	var capturedOpts KafkaOptions
	var capturedTopic string
	var capturedMsg kafka.Message
	PublishKafkaMessage = func(_ context.Context, opts KafkaOptions, topic string, msg kafka.Message) error {
		capturedOpts = opts
		capturedTopic = topic
		capturedMsg = msg
		return nil
	}

	service := NewKafkaService(KafkaOptions{
		Brokers: []string{"broker-1:9092", "broker-2:9092"},
		Topic:   "argocd-notifications",
	})

	err := service.Send(Notification{Message: "hello world"}, Destination{Service: "kafka"})
	require.NoError(t, err)
	assert.Equal(t, []string{"broker-1:9092", "broker-2:9092"}, capturedOpts.Brokers)
	assert.Equal(t, "argocd-notifications", capturedTopic)
	assert.Equal(t, "hello world", string(capturedMsg.Value))
	assert.Nil(t, capturedMsg.Key)
}

func TestSend_Kafka_WithRecipient(t *testing.T) {
	save := PublishKafkaMessage
	defer func() { PublishKafkaMessage = save }()

	var capturedTopic string
	PublishKafkaMessage = func(_ context.Context, _ KafkaOptions, topic string, _ kafka.Message) error {
		capturedTopic = topic
		return nil
	}

	service := NewKafkaService(KafkaOptions{Brokers: []string{"b:9092"}, Topic: "default-topic"})
	err := service.Send(Notification{Message: "hi"}, Destination{Recipient: "override-topic"})
	require.NoError(t, err)
	assert.Equal(t, "override-topic", capturedTopic)
}

func TestSend_Kafka_WithKey(t *testing.T) {
	save := PublishKafkaMessage
	defer func() { PublishKafkaMessage = save }()

	var capturedMsg kafka.Message
	PublishKafkaMessage = func(_ context.Context, _ KafkaOptions, _ string, msg kafka.Message) error {
		capturedMsg = msg
		return nil
	}

	service := NewKafkaService(KafkaOptions{Brokers: []string{"b:9092"}, Topic: "t"})
	err := service.Send(
		Notification{Message: "body", Kafka: &KafkaNotification{Key: "my-app"}},
		Destination{},
	)
	require.NoError(t, err)
	assert.Equal(t, "my-app", string(capturedMsg.Key))
	assert.Equal(t, "body", string(capturedMsg.Value))
}

func TestSend_Kafka_NoTopic(t *testing.T) {
	save := PublishKafkaMessage
	defer func() { PublishKafkaMessage = save }()
	PublishKafkaMessage = func(_ context.Context, _ KafkaOptions, _ string, _ kafka.Message) error {
		t.Fatal("publish must not be called when no topic is configured")
		return nil
	}

	service := NewKafkaService(KafkaOptions{Brokers: []string{"b:9092"}})
	err := service.Send(Notification{Message: "x"}, Destination{})
	require.ErrorContains(t, err, "no topic configured")
}

func TestSend_Kafka_PublishError(t *testing.T) {
	save := PublishKafkaMessage
	defer func() { PublishKafkaMessage = save }()
	PublishKafkaMessage = func(_ context.Context, _ KafkaOptions, _ string, _ kafka.Message) error {
		return errors.New("broker unreachable")
	}

	service := NewKafkaService(KafkaOptions{Brokers: []string{"b:9092"}, Topic: "t"})
	err := service.Send(Notification{Message: "x"}, Destination{})
	require.Error(t, err)
}

func TestGetTemplater_Kafka(t *testing.T) {
	n := Notification{
		Message: "{{.message}}",
		Kafka:   &KafkaNotification{Key: "{{.app}}"},
	}

	templater, err := n.GetTemplater("", template.FuncMap{})
	require.NoError(t, err)

	var notification Notification
	err = templater(&notification, map[string]any{
		"message": "deployment ready",
		"app":     "my-app",
	})
	require.NoError(t, err)
	assert.Equal(t, "deployment ready", notification.Message)
	assert.Equal(t, "my-app", notification.Kafka.Key)
}

func TestKafka_newSASLMechanism(t *testing.T) {
	// default + explicit plain
	m, err := newSASLMechanism(&KafkaSASL{Username: "u", Password: "p"})
	require.NoError(t, err)
	assert.Equal(t, plain.Mechanism{Username: "u", Password: "p"}, m)

	m, err = newSASLMechanism(&KafkaSASL{Mechanism: "PLAIN", Username: "u", Password: "p"})
	require.NoError(t, err)
	assert.Equal(t, "PLAIN", m.Name())

	for _, mech := range []string{"scram-sha-256", "scram-sha-512"} {
		m, err := newSASLMechanism(&KafkaSASL{Mechanism: mech, Username: "u", Password: "p"})
		require.NoError(t, err, mech)
		assert.NotNil(t, m)
	}

	_, err = newSASLMechanism(&KafkaSASL{Mechanism: "bogus"})
	require.ErrorContains(t, err, "unsupported SASL mechanism")
}

func TestNewService_Kafka(t *testing.T) {
	svc, err := NewService("kafka", []byte("brokers:\n  - localhost:9092\ntopic: my-topic\n"))
	require.NoError(t, err)
	require.NotNil(t, svc)
}
