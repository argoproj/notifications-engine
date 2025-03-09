package services

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

func TestGetTemplater_Newrelic(t *testing.T) {
	n := Notification{
		Newrelic: &NewrelicNotification{
			Changelog:   "Added: /v2/deployments.rb",
			Description: "Deployment finished for {{.app.metadata.name}}. Visit: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
			User:        "{{.context.user}}",
		},
	}

	templater, err := n.GetTemplater("newrelic", template.FuncMap{})
	if !assert.NoError(t, err) {
		return
	}

	var notification Notification

	err = templater(&notification, map[string]interface{}{
		"context": map[string]interface{}{
			"argocdUrl": "https://example.com",
			"user":      "somebot",
		},
		"app": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "argocd-notifications",
			},
			"status": map[string]interface{}{
				"operationState": map[string]interface{}{
					"syncResult": map[string]interface{}{
						"revision": "0123456789",
					},
				},
			},
		},
	})

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "0123456789", notification.Newrelic.Revision)
	assert.Equal(t, "Added: /v2/deployments.rb", notification.Newrelic.Changelog)
	assert.Equal(t, "Deployment finished for argocd-notifications. Visit: https://example.com/applications/argocd-notifications", notification.Newrelic.Description)
	assert.Equal(t, "somebot", notification.Newrelic.User)
}

func TestSend_Newrelic(t *testing.T) {
	t.Run("revision deployment marker", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			if !assert.NoError(t, err) {
				t.FailNow()
			}

			assert.Equal(t, r.URL.Path, "/v2/applications/123456789/deployments.json")
			assert.Equal(t, r.Header["Content-Type"], []string{"application/json"})
			assert.Equal(t, r.Header["X-Api-Key"], []string{"NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ"})

			assert.JSONEq(t, `{
				"deployment": {
					"revision": "2027ed5",
					"description": "message",
					"user": "datanerd@example.com"
				}
			}`, string(b))
		}))
		defer ts.Close()

		service := NewNewrelicService(NewrelicOptions{
			ApiKey: "NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ",
			ApiURL: ts.URL,
		})
		err := service.Send(Notification{
			Message: "message",
			Newrelic: &NewrelicNotification{
				Revision: "2027ed5",
				User:     "datanerd@example.com",
			},
		}, Destination{
			Service:   "newrelic",
			Recipient: "123456789",
		})

		if !assert.NoError(t, err) {
			t.FailNow()
		}
	})

	t.Run("complete deployment marker", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			if !assert.NoError(t, err) {
				t.FailNow()
			}

			assert.Equal(t, r.URL.Path, "/v2/applications/123456789/deployments.json")
			assert.Equal(t, r.Header["Content-Type"], []string{"application/json"})
			assert.Equal(t, r.Header["X-Api-Key"], []string{"NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ"})

			assert.JSONEq(t, `{
				"deployment": {
					"revision": "2027ed5",
					"changelog": "Added: /v2/deployments.rb, Removed: None",
					"description": "Added a deployments resource to the v2 API",
					"user": "datanerd@example.com"
				}
			}`, string(b))
		}))
		defer ts.Close()

		service := NewNewrelicService(NewrelicOptions{
			ApiKey: "NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ",
			ApiURL: ts.URL,
		})
		err := service.Send(Notification{
			Message: "message",
			Newrelic: &NewrelicNotification{
				Revision:    "2027ed5",
				Changelog:   "Added: /v2/deployments.rb, Removed: None",
				Description: "Added a deployments resource to the v2 API",
				User:        "datanerd@example.com",
			},
		}, Destination{
			Service:   "newrelic",
			Recipient: "123456789",
		})

		if !assert.NoError(t, err) {
			t.FailNow()
		}
	})

	t.Run("recipient is application name", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v2/applications.json" {
				_, err := w.Write([]byte(`{
					"applications": [
						{"id": "123456789"}
					]
				}`))
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				return
			}

			b, err := io.ReadAll(r.Body)
			if !assert.NoError(t, err) {
				t.FailNow()
			}

			assert.Equal(t, "/v2/applications/123456789/deployments.json", r.URL.Path)
			assert.Equal(t, []string{"application/json"}, r.Header["Content-Type"])
			assert.Equal(t, []string{"NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ"}, r.Header["X-Api-Key"])

			assert.JSONEq(t, `{
				"deployment": {
					"revision": "2027ed5",
					"description": "message",
					"user": "datanerd@example.com"
				}
			}`, string(b))
		}))
		defer ts.Close()

		service := NewNewrelicService(NewrelicOptions{
			ApiKey: "NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ",
			ApiURL: ts.URL,
		})
		err := service.Send(Notification{
			Message: "message",
			Newrelic: &NewrelicNotification{
				Revision: "2027ed5",
				User:     "datanerd@example.com",
			},
		}, Destination{
			Service:   "newrelic",
			Recipient: "myapp",
		})

		if !assert.NoError(t, err) {
			t.FailNow()
		}
	})

	t.Run("missing config", func(t *testing.T) {
		service := NewNewrelicService(NewrelicOptions{
			ApiKey: "NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ",
		})
		err := service.Send(Notification{
			Message: "message",
		}, Destination{
			Service:   "newrelic",
			Recipient: "123456789",
		})

		if assert.Error(t, err) {
			assert.Equal(t, err, ErrMissingConfig)
		}
	})

	t.Run("missing apiKey", func(t *testing.T) {
		service := NewNewrelicService(NewrelicOptions{})
		err := service.Send(Notification{
			Message: "message",
		}, Destination{
			Service:   "newrelic",
			Recipient: "123456789",
		})

		if assert.Error(t, err) {
			assert.Equal(t, err, ErrMissingApiKey)
		}
	})
}

func TestGetApplicationId(t *testing.T) {
	t.Run("successful lookup by application name", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v2/applications.json", r.URL.Path)
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "myapp", r.URL.Query().Get("filter[name]"))
			assert.Equal(t, []string{"application/json"}, r.Header["Content-Type"])
			assert.Equal(t, []string{"NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ"}, r.Header["X-Api-Key"])

			_, err := w.Write([]byte(`{
				"applications": [
					{"id": "123456789"}
				]
			}`))
			if !assert.NoError(t, err) {
				t.FailNow()
			}
		}))
		defer ts.Close()
		service := NewNewrelicService(NewrelicOptions{
			ApiKey: "NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ",
			ApiURL: ts.URL,
		}).(*newrelicService)
		appId, err := service.getApplicationId(http.DefaultClient, "myapp")
		assert.NoError(t, err)
		assert.Equal(t, "123456789", appId)
	})

	t.Run("application not found", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{"applications": []}`))
			if !assert.NoError(t, err) {
				t.FailNow()
			}
		}))
		defer ts.Close()
		service := NewNewrelicService(NewrelicOptions{
			ApiKey: "NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ",
			ApiURL: ts.URL,
		}).(*newrelicService)
		_, err := service.getApplicationId(http.DefaultClient, "myapp")
		assert.Equal(t, ErrAppIdNoMatches, err)
	})

	t.Run("multiple matches for application name", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(`{
				"applications": [
					{"id": "123456789"},
					{"id": "987654321"}
				]
		  }`))
			if !assert.NoError(t, err) {
				t.FailNow()
			}
		}))
		defer ts.Close()
		service := NewNewrelicService(NewrelicOptions{
			ApiKey: "NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ",
			ApiURL: ts.URL,
		}).(*newrelicService)
		_, err := service.getApplicationId(http.DefaultClient, "myapp")
		assert.Equal(t, ErrAppIdMultipleMatches, err)
	})
}
