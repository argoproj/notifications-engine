package services

import (
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"
)

func TestNatsService_Send(t *testing.T) {
	t.Run("basic successful send without authentication", func(t *testing.T) {
		server, err := natsserver.NewServer(&natsserver.Options{
			ServerName: "test-nats-server",
			DontListen: true, // no tcp socket
		})
		if err != nil {
			t.Error(err)
		}
		defer server.Shutdown()

		server.Start()

		if !server.ReadyForConnections(5 * time.Second) {
			t.Error("NATS server not ready for connections")
		}

		nc, err := nats.Connect(server.ClientURL(), nats.InProcessServer(server))
		if err != nil {
			t.Error("failed to connect to NATS server:", err)
		}
		sub, err := nc.SubscribeSync("test-topic")
		if err != nil {
			t.Error("failed to subscribe to test-topic:", err)
		}

		natsService := NewNatsService(NatsOptions{
			Url:     server.ClientURL(),
			Headers: map[string]string{"foo": "bar"},
		}, nats.InProcessServer(server))

		err = natsService.Send(Notification{Message: "message"}, Destination{Service: "nats", Recipient: "test-topic"})
		if err != nil {
			t.Error("Failed to send message:", err)
		}

		receivedMsg, err := sub.NextMsg(time.Second)
		if err != nil {
			t.Error("Failed to receive sent message:", err)
		}

		assert.NotNil(t, receivedMsg)
		assert.Equal(t, "message", string(receivedMsg.Data), "Received message does not match sent message")
		assert.Equal(t, "bar", receivedMsg.Header.Get("foo"), "Received message header does not match expected value")
	})

	t.Run("basic successful send with username and password", func(t *testing.T) {
		server, err := natsserver.NewServer(&natsserver.Options{
			ServerName: "test-nats-server",
			DontListen: true, // no tcp socket
			Users: []*natsserver.User{
				{
					Username: "testuser",
					Password: "testpassword",
				},
			},
		})
		if err != nil {
			t.Error(err)
		}
		defer server.Shutdown()

		server.Start()

		if !server.ReadyForConnections(5 * time.Second) {
			t.Error("NATS server not ready for connections")
		}

		nc, err := nats.Connect(server.ClientURL(), nats.InProcessServer(server), nats.UserInfo("testuser", "testpassword"))
		if err != nil {
			t.Error("failed to connect to NATS server:", err)
		}
		sub, err := nc.SubscribeSync("test-topic")
		if err != nil {
			t.Error("failed to subscribe to test-topic:", err)
		}

		natsService := NewNatsService(NatsOptions{
			Url:      server.ClientURL(),
			Headers:  map[string]string{"foo": "bar"},
			Username: "testuser",
			Password: "testpassword",
		}, nats.InProcessServer(server))

		err = natsService.Send(Notification{Message: "message"}, Destination{Service: "nats", Recipient: "test-topic"})
		if err != nil {
			t.Error("Failed to send message:", err)
		}

		receivedMsg, err := sub.NextMsg(time.Second)
		if err != nil {
			t.Error("Failed to receive sent message:", err)
		}

		assert.NotNil(t, receivedMsg)
		assert.Equal(t, "message", string(receivedMsg.Data), "Received message does not match sent message")
		assert.Equal(t, "bar", receivedMsg.Header.Get("foo"), "Received message header does not match expected value")
	})

	t.Run("basic successful send with nkey authentication", func(t *testing.T) {
		// Generate NKey pair
		kp, err := nkeys.CreatePair(nkeys.PrefixByteUser)
		assert.NoError(t, err)
		defer kp.Wipe()

		publicKey, err := kp.PublicKey()
		assert.NoError(t, err)

		seed, err := kp.Seed()
		assert.NoError(t, err)

		// Start NATS server with NKey-based authorization
		server, err := natsserver.NewServer(&natsserver.Options{
			ServerName: "test-nats-server-nkey",
			DontListen: true,
			Nkeys: []*natsserver.NkeyUser{{
				Nkey: publicKey,
			}},
		})
		assert.NoError(t, err)
		defer server.Shutdown()

		server.Start()
		assert.True(t, server.ReadyForConnections(5*time.Second), "NATS server not ready")

		// Connect and subscribe using NKey auth
		nc, err := nats.Connect(server.ClientURL(),
			nats.InProcessServer(server),
			nats.Nkey(publicKey, func(nonce []byte) ([]byte, error) {
				return kp.Sign(nonce)
			}),
		)
		assert.NoError(t, err)

		sub, err := nc.SubscribeSync("test-topic")
		assert.NoError(t, err)

		// Use service with seed
		natsService := NewNatsService(NatsOptions{
			Url:     server.ClientURL(),
			Headers: map[string]string{"foo": "bar"},
			NKey:    string(seed),
		}, nats.InProcessServer(server))

		err = natsService.Send(Notification{Message: "message"}, Destination{Service: "nats", Recipient: "test-topic"})
		assert.NoError(t, err)

		receivedMsg, err := sub.NextMsg(time.Second)
		assert.NoError(t, err)

		assert.NotNil(t, receivedMsg)
		assert.Equal(t, "message", string(receivedMsg.Data))
		assert.Equal(t, "bar", receivedMsg.Header.Get("foo"))
	})
}
