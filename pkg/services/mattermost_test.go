package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSend_Mattermost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		assert.JSONEq(t, `{
			"channel_id": "channel",
			"message": "message",
			"props": {
				"attachments": [{
					"title": "title",
					"title_link": "https://argocd.example.com/applications/argocd-notifications",
					"color": "#18be52",
					"fields": [{
						"title": "Sync Status",
						"value": "Synced",
						"short": true
					}, {
						"title": "Repository",
						"value": "https://example.com",
						"short": true
					}]
				}]
			}
		}`, string(b))
	}))
	defer ts.Close()

	service := NewMattermostService(MattermostOptions{
		ApiURL:             ts.URL,
		Token:              "token",
		InsecureSkipVerify: true,
	})
	err := service.Send(Notification{
		Message: "message",
		Mattermost: &MattermostNotification{
			Attachments: `[{
				"title": "title",
				"title_link": "https://argocd.example.com/applications/argocd-notifications",
				"color": "#18be52",
				"fields": [{
					"title": "Sync Status",
					"value": "Synced",
					"short": true
				}, {
					"title": "Repository",
					"value": "https://example.com",
					"short": true
				}]
			}]`,
		},
	}, Destination{
		Service:   "mattermost",
		Recipient: "channel",
	})
	require.NoError(t, err)
}

func TestGetTemplater_Mattermost(t *testing.T) {
	n := Notification{
		Mattermost: &MattermostNotification{
			Attachments:    "{{.foo}}",
			GroupingKey:    "{{.app}}",
			DeliveryPolicy: MattermostUpdate,
		},
	}
	templater, err := n.GetTemplater("", template.FuncMap{})

	require.NoError(t, err)

	var notification Notification
	err = templater(&notification, map[string]any{
		"foo": "hello",
		"app": "my-app",
	})

	require.NoError(t, err)

	assert.Equal(t, "hello", notification.Mattermost.Attachments)
	assert.Equal(t, "my-app", notification.Mattermost.GroupingKey)
	assert.Equal(t, MattermostUpdate, notification.Mattermost.DeliveryPolicy)
}

func TestMattermostDeliveryPolicyUnmarshal(t *testing.T) {
	for _, policy := range []MattermostDeliveryPolicy{MattermostPost, MattermostPostAndUpdate, MattermostUpdate} {
		t.Run(string(policy), func(t *testing.T) {
			var notification MattermostNotification
			require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(`{"deliveryPolicy":%q}`, policy)), &notification))
			assert.Equal(t, policy, notification.DeliveryPolicy)
		})
	}
	var notification MattermostNotification
	require.EqualError(t, json.Unmarshal([]byte(`{"deliveryPolicy":"Invalid"}`), &notification), `unsupported Mattermost delivery policy "Invalid"`)
}

func TestSend_MattermostWithoutGroupingKeyAlwaysCreatesIndependentMessages(t *testing.T) {
	var posts atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		posts.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	service := newMattermostService(MattermostOptions{ApiURL: ts.URL}, newMattermostUpdateState())
	destination := Destination{Recipient: "channel"}
	for _, policy := range []MattermostDeliveryPolicy{"", MattermostPost, MattermostPostAndUpdate, MattermostUpdate} {
		notification := Notification{Message: "message", Mattermost: &MattermostNotification{DeliveryPolicy: policy}}
		require.NoError(t, service.Send(notification, destination))
	}
	assert.Equal(t, int32(4), posts.Load())
}

