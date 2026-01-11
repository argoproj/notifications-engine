package services

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrafana_SuccessfullySendsNotification(t *testing.T) {
	var receivedHeaders http.Header
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		receivedHeaders = request.Header
		data, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		receivedBody = string(data)
	}))
	defer server.Close()

	service := NewGrafanaService(GrafanaOptions{
		ApiUrl:             server.URL,
		ApiKey:             "something-secret-but-not-relevant-in-this-test",
		InsecureSkipVerify: true,
	})
	err := service.Send(
		Notification{
			Message: "Annotation description",
		}, Destination{Recipient: "tag1|tag2", Service: "test-service"})
	require.NoError(t, err)

	assert.Contains(t, receivedBody, "tag1")
	assert.Contains(t, receivedBody, "tag2")
	assert.Contains(t, receivedBody, "Annotation description")
	assert.Contains(t, receivedHeaders.Get("Authorization"), "Bearer")
}

func TestGrafana_UnSuccessfullySendsNotification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	service := NewGrafanaService(GrafanaOptions{
		ApiUrl: server.URL,
		ApiKey: "something-secret-but-not-relevant-in-this-test",

		InsecureSkipVerify: true,
	})
	err := service.Send(
		Notification{}, Destination{Recipient: "tag1|tag2", Service: "test-service"})
	require.Error(t, err)
}
