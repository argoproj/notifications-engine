package http

import (
	"net/http"
	"net/http/httptest"
	"net/url"
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

	t.Run("aborts a request to a server that never responds", func(t *testing.T) {
		// Hang until the client gives up: the Go server cancels the request context when
		// the client closes the connection, so the handler returns and server.Close()
		// doesn't block on an outstanding request.
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()

		client, err := NewServiceHTTPClient(TransportOptions{RequestTimeout: "50ms"}, false, server.URL, "test")
		require.NoError(t, err)

		_, err = client.Get(server.URL)
		require.Error(t, err)

		var urlErr *url.Error
		require.ErrorAs(t, err, &urlErr)
		assert.True(t, urlErr.Timeout(), "expected a timeout error, got %v", err)
	})

	t.Run("does not abort a request the server answers in time", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client, err := NewServiceHTTPClient(TransportOptions{RequestTimeout: "5s"}, false, server.URL, "test")
		require.NoError(t, err)

		resp, err := client.Get(server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
