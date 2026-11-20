package http

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServiceHTTPClient_RequestTimeout(t *testing.T) {
	t.Run("empty requestTimeout leaves client.Timeout unset", func(t *testing.T) {
		client, err := NewServiceHTTPClient(TransportOptions{}, false, "https://example.com", "test")
		require.NoError(t, err)
		assert.Zero(t, client.Timeout)
	})

	t.Run("valid requestTimeout sets client.Timeout", func(t *testing.T) {
		client, err := NewServiceHTTPClient(TransportOptions{RequestTimeout: "30s"}, false, "https://example.com", "test")
		require.NoError(t, err)
		assert.Equal(t, 30*time.Second, client.Timeout)
	})

	t.Run("invalid requestTimeout returns an error", func(t *testing.T) {
		client, err := NewServiceHTTPClient(TransportOptions{RequestTimeout: "30sec"}, false, "https://example.com", "test")
		require.Error(t, err)
		assert.Nil(t, client)
	})
}
