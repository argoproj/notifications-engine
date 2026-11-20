package services

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
)

func TestWebhook_SuccessfullySendsNotification(t *testing.T) {
	var receivedHeaders http.Header
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		receivedHeaders = request.Header
		data, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		receivedBody = string(data)
	}))
	defer server.Close()

	service := NewWebhookService(WebhookOptions{
		BasicAuth:          &BasicAuth{Username: "testUsername", Password: "testPassword"},
		URL:                server.URL,
		Headers:            []Header{{Name: "testHeader", Value: "testHeaderValue"}},
		InsecureSkipVerify: true,
	})
	err := service.Send(
		Notification{
			Webhook: map[string]WebhookNotification{
				"test": {Body: "hello world", Method: http.MethodPost},
			},
		}, Destination{Recipient: "test", Service: "test"})
	require.NoError(t, err)

	assert.Equal(t, "hello world", receivedBody)
	assert.Equal(t, "testHeaderValue", receivedHeaders.Get("testHeader"))
	assert.Contains(t, receivedHeaders.Get("Authorization"), "Basic")
}

func TestWebhook_WithNoOverrides_SuccessfullySendsNotification(t *testing.T) {
	var receivedHeaders http.Header
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		receivedHeaders = request.Header
		data, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		receivedBody = string(data)
	}))
	defer server.Close()

	service := NewWebhookService(WebhookOptions{
		BasicAuth: &BasicAuth{Username: "testUsername", Password: "testPassword"},
		URL:       server.URL,
		Headers:   []Header{{Name: "testHeader", Value: "testHeaderValue"}},
	})
	err := service.Send(Notification{}, Destination{Recipient: "test", Service: "test"})
	require.NoError(t, err)

	assert.Empty(t, receivedBody)
	assert.Equal(t, "testHeaderValue", receivedHeaders.Get("testHeader"))
	assert.Contains(t, receivedHeaders.Get("Authorization"), "Basic")
}

func TestWebhook_SubPath(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
	}))
	defer server.Close()

	service := NewWebhookService(WebhookOptions{
		URL: server.URL + "/subpath1",
	})

	err := service.Send(Notification{
		Webhook: map[string]WebhookNotification{
			"test": {Body: "hello world", Method: http.MethodPost},
		},
	}, Destination{Recipient: "test", Service: "test"})
	require.NoError(t, err)
	assert.Equal(t, "/subpath1", receivedPath)

	err = service.Send(Notification{
		Webhook: map[string]WebhookNotification{
			"test": {Body: "hello world", Method: http.MethodPost, Path: "/subpath2"},
		},
	}, Destination{Recipient: "test", Service: "test"})
	require.NoError(t, err)
	assert.Equal(t, "/subpath1/subpath2", receivedPath)
}

func TestGetTemplater_Webhook(t *testing.T) {
	n := Notification{
		Webhook: WebhookNotifications{
			"github": {
				Method: "POST",
				Body:   "{{.foo}}",
				Path:   "{{.bar}}",
			},
		},
	}

	templater, err := n.GetTemplater("", template.FuncMap{})
	require.NoError(t, err)

	var notification Notification
	err = templater(&notification, map[string]any{
		"foo": "hello",
		"bar": "world",
	})

	require.NoError(t, err)

	assert.Equal(t, "POST", notification.Webhook["github"].Method)
	assert.Equal(t, "hello", notification.Webhook["github"].Body)
	assert.Equal(t, "world", notification.Webhook["github"].Path)
}

func TestWebhookService_Send_Retry(t *testing.T) {
	// Set up a mock server to receive requests
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count++
		if count < 5 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	service := NewWebhookService(WebhookOptions{
		BasicAuth:          &BasicAuth{Username: "testUsername", Password: "testPassword"},
		URL:                server.URL,
		Headers:            []Header{{Name: "testHeader", Value: "testHeaderValue"}},
		InsecureSkipVerify: true,
	})
	err := service.Send(
		Notification{
			Webhook: map[string]WebhookNotification{
				"test": {Body: "hello world", Method: http.MethodPost},
			},
		}, Destination{Recipient: "test", Service: "test"})

	// Check if the error is due to a server error after retries
	if !strings.Contains(err.Error(), "giving up after 4 attempt(s)") {
		t.Errorf("Expected 'giving up after 4 attempt(s)' substring, got %v", err)
	}

	// Check that the mock server received 4 requests
	if count != 4 {
		t.Errorf("Expected 4 requests, got %d", count)
	}
}

func TestWebhookService_Send_RequestTimeout(t *testing.T) {
	// A handler that never responds within the test's lifetime, simulating a destination
	// that accepts the connection but hangs.
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(10 * time.Second)
	}))
	defer server.Close()

	service := NewWebhookService(WebhookOptions{
		URL: server.URL,
		// RetryMax is explicitly non-zero so it isn't overwritten by NewWebhookService's
		// zero-value default of 3; small retry waits keep the test fast regardless.
		RetryMax:     1,
		RetryWaitMin: 1 * time.Millisecond,
		RetryWaitMax: 1 * time.Millisecond,
		TransportOptions: httputil.TransportOptions{
			RequestTimeout: "50ms",
		},
	})

	start := time.Now()
	err := service.Send(
		Notification{
			Webhook: map[string]WebhookNotification{
				"test": {Body: "hello world", Method: http.MethodPost},
			},
		}, Destination{Recipient: "test", Service: "test"})
	elapsed := time.Since(start)

	require.Error(t, err)
	// Bounded by (RetryMax+1) attempts at ~50ms each, plus negligible backoff - nowhere
	// near the handler's 10s sleep. This is the behavior the requestTimeout option exists
	// to guarantee.
	assert.Less(t, elapsed, 5*time.Second)
}

func TestWebhookService_Send_InvalidRequestTimeout(t *testing.T) {
	service := NewWebhookService(WebhookOptions{
		URL: "http://127.0.0.1:0",
		TransportOptions: httputil.TransportOptions{
			RequestTimeout: "30sec",
		},
	})

	err := service.Send(
		Notification{
			Webhook: map[string]WebhookNotification{
				"test": {Body: "hello world", Method: http.MethodPost},
			},
		}, Destination{Recipient: "test", Service: "test"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "requestTimeout")
}
