package services

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidEmail(t *testing.T) {
	assert.Equal(t, true, validEmail.MatchString("test@test.com"))
	assert.Equal(t, true, validEmail.MatchString("test.test@test.com"))
	assert.Equal(t, false, validEmail.MatchString("notAnEmail"))
	assert.Equal(t, false, validEmail.MatchString("notAnEmail@"))
}

func TestSend_Webex(t *testing.T) {
	t.Run("successful attempt - email", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			if !assert.NoError(t, err) {
				t.FailNow()
			}

			assert.Equal(t, r.URL.Path, "/v1/messages")
			assert.Equal(t, r.Header["Content-Type"], []string{"application/json"})
			assert.Equal(t, r.Header["Authorization"], []string{"Bearer NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ"})

			assert.JSONEq(t, `{
				"toPersonEmail": "test@test.com",
				"markdown": "message"
			}`, string(b))
		}))
		defer ts.Close()

		service := NewWebexService(WebexOptions{
			Token:  "NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ",
			ApiURL: ts.URL,
		})
		err := service.Send(Notification{
			Message: "message",
		}, Destination{
			Service:   "webex",
			Recipient: "test@test.com",
		})

		if !assert.NoError(t, err) {
			t.FailNow()
		}
	})

	t.Run("successful attempt - room", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			if !assert.NoError(t, err) {
				t.FailNow()
			}

			assert.Equal(t, r.URL.Path, "/v1/messages")
			assert.Equal(t, r.Header["Content-Type"], []string{"application/json"})
			assert.Equal(t, r.Header["Authorization"], []string{"Bearer NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ"})

			assert.JSONEq(t, `{
				"roomId": "roomId123",
				"markdown": "message"
			}`, string(b))
		}))
		defer ts.Close()

		service := NewWebexService(WebexOptions{
			Token:  "NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ",
			ApiURL: ts.URL,
		})
		err := service.Send(Notification{
			Message: "message",
		}, Destination{
			Service:   "webex",
			Recipient: "roomId123",
		})

		if !assert.NoError(t, err) {
			t.FailNow()
		}
	})

	t.Run("auth error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(401)
			b, err := io.ReadAll(r.Body)
			if !assert.NoError(t, err) {
				t.FailNow()
			}

			assert.Equal(t, r.URL.Path, "/v1/messages")
			assert.Equal(t, r.Header["Content-Type"], []string{"application/json"})
			assert.Equal(t, r.Header["Authorization"], []string{"Bearer NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ"})

			assert.JSONEq(t, `{
				"toPersonEmail": "test@test.com",
				"markdown": "message"
			}`, string(b))
		}))
		defer ts.Close()

		service := NewWebexService(WebexOptions{
			Token:  "NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ",
			ApiURL: ts.URL,
		})
		err := service.Send(Notification{
			Message: "message",
		}, Destination{
			Service:   "webex",
			Recipient: "test@test.com",
		})

		if !assert.Error(t, err) {
			t.FailNow()
		}
	})

}
