package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mattermostTestPost struct {
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
	RootID    string `json:"root_id"`
}

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
			Attachments: "{{.foo}}",
			GroupingKey: "{{.app}}",
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
}

func TestSend_MattermostThreading(t *testing.T) {
	var mutex sync.Mutex
	posts := make([]mattermostTestPost, 0)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var post mattermostTestPost
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mutex.Lock()
		posts = append(posts, post)
		postNumber := len(posts)
		mutex.Unlock()
		_, _ = fmt.Fprintf(w, `{"id":"post-%d"}`, postNumber)
	}))
	defer ts.Close()

	state := newMattermostThreadState()
	newService := func() NotificationService {
		return newMattermostService(MattermostOptions{ApiURL: ts.URL, Token: "token"}, state)
	}
	destination := Destination{Service: "mattermost", Recipient: "channel"}
	grouped := func(message string) Notification {
		return Notification{Message: message, Mattermost: &MattermostNotification{GroupingKey: "app-uid"}}
	}

	require.NoError(t, newService().Send(grouped("first error"), destination))
	// Recreating a service in the same process must preserve the root post.
	require.NoError(t, newService().Send(grouped("second error"), destination))
	// Notifications without groupingKey remain independent posts.
	require.NoError(t, newService().Send(Notification{Message: "success"}, destination))

	mutex.Lock()
	defer mutex.Unlock()
	require.Len(t, posts, 3)
	assert.Empty(t, posts[0].RootID)
	assert.Equal(t, "post-1", posts[1].RootID)
	assert.Empty(t, posts[2].RootID)
}

func TestSend_MattermostThreadStateRestart(t *testing.T) {
	var mutex sync.Mutex
	posts := make([]mattermostTestPost, 0)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var post mattermostTestPost
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mutex.Lock()
		posts = append(posts, post)
		postNumber := len(posts)
		mutex.Unlock()
		_, _ = fmt.Fprintf(w, `{"id":"post-%d"}`, postNumber)
	}))
	defer ts.Close()

	opts := MattermostOptions{ApiURL: ts.URL, Token: "token"}
	destination := Destination{Service: "mattermost", Recipient: "channel"}
	notification := Notification{Mattermost: &MattermostNotification{GroupingKey: "app-uid"}}
	require.NoError(t, newMattermostService(opts, newMattermostThreadState()).Send(notification, destination))
	require.NoError(t, newMattermostService(opts, newMattermostThreadState()).Send(notification, destination))

	mutex.Lock()
	defer mutex.Unlock()
	require.Len(t, posts, 2)
	assert.Empty(t, posts[0].RootID)
	assert.Empty(t, posts[1].RootID)
}

func TestSend_MattermostConcurrentThreadCreation(t *testing.T) {
	const sends = 32
	var mutex sync.Mutex
	posts := make([]mattermostTestPost, 0, sends)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var post mattermostTestPost
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mutex.Lock()
		posts = append(posts, post)
		postNumber := len(posts)
		mutex.Unlock()
		_, _ = fmt.Fprintf(w, `{"id":"post-%d"}`, postNumber)
	}))
	defer ts.Close()

	service := newMattermostService(MattermostOptions{ApiURL: ts.URL, Token: "token"}, newMattermostThreadState())
	destination := Destination{Service: "mattermost", Recipient: "channel"}
	start := make(chan struct{})
	errs := make(chan error, sends)
	var wg sync.WaitGroup
	for i := 0; i < sends; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs <- service.Send(Notification{
				Message:    fmt.Sprintf("error-%d", i),
				Mattermost: &MattermostNotification{GroupingKey: "app-uid"},
			}, destination)
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	mutex.Lock()
	defer mutex.Unlock()
	require.Len(t, posts, sends)
	rootPosts := 0
	for _, post := range posts {
		if post.RootID == "" {
			rootPosts++
		} else {
			assert.Equal(t, "post-1", post.RootID)
		}
	}
	assert.Equal(t, 1, rootPosts)
}

func TestSend_MattermostDoesNotSerializeUnrelatedThreads(t *testing.T) {
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseFirst) }) }
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var post mattermostTestPost
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if post.Message == "first" {
			close(firstStarted)
			<-releaseFirst
		}
		_, _ = io.WriteString(w, `{"id":"root"}`)
	}))
	defer ts.Close()
	defer release()

	service := newMattermostService(MattermostOptions{ApiURL: ts.URL}, newMattermostThreadState())
	destination := Destination{Recipient: "channel"}
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- service.Send(Notification{
			Message:    "first",
			Mattermost: &MattermostNotification{GroupingKey: "app-1"},
		}, destination)
	}()
	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first grouped send did not reach the server")
	}

	secondDone := make(chan error, 1)
	go func() {
		secondDone <- service.Send(Notification{
			Message:    "second",
			Mattermost: &MattermostNotification{GroupingKey: "app-2"},
		}, destination)
	}()
	select {
	case err := <-secondDone:
		release()
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		release()
		t.Fatal("send for an unrelated grouping key was blocked")
	}
	require.NoError(t, <-firstDone)
}

