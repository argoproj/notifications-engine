package controller

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	kubetesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/mocks"
	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/argoproj/notifications-engine/pkg/subscriptions"
	"github.com/argoproj/notifications-engine/pkg/triggers"
)

var (
	testGVR       = schema.GroupVersionResource{Group: "argoproj.io", Resource: "applications", Version: "v1alpha1"}
	testNamespace = "default"
	logEntry      = logrus.NewEntry(logrus.New())
)

func mustToJson(val interface{}) string {
	res, err := json.Marshal(val)
	if err != nil {
		panic(err)
	}
	return string(res)
}

func withAnnotations(annotations map[string]string) func(obj *unstructured.Unstructured) {
	return func(app *unstructured.Unstructured) {
		app.SetAnnotations(annotations)
	}
}

func newFakeClient(objects ...runtime.Object) *fake.FakeDynamicClient {
	return fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{testGVR: "List"}, objects...)
}

func newResource(name string, modifiers ...func(app *unstructured.Unstructured)) *unstructured.Unstructured {
	app := unstructured.Unstructured{}
	app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Kind: "application", Version: "v1alpha1"})
	app.SetName(name)
	app.SetNamespace(testNamespace)
	for i := range modifiers {
		modifiers[i](&app)
	}
	return &app
}

func newController(t *testing.T, ctx context.Context, client dynamic.Interface, opts ...Opts) (*notificationController, *mocks.MockAPI, error) {
	mockCtrl := gomock.NewController(t)
	go func() {
		<-ctx.Done()
		mockCtrl.Finish()
	}()
	mockAPI := mocks.NewMockAPI(mockCtrl)
	mockAPI.EXPECT().GetConfig().Return(api.Config{}).AnyTimes()
	resourceClient := client.Resource(testGVR)
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (object runtime.Object, err error) {
				return resourceClient.List(context.Background(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				return resourceClient.Watch(context.Background(), options)
			},
		},
		&unstructured.Unstructured{},
		time.Minute,
		cache.Indexers{},
	)

	go informer.Run(ctx.Done())

	c := NewController(resourceClient, informer, &mocks.FakeFactory{Api: mockAPI}, opts...)
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return nil, nil, errors.New("failed to sync informers")
	}

	return c, mockAPI, nil
}

func TestSendsNotificationIfTriggered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	app := newResource("test", withAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
	}))

	ctrl, api, err := newController(t, ctx, newFakeClient(app))
	assert.NoError(t, err)

	receivedObj := map[string]interface{}{}
	api.EXPECT().RunTrigger("my-trigger", gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil)
	api.EXPECT().Send(mock.MatchedBy(func(obj map[string]interface{}) bool {
		receivedObj = obj
		return true
	}), []string{"test"}, services.Destination{Service: "mock", Recipient: "recipient"}).Return(nil)

	_, err = ctrl.processResource(app, logEntry)

	assert.NoError(t, err)

	state := NewStateFromRes(app)
	assert.NotNil(t, state.NotifiedState[StateItemKey("mock", triggers.ConditionResult{}, services.Destination{Service: "mock", Recipient: "recipient"})])
	assert.Equal(t, app.Object, receivedObj)
}

func TestDoesNotSendNotificationIfAnnotationPresent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	state := emptyNotificationsState()
	_ = state.SetAlreadyNotified("my-trigger", triggers.ConditionResult{}, services.Destination{Service: "mock", Recipient: "recipient"}, true)
	app := newResource("test", withAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
		NotifiedAnnotationKey: mustToJson(state.NotifiedState),
		ServiceAnnotationKey:  mustToJson(state.ServiceState),
	}))
	ctrl, api, err := newController(t, ctx, newFakeClient(app))
	assert.NoError(t, err)

	api.EXPECT().RunTrigger("my-trigger", gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil)

	_, err = ctrl.processResource(app, logEntry)

	assert.NoError(t, err)
}

func TestDoesNotSendNotificationIfSkipFirstRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	app := newResource("test", withAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
	}))

	ctrl, api, err := newController(t, ctx, newFakeClient(app), WithSkipFirstRun(true))
	assert.NoError(t, err)

	receivedObj := map[string]interface{}{}
	api.EXPECT().RunTrigger("my-trigger", gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil).Times(2)
	api.EXPECT().Send(mock.MatchedBy(func(obj map[string]interface{}) bool {
		receivedObj = obj
		return true
	}), []string{"test"}, services.Destination{Service: "mock", Recipient: "recipient"}).Return(nil)

	annotations, err := ctrl.processResource(app, logEntry)
	assert.NoError(t, err)

	assert.NotContains(t, annotations, NotifiedAnnotationKey)
	assert.Contains(t, annotations, ServiceAnnotationKey)
	assert.Equal(t, map[string]interface{}{}, receivedObj)

	app.SetAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
		NotifiedAnnotationKey: annotations[NotifiedAnnotationKey],
		ServiceAnnotationKey:  annotations[ServiceAnnotationKey],
	})

	annotations2, err := ctrl.processResource(app, logEntry)
	assert.NoError(t, err)

	assert.Contains(t, annotations2, NotifiedAnnotationKey)
	assert.Equal(t, annotations[ServiceAnnotationKey], annotations2[ServiceAnnotationKey])
}