func TestSend_MattermostDeliveryPoliciesWithGroupingKey(t *testing.T) {
	tests := map[string][]string{
		string(MattermostPost):          {"POST root", "POST reply"},
		string(MattermostUpdate):        {"POST root", "GET root", "PUT root"},
		string(MattermostPostAndUpdate): {"POST root", "POST reply", "GET root", "PUT root"},
	}
	for policy, expected := range tests {
		t.Run(policy, func(t *testing.T) {
			var mutex sync.Mutex
			var operations []string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodPost:
					var body struct {
						RootID string `json:"root_id"`
					}
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					operation := "POST root"
					if body.RootID != "" {
						assert.Equal(t, "root", body.RootID)
						operation = "POST reply"
					}
					mutex.Lock()
					operations = append(operations, operation)
					mutex.Unlock()
					_, _ = io.WriteString(w, `{"id":"root"}`)
				case http.MethodGet:
					mutex.Lock()
					operations = append(operations, "GET root")
					mutex.Unlock()
					_, _ = io.WriteString(w, `{"props":{}}`)
				case http.MethodPut:
					mutex.Lock()
					operations = append(operations, "PUT root")
					mutex.Unlock()
				}
			}))
			defer ts.Close()

			service := newMattermostService(MattermostOptions{ApiURL: ts.URL}, newMattermostUpdateState())
			notification := Notification{Mattermost: &MattermostNotification{GroupingKey: "operation", DeliveryPolicy: MattermostDeliveryPolicy(policy)}}
			require.NoError(t, service.Send(notification, Destination{Recipient: "channel"}))
			require.NoError(t, service.Send(notification, Destination{Recipient: "channel"}))
			assert.Equal(t, expected, operations)
		})
	}
}

func TestSend_MattermostUpdateAndServiceReconstruction(t *testing.T) {
	var mutex sync.Mutex
	methods := make([]string, 0, 5)
	type patchPayload struct {
		Message string                     `json:"message"`
		Props   map[string]json.RawMessage `json:"props"`
	}
	patches := make([]patchPayload, 0, 2)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		methods = append(methods, r.Method)
		mutex.Unlock()
		switch r.Method {
		case http.MethodPost:
			_, _ = io.WriteString(w, `{"id":"post-1"}`)
		case http.MethodGet:
			assert.Equal(t, "/api/v4/posts/post-1", r.URL.Path)
			_, _ = io.WriteString(w, `{"props":{"custom":"preserved","large":9007199254740993,"attachments":[{"title":"old"}]}}`)
		case http.MethodPut:
			assert.Equal(t, "/api/v4/posts/post-1/patch", r.URL.Path)
			var patch patchPayload
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mutex.Lock()
			patches = append(patches, patch)
			mutex.Unlock()
		}
	}))
	defer ts.Close()

	state := newMattermostUpdateState()
	options := MattermostOptions{ApiURL: ts.URL, Token: "token"}
	destination := Destination{Recipient: "channel"}
	first := Notification{Message: "running", Mattermost: &MattermostNotification{GroupingKey: "operation", DeliveryPolicy: MattermostUpdate}}
	second := Notification{Message: "complete", Mattermost: &MattermostNotification{GroupingKey: "operation", DeliveryPolicy: MattermostUpdate, Attachments: `[{"title":"new"}]`}}
	third := Notification{Message: "cleared", Mattermost: &MattermostNotification{GroupingKey: "operation", DeliveryPolicy: MattermostUpdate}}
	require.NoError(t, newMattermostService(options, state).Send(first, destination))
	require.NoError(t, newMattermostService(options, state).Send(second, destination))
	require.NoError(t, newMattermostService(options, state).Send(third, destination))

	mutex.Lock()
	defer mutex.Unlock()
	assert.Equal(t, []string{"POST", "GET", "PUT", "GET", "PUT"}, methods)
	require.Len(t, patches, 2)
	assert.Equal(t, "complete", patches[0].Message)
	assert.JSONEq(t, `"preserved"`, string(patches[0].Props["custom"]))
	assert.Equal(t, `9007199254740993`, string(patches[0].Props["large"]))
	assert.JSONEq(t, `[{"title":"new"}]`, string(patches[0].Props["attachments"]))
	assert.Equal(t, "cleared", patches[1].Message)
	assert.JSONEq(t, `[]`, string(patches[1].Props["attachments"]))
}

func TestSend_MattermostUpdateWithoutGroupingKeyIsIndependent(t *testing.T) {
	var posts atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		posts.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	service := newMattermostService(MattermostOptions{ApiURL: ts.URL}, newMattermostUpdateState())
	notification := Notification{Mattermost: &MattermostNotification{DeliveryPolicy: MattermostUpdate}}
	require.NoError(t, service.Send(notification, Destination{Recipient: "channel"}))
	require.NoError(t, service.Send(notification, Destination{Recipient: "channel"}))
	assert.Equal(t, int32(2), posts.Load())
}

