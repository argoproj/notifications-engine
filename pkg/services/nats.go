package services

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

type NatsNotification struct{}

type NatsOptions struct {
	// Url is the NATS server URL to connect to
	// e.g. nats://nats.nats.svc.cluster.local:4222
	Url string `json:"url"`
	// Headers is an optional map of headers to include in the NATS message
	Headers map[string]string `json:"headers,omitempty"`
	// NKey is optional for nkey authentication
	NKey string `json:"nkey,omitempty"`
	// Username and Password are optional for basic auth
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func NewNatsService(opts NatsOptions, defaultConnectionOpts ...nats.Option) NotificationService {
	return natsService{opts: opts, defaultConnectionOpts: defaultConnectionOpts}
}

type natsService struct {
	opts NatsOptions
	// defaultConnectionOpts are additional options to pass to nats.Connect
	defaultConnectionOpts []nats.Option
}

// Send implements NotificationService.
func (n natsService) Send(notification Notification, dest Destination) error {
	var options []nats.Option
	options = append(options, n.defaultConnectionOpts...)

	if n.opts.NKey != "" {
		// create nkey key pair from seed
		keyPair, err := nkeys.FromSeed([]byte(n.opts.NKey))
		if err != nil {
			return fmt.Errorf("failed to create NKey from seed: %w", err)
		}

		// get the public nkey
		publicKey, err := keyPair.PublicKey()
		if err != nil {
			return fmt.Errorf("failed to get public NKey: %w", err)
		}

		// add nkey authentication option
		options = append(options, nats.Nkey(publicKey, func(nonce []byte) ([]byte, error) {
			return keyPair.Sign(nonce)
		}))
	}

	if n.opts.Username != "" && n.opts.Password != "" {
		options = append(options, nats.UserInfo(n.opts.Username, n.opts.Password))
	}

	conn, err := nats.Connect(n.opts.Url, options...)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS server: %w", err)
	}
	defer conn.Close()

	msg := nats.NewMsg(dest.Recipient)
	msg.Data = []byte(notification.Message)

	if len(n.opts.Headers) > 0 {
		msg.Header = make(nats.Header)
		for key, value := range n.opts.Headers {
			msg.Header.Set(key, value)
		}
	}

	err = conn.PublishMsg(msg)
	if err != nil {
		return fmt.Errorf("failed to publish message to NATS: %w", err)
	}
	return nil
}