func TestRemovesAnnotationIfNoTrigger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	state := emptyNotificationsState()
	_ = state.SetAlreadyNotified("my-trigger", triggers.ConditionResult{}, services.Destination{Service: "mock", Recipient: "recipient"}, true)
	app := newResource("test", withAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
		NotifiedAnnotationKey: mustToJson(state.NotifiedState),
		ServiceAnnotationKey:  mustToJson(state.ServiceState),
	}))
	ctrl, api, err := newController(t, ctx, newFakeClient(app))
	assert.NoError(t, err)

	api.EXPECT().RunTrigger("my-trigger", gomock.Any()).Return([]triggers.ConditionResult{{Triggered: false}}, nil)

	annotations, err := ctrl.processResource(app, logEntry)
	app.SetAnnotations(map[string]string{
		NotifiedAnnotationKey: mustToJson(annotations[NotifiedAnnotationKey]),
		ServiceAnnotationKey:  mustToJson(annotations[ServiceAnnotationKey]),
	})

	assert.NoError(t, err)
	state = NewStateFromRes(app)
	assert.Equal(t, emptyNotificationsState(), state)
}

func TestUpdatedAnnotationsSavedAsPatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	state := emptyNotificationsState()
	_ = state.SetAlreadyNotified("my-trigger", triggers.ConditionResult{}, services.Destination{Service: "mock", Recipient: "recipient"}, true)

	app := newResource("test", withAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
		NotifiedAnnotationKey: mustToJson(state.NotifiedState),
	}))

	patchCh := make(chan []byte)

	client := newFakeClient(app)
	client.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patchCh <- action.(kubetesting.PatchAction).GetPatch()
		return true, nil, nil
	})
	ctrl, api, err := newController(t, ctx, client)
	assert.NoError(t, err)
	api.EXPECT().RunTrigger("my-trigger", gomock.Any()).Return([]triggers.ConditionResult{{Triggered: false}}, nil)

	go ctrl.Run(1, ctx.Done())

	select {
	case <-time.After(time.Second * 5):
		t.Error("application was not patched")
	case patchData := <-patchCh:
		patch := map[string]interface{}{}
		err = json.Unmarshal(patchData, &patch)
		assert.NoError(t, err)
		val, ok, err := unstructured.NestedFieldNoCopy(patch, "metadata", "annotations", NotifiedAnnotationKey)
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Nil(t, val)
	}
}

func TestAnnotationIsTheSame(t *testing.T) {
	t.Run("same", func(t *testing.T) {
		app1 := newResource("test", withAnnotations(map[string]string{
			subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
		}))
		app2 := newResource("test", withAnnotations(map[string]string{
			subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
		}))
		assert.True(t, mapsEqual(app1.GetAnnotations(), app2.GetAnnotations()))
	})

	t.Run("same-nil-nil", func(t *testing.T) {
		app1 := newResource("test", withAnnotations(nil))
		app2 := newResource("test", withAnnotations(nil))
		assert.True(t, mapsEqual(app1.GetAnnotations(), app2.GetAnnotations()))
	})

	t.Run("same-nil-emptyMap", func(t *testing.T) {
		app1 := newResource("test", withAnnotations(nil))
		app2 := newResource("test", withAnnotations(map[string]string{}))
		assert.True(t, mapsEqual(app1.GetAnnotations(), app2.GetAnnotations()))
	})

	t.Run("same-emptyMap-nil", func(t *testing.T) {
		app1 := newResource("test", withAnnotations(map[string]string{}))
		app2 := newResource("test", withAnnotations(nil))
		assert.True(t, mapsEqual(app1.GetAnnotations(), app2.GetAnnotations()))
	})

	t.Run("same-emptyMap-emptyMap", func(t *testing.T) {
		app1 := newResource("test", withAnnotations(map[string]string{}))
		app2 := newResource("test", withAnnotations(map[string]string{}))
		assert.True(t, mapsEqual(app1.GetAnnotations(), app2.GetAnnotations()))
	})

	t.Run("notSame-nil-map", func(t *testing.T) {
		app1 := newResource("test", withAnnotations(nil))
		app2 := newResource("test", withAnnotations(map[string]string{
			subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
		}))
		assert.False(t, mapsEqual(app1.GetAnnotations(), app2.GetAnnotations()))
	})

	t.Run("notSame-map-nil", func(t *testing.T) {
		app1 := newResource("test", withAnnotations(map[string]string{
			subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
		}))
		app2 := newResource("test", withAnnotations(nil))
		assert.False(t, mapsEqual(app1.GetAnnotations(), app2.GetAnnotations()))
	})

	t.Run("notSame-map-map", func(t *testing.T) {
		app1 := newResource("test", withAnnotations(map[string]string{
			subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
		}))
		app2 := newResource("test", withAnnotations(map[string]string{
			subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient2",
		}))
		assert.False(t, mapsEqual(app1.GetAnnotations(), app2.GetAnnotations()))
	})
}