func TestSend_MattermostThreadIsolation(t *testing.T) {
	type recorder struct {
		mutex sync.Mutex
		posts []mattermostTestPost
	}
	newServer := func(recorder *recorder) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var post mattermostTestPost
			if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			recorder.mutex.Lock()
			recorder.posts = append(recorder.posts, post)
			postNumber := len(recorder.posts)
			recorder.mutex.Unlock()
			_, _ = fmt.Fprintf(w, `{"id":"post-%d"}`, postNumber)
		}))
	}

	firstRecorder := &recorder{}
	firstServer := newServer(firstRecorder)
	defer firstServer.Close()
	secondRecorder := &recorder{}
	secondServer := newServer(secondRecorder)
	defer secondServer.Close()
	state := newMattermostThreadState()
	firstService := newMattermostService(MattermostOptions{ApiURL: firstServer.URL}, state)
	secondService := newMattermostService(MattermostOptions{ApiURL: secondServer.URL}, state)
	notify := func(service NotificationService, channel, key string) {
		require.NoError(t, service.Send(
			Notification{Mattermost: &MattermostNotification{GroupingKey: key}},
			Destination{Service: "mattermost", Recipient: channel},
		))
	}

	notify(firstService, "channel-1", "app-1")
	notify(firstService, "channel-1", "app-1")
	notify(firstService, "channel-1", "app-2")
	notify(firstService, "channel-2", "app-1")
	notify(secondService, "channel-1", "app-1")

	firstRecorder.mutex.Lock()
	defer firstRecorder.mutex.Unlock()
	require.Len(t, firstRecorder.posts, 4)
	assert.Empty(t, firstRecorder.posts[0].RootID)
	assert.Equal(t, "post-1", firstRecorder.posts[1].RootID)
	assert.Empty(t, firstRecorder.posts[2].RootID)
	assert.Empty(t, firstRecorder.posts[3].RootID)
	secondRecorder.mutex.Lock()
	defer secondRecorder.mutex.Unlock()
	require.Len(t, secondRecorder.posts, 1)
	assert.Empty(t, secondRecorder.posts[0].RootID)
}

func TestSend_MattermostRootFailuresAreNotCached(t *testing.T) {
	var mutex sync.Mutex
	posts := make([]mattermostTestPost, 0, 3)
	requestNumber := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var post mattermostTestPost
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mutex.Lock()
		posts = append(posts, post)
		requestNumber++
		currentRequest := requestNumber
		mutex.Unlock()
		if currentRequest == 1 {
			http.Error(w, "temporary failure", http.StatusInternalServerError)
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":"post-%d"}`, currentRequest)
	}))
	defer ts.Close()

	service := newMattermostService(MattermostOptions{ApiURL: ts.URL}, newMattermostThreadState())
	destination := Destination{Recipient: "channel"}
	notification := Notification{Mattermost: &MattermostNotification{GroupingKey: "app-uid"}}
	require.Error(t, service.Send(notification, destination))
	require.NoError(t, service.Send(notification, destination))
	require.NoError(t, service.Send(notification, destination))

	mutex.Lock()
	defer mutex.Unlock()
	require.Len(t, posts, 3)
	assert.Empty(t, posts[0].RootID)
	assert.Empty(t, posts[1].RootID)
	assert.Equal(t, "post-2", posts[2].RootID)
}

func TestSend_MattermostInvalidRootResponsesAreNotCached(t *testing.T) {
	var mutex sync.Mutex
	posts := make([]mattermostTestPost, 0, 3)
	requestNumber := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var post mattermostTestPost
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mutex.Lock()
		posts = append(posts, post)
		requestNumber++
		currentRequest := requestNumber
		mutex.Unlock()
		switch currentRequest {
		case 1:
			_, _ = io.WriteString(w, `{}`)
		case 2:
			_, _ = io.WriteString(w, `not-json`)
		default:
			_, _ = io.WriteString(w, `{"id":"post-3"}`)
		}
	}))
	defer ts.Close()

	service := newMattermostService(MattermostOptions{ApiURL: ts.URL}, newMattermostThreadState())
	destination := Destination{Recipient: "channel"}
	notification := Notification{Mattermost: &MattermostNotification{GroupingKey: "app-uid"}}
	require.EqualError(t, service.Send(notification, destination), "mattermost post response does not contain an id")
	require.ErrorContains(t, service.Send(notification, destination), "failed to unmarshal Mattermost post response")
	require.NoError(t, service.Send(notification, destination))

	mutex.Lock()
	defer mutex.Unlock()
	require.Len(t, posts, 3)
	for _, post := range posts {
		assert.Empty(t, post.RootID)
	}
}

func TestSend_MattermostReplyFailurePreservesRoot(t *testing.T) {
	var mutex sync.Mutex
	posts := make([]mattermostTestPost, 0, 3)
	requestNumber := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var post mattermostTestPost
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mutex.Lock()
		posts = append(posts, post)
		requestNumber++
		currentRequest := requestNumber
		mutex.Unlock()
		if currentRequest == 2 {
			http.Error(w, "temporary failure", http.StatusInternalServerError)
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":"post-%d"}`, currentRequest)
	}))
	defer ts.Close()

	service := newMattermostService(MattermostOptions{ApiURL: ts.URL}, newMattermostThreadState())
	destination := Destination{Recipient: "channel"}
	notification := Notification{Mattermost: &MattermostNotification{GroupingKey: "app-uid"}}
	require.NoError(t, service.Send(notification, destination))
	require.Error(t, service.Send(notification, destination))
	require.NoError(t, service.Send(notification, destination))

	mutex.Lock()
	defer mutex.Unlock()
	require.Len(t, posts, 3)
	assert.Empty(t, posts[0].RootID)
	assert.Equal(t, "post-1", posts[1].RootID)
	assert.Equal(t, "post-1", posts[2].RootID)
}