func TestSend_MattermostConcurrentGroupedDeliveryCreatesOneRoot(t *testing.T) {
	const sends = 16
	for _, policy := range []MattermostDeliveryPolicy{MattermostPost, MattermostPostAndUpdate, MattermostUpdate} {
		t.Run(string(policy), func(t *testing.T) {
			var roots atomic.Int32
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodPost:
					var body struct {
						RootID string `json:"root_id"`
					}
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					if body.RootID == "" {
						roots.Add(1)
					}
					_, _ = io.WriteString(w, `{"id":"root"}`)
				case http.MethodGet:
					_, _ = io.WriteString(w, `{"props":{}}`)
				case http.MethodPut:
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer ts.Close()
			service := newMattermostService(MattermostOptions{ApiURL: ts.URL}, newMattermostUpdateState())
			destination := Destination{Recipient: "channel"}
			start := make(chan struct{})
			errs := make(chan error, sends)
			var wait sync.WaitGroup
			for i := 0; i < sends; i++ {
				wait.Add(1)
				go func(i int) {
					defer wait.Done()
					<-start
					errs <- service.Send(Notification{Message: fmt.Sprintf("message-%d", i), Mattermost: &MattermostNotification{GroupingKey: "operation", DeliveryPolicy: policy}}, destination)
				}(i)
			}
			close(start)
			wait.Wait()
			close(errs)
			for err := range errs {
				require.NoError(t, err)
			}
			assert.Equal(t, int32(1), roots.Load())
		})
	}
}

func TestSend_MattermostUpdateStateIsolationAndRestart(t *testing.T) {
	var posts atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			post := posts.Add(1)
			_, _ = fmt.Fprintf(w, `{"id":"post-%d"}`, post)
		case http.MethodGet:
			_, _ = io.WriteString(w, `{"props":{}}`)
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()
	var otherPosts atomic.Int32
	otherTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			otherPosts.Add(1)
			_, _ = io.WriteString(w, `{"id":"other-post"}`)
		case http.MethodGet:
			_, _ = io.WriteString(w, `{"props":{}}`)
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer otherTS.Close()
	state := newMattermostUpdateState()
	service := newMattermostService(MattermostOptions{ApiURL: ts.URL}, state)
	notify := func(s NotificationService, channel, key string) {
		require.NoError(t, s.Send(Notification{Mattermost: &MattermostNotification{GroupingKey: key, DeliveryPolicy: MattermostUpdate}}, Destination{Recipient: channel}))
	}
	notify(service, "channel-1", "key-1")
	notify(service, "channel-1", "key-1")
	notify(service, "channel-1", "key-2")
	notify(service, "channel-2", "key-1")
	otherService := newMattermostService(MattermostOptions{ApiURL: otherTS.URL}, state)
	notify(otherService, "channel-1", "key-1")
	notify(otherService, "channel-1", "key-1")
	notify(newMattermostService(MattermostOptions{ApiURL: ts.URL}, newMattermostUpdateState()), "channel-1", "key-1")
	assert.Equal(t, int32(4), posts.Load())
	assert.Equal(t, int32(1), otherPosts.Load())
}

