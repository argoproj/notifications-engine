package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	texttemplate "text/template"

	kafka "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	log "github.com/sirupsen/logrus"
)

// KafkaOptions holds the configuration for the Kafka notification service.
type KafkaOptions struct {
	// Brokers is the list of Kafka broker addresses (host:port) to connect to.
	Brokers []string `json:"brokers"`
	// Topic is the Kafka topic to publish to. It can be overridden per
	// subscription by the recipient (e.g. subscribe.<trigger>.kafka: my-topic).
	Topic string `json:"topic,omitempty"`
	// TLS optionally enables TLS for the broker connection.
	TLS *KafkaTLS `json:"tls,omitempty"`
	// SASL optionally enables SASL authentication.
	SASL *KafkaSASL `json:"sasl,omitempty"`
}

// KafkaTLS configures TLS for the Kafka connection.
type KafkaTLS struct {
	Enabled            bool `json:"enabled,omitempty"`
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// KafkaSASL configures SASL authentication for the Kafka connection.
type KafkaSASL struct {
	// Mechanism is one of "plain", "scram-sha-256" or "scram-sha-512".
	// Defaults to "plain" when a username is set and no mechanism is given.
	Mechanism string `json:"mechanism,omitempty"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
}

// KafkaNotification holds per-notification Kafka options that support templating.
type KafkaNotification struct {
	// Key is the optional message key. Kafka uses the key to select the
	// partition, so records with the same key preserve their order. It is
	// rendered as a template, e.g. "{{.app.metadata.name}}".
	Key string `json:"key,omitempty"`
}

func (n *KafkaNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	keyTmpl, err := texttemplate.New(name).Funcs(f).Parse(n.Key)
	if err != nil {
		return nil, err
	}
	return func(notification *Notification, vars map[string]any) error {
		if notification.Kafka == nil {
			notification.Kafka = &KafkaNotification{}
		}
		var keyData bytes.Buffer
		if err := keyTmpl.Execute(&keyData, vars); err != nil {
			return err
		}
		notification.Kafka.Key = keyData.String()
		return nil
	}, nil
}

func NewKafkaService(opts KafkaOptions) NotificationService {
	return &kafkaService{opts: opts}
}

type kafkaService struct {
	opts KafkaOptions
}

// Send implements NotificationService. It publishes the rendered notification
// message to a Kafka topic. The message body is fully controlled by the
// template, so any envelope (e.g. CloudEvents) can be produced by the template
// itself.
func (s *kafkaService) Send(notif Notification, dest Destination) error {
	topic := s.opts.Topic
	if dest.Recipient != "" {
		topic = dest.Recipient
	}
	if topic == "" {
		return fmt.Errorf("kafka: no topic configured (set the service `topic` or the subscription recipient)")
	}

	msg := kafka.Message{
		Value: []byte(notif.Message),
	}
	if notif.Kafka != nil && notif.Kafka.Key != "" {
		msg.Key = []byte(notif.Kafka.Key)
	}

	if err := PublishKafkaMessage(context.Background(), s.opts, topic, msg); err != nil {
		log.Errorf("failed to publish kafka message: %v", err)
		return err
	}
	return nil
}

// PublishKafkaMessage publishes a single message to Kafka. It is a package-level
// variable so it can be overridden in tests without a running broker.
var PublishKafkaMessage = func(ctx context.Context, opts KafkaOptions, topic string, msg kafka.Message) error {
	transport := &kafka.Transport{}

	if opts.TLS != nil && opts.TLS.Enabled {
		transport.TLS = &tls.Config{
			//nolint:gosec // InsecureSkipVerify is opt-in via config for private/self-signed clusters.
			InsecureSkipVerify: opts.TLS.InsecureSkipVerify,
		}
	}

	if opts.SASL != nil {
		mechanism, err := newSASLMechanism(opts.SASL)
		if err != nil {
			return err
		}
		transport.SASL = mechanism
	}

	writer := &kafka.Writer{
		Addr:      kafka.TCP(opts.Brokers...),
		Topic:     topic,
		Balancer:  &kafka.Hash{},
		Transport: transport,
	}
	defer func() { _ = writer.Close() }()

	if err := writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to publish message to Kafka topic %q: %w", topic, err)
	}
	return nil
}

func newSASLMechanism(cfg *KafkaSASL) (sasl.Mechanism, error) {
	switch strings.ToLower(cfg.Mechanism) {
	case "", "plain":
		return plain.Mechanism{Username: cfg.Username, Password: cfg.Password}, nil
	case "scram-sha-256":
		return scram.Mechanism(scram.SHA256, cfg.Username, cfg.Password)
	case "scram-sha-512":
		return scram.Mechanism(scram.SHA512, cfg.Username, cfg.Password)
	default:
		return nil, fmt.Errorf("unsupported SASL mechanism %q (supported: plain, scram-sha-256, scram-sha-512)", cfg.Mechanism)
	}
}
