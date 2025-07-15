package services

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidEmail(t *testing.T) {
	assert.True(t, validEmail.MatchString("test@test.com"))
	assert.True(t, validEmail.MatchString("test.test@test.com"))
	assert.False(t, validEmail.MatchString("notAnEmail"))
	assert.False(t, validEmail.MatchString("notAnEmail@"))
}

func TestSend_Webex(t *testing.T) {
	t.Run("successful attempt - email", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			assert.Equal(t, "/v1/messages", r.URL.Path)
			assert.Equal(t, []string{"application/json"}, r.Header["Content-Type"])
			assert.Equal(t, []string{"Bearer NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ"}, r.Header["Authorization"])

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

		require.NoError(t, err)
	})

	t.Run("successful attempt - room", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			assert.Equal(t, "/v1/messages", r.URL.Path)
			assert.Equal(t, []string{"application/json"}, r.Header["Content-Type"])
			assert.Equal(t, []string{"Bearer NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ"}, r.Header["Authorization"])

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

		require.NoError(t, err)
	})

	t.Run("auth error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			assert.Equal(t, "/v1/messages", r.URL.Path)
			assert.Equal(t, []string{"application/json"}, r.Header["Content-Type"])
			assert.Equal(t, []string{"Bearer NRAK-5F2FIVA5UTA4FFDD11XCXVA7WPJ"}, r.Header["Authorization"])

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

		require.Error(t, err)
	})
}
