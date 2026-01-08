package services

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

func TestWebhook_SuccessfullySendsNotification(t *testing.T) {
	var receivedHeaders http.Header
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedHeaders = request.Header
		data, err := io.ReadAll(request.Body)
		assert.NoError(t, err)
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
		}, Destination{Recipient: server.URL, Service: "test"})
	assert.NoError(t, err)

	assert.Equal(t, "hello world", receivedBody)
	assert.Equal(t, receivedHeaders.Get("testHeader"), "testHeaderValue")
	assert.Contains(t, receivedHeaders.Get("Authorization"), "Basic")
}

func TestWebhook_WithNoOverrides_OverrideRecipientWithUrl_SuccessfullySendsNotification(t *testing.T) {
	var receivedHeaders http.Header
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedHeaders = request.Header
		data, err := ioutil.ReadAll(request.Body)
		assert.NoError(t, err)
		receivedBody = string(data)
	}))
	defer server.Close()

	service := NewWebhookService(WebhookOptions{
		BasicAuth: &BasicAuth{Username: "testUsername", Password: "testPassword"},
		URL:       server.URL,
		Headers:   []Header{{Name: "testHeader", Value: "testHeaderValue"}},
	})
	err := service.Send(Notification{}, Destination{Recipient: server.URL, Service: "test"})
	assert.NoError(t, err)

	assert.Equal(t, "", receivedBody)
	assert.Equal(t, receivedHeaders.Get("testHeader"), "testHeaderValue")
	assert.Contains(t, receivedHeaders.Get("Authorization"), "Basic")
}

func TestWebhook_WithNoOverrides_SuccessfullySendsNotification(t *testing.T) {
	var receivedHeaders http.Header
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedHeaders = request.Header
		data, err := io.ReadAll(request.Body)
		assert.NoError(t, err)
		receivedBody = string(data)
	}))
	defer server.Close()

	service := NewWebhookService(WebhookOptions{
		BasicAuth: &BasicAuth{Username: "testUsername", Password: "testPassword"},
		URL:       server.URL,
		Headers:   []Header{{Name: "testHeader", Value: "testHeaderValue"}},
	})
	err := service.Send(Notification{}, Destination{Recipient: "test", Service: "test"})
	assert.NoError(t, err)

	assert.Equal(t, "", receivedBody)
	assert.Equal(t, receivedHeaders.Get("testHeader"), "testHeaderValue")
	assert.Contains(t, receivedHeaders.Get("Authorization"), "Basic")
}

func TestWebhook_SubPath_SuccessfullySendsNotification(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
	}))
	defer server.Close()

	service := NewWebhookService(WebhookOptions{
		URL: fmt.Sprintf("%s/subpath1", server.URL),
	})

	err := service.Send(Notification{
		Webhook: map[string]WebhookNotification{
			"test": {Body: "hello world", Method: http.MethodPost},
		},
	}, Destination{Recipient: "test", Service: "test"})
	assert.NoError(t, err)
	assert.Equal(t, "/subpath1", receivedPath)

	err = service.Send(Notification{
		Webhook: map[string]WebhookNotification{
			"test": {Body: "hello world", Method: http.MethodPost, Path: "/subpath2"},
		},
	}, Destination{Recipient: "test", Service: "test"})
	assert.NoError(t, err)
	assert.Equal(t, "/subpath1/subpath2", receivedPath)
}

func TestWebhook_SubPath_OverrideRecipientWithUrl_SuccessfullySendsNotification(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
	}))
	defer server.Close()

	recipientUrl := fmt.Sprintf("%s/subpath1", server.URL)

	service := NewWebhookService(WebhookOptions{
		URL: recipientUrl,
	})

	err := service.Send(Notification{
		Webhook: map[string]WebhookNotification{
			"test": {Body: "hello world", Method: http.MethodPost},
		},
	}, Destination{Recipient: recipientUrl, Service: "test"})
	assert.NoError(t, err)
	assert.Equal(t, "/subpath1", receivedPath)

	err = service.Send(Notification{
		Webhook: map[string]WebhookNotification{
			"test": {Body: "hello world", Method: http.MethodPost, Path: "/subpath2"},
		},
	}, Destination{Recipient: recipientUrl, Service: "test"})
	assert.NoError(t, err)
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
	if !assert.NoError(t, err) {
		return
	}

	var notification Notification
	err = templater(&notification, map[string]interface{}{
		"foo": "hello",
		"bar": "world",
	})

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, notification.Webhook["github"].Method, "POST")
	assert.Equal(t, notification.Webhook["github"].Body, "hello")
	assert.Equal(t, notification.Webhook["github"].Path, "world")
}

func TestWebhookService_Send_Retry(t *testing.T) {
	// Set up a mock server to receive requests
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