func TestSend_MattermostUpdateFailuresDoNotReplacePost(t *testing.T) {
	var posts atomic.Int32
	var getCalls atomic.Int32
	var putCalls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			call := posts.Add(1)
			if call == 1 {
				http.Error(w, "temporary", http.StatusBadGateway)
				return
			}
			_, _ = io.WriteString(w, `{"id":"root"}`)
		case http.MethodGet:
			if getCalls.Add(1) == 1 {
				http.Error(w, "temporary", http.StatusBadGateway)
				return
			}
			_, _ = io.WriteString(w, `{"props":{}}`)
		case http.MethodPut:
			if putCalls.Add(1) == 1 {
				http.Error(w, "temporary", http.StatusBadGateway)
				return
			}
		}
	}))
	defer ts.Close()
	service := newMattermostService(MattermostOptions{ApiURL: ts.URL}, newMattermostUpdateState())
	destination := Destination{Recipient: "channel"}
	notification := Notification{Mattermost: &MattermostNotification{GroupingKey: "operation", DeliveryPolicy: MattermostUpdate}}
	require.Error(t, service.Send(notification, destination))
	require.NoError(t, service.Send(notification, destination))
	require.Error(t, service.Send(notification, destination))
	require.Error(t, service.Send(notification, destination))
	require.NoError(t, service.Send(notification, destination))
	assert.Equal(t, int32(2), posts.Load())
}

func TestSend_MattermostPostAndUpdateStopsOnPartialFailure(t *testing.T) {
	var posts atomic.Int32
	var gets atomic.Int32
	var puts atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			call := posts.Add(1)
			if call == 1 {
				_, _ = io.WriteString(w, `{"id":"root"}`)
				return
			}
			if call == 2 {
				http.Error(w, "reply failed", http.StatusBadGateway)
				return
			}
			_, _ = io.WriteString(w, `{"id":"reply"}`)
		case http.MethodGet:
			gets.Add(1)
			_, _ = io.WriteString(w, `{"props":{}}`)
		case http.MethodPut:
			if puts.Add(1) == 1 {
				http.Error(w, "update failed", http.StatusBadGateway)
				return
			}
		}
	}))
	defer ts.Close()

	service := newMattermostService(MattermostOptions{ApiURL: ts.URL}, newMattermostUpdateState())
	destination := Destination{Recipient: "channel"}
	notification := Notification{Mattermost: &MattermostNotification{GroupingKey: "operation", DeliveryPolicy: MattermostPostAndUpdate}}
	require.NoError(t, service.Send(notification, destination))
	require.ErrorContains(t, service.Send(notification, destination), "reply failed")
	assert.Zero(t, gets.Load(), "a failed reply must prevent the root update")
	require.ErrorContains(t, service.Send(notification, destination), "update failed")
	require.NoError(t, service.Send(notification, destination))
	assert.Equal(t, int32(4), posts.Load(), "the retry posts another reply after a successful reply and failed update")
	assert.Equal(t, int32(2), gets.Load())
	assert.Equal(t, int32(2), puts.Load())
}

func TestSend_MattermostInvalidCreateResponsesAreNotCached(t *testing.T) {
	var posts atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			switch posts.Add(1) {
			case 1:
				_, _ = io.WriteString(w, `{}`)
			case 2:
				_, _ = io.WriteString(w, `not-json`)
			default:
				_, _ = io.WriteString(w, `{"id":"root"}`)
			}
		case http.MethodGet:
			_, _ = io.WriteString(w, `{"props":{}}`)
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()
	service := newMattermostService(MattermostOptions{ApiURL: ts.URL}, newMattermostUpdateState())
	destination := Destination{Recipient: "channel"}
	notification := Notification{Mattermost: &MattermostNotification{GroupingKey: "operation", DeliveryPolicy: MattermostUpdate}}
	require.EqualError(t, service.Send(notification, destination), "mattermost post response does not contain an id")
	require.ErrorContains(t, service.Send(notification, destination), "failed to unmarshal Mattermost post response")
	require.NoError(t, service.Send(notification, destination))
	require.NoError(t, service.Send(notification, destination))
	assert.Equal(t, int32(3), posts.Load())
}

func TestSend_MattermostRejectsUnknownDeliveryPolicy(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { calls.Add(1) }))
	defer ts.Close()
	service := newMattermostService(MattermostOptions{ApiURL: ts.URL}, newMattermostUpdateState())
	err := service.Send(Notification{Mattermost: &MattermostNotification{DeliveryPolicy: MattermostDeliveryPolicy("Invalid")}}, Destination{Recipient: "channel"})
	require.EqualError(t, err, `unsupported Mattermost delivery policy "Invalid"`)
	assert.Equal(t, int32(0), calls.Load())
}
