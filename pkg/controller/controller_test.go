package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	kubetesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	notificationApi "github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/mocks"
	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/argoproj/notifications-engine/pkg/subscriptions"
	"github.com/argoproj/notifications-engine/pkg/triggers"
)

var (
	testGVR               = schema.GroupVersionResource{Group: "argoproj.io", Resource: "applications", Version: "v1alpha1"}
	testNamespace         = "default"
	logEntry              = logrus.NewEntry(logrus.New())
	notifiedAnnotationKey = subscriptions.NotifiedAnnotationKey()
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
	t.Helper()
	mockCtrl := gomock.NewController(t)
	go func() {
		<-ctx.Done()
		mockCtrl.Finish()
	}()
	mockAPI := mocks.NewMockAPI(mockCtrl)
	resourceClient := client.Resource(testGVR)
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (object runtime.Object, err error) {
				return resourceClient.List(context.Background(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return resourceClient.Watch(context.Background(), options)
			},
		},
		&unstructured.Unstructured{},
		time.Minute,
		cache.Indexers{},
	)

	go informer.Run(ctx.Done())

	c := NewControllerWithNamespaceSupport(resourceClient, informer, &mocks.FakeFactory{Api: mockAPI}, opts...)
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return nil, nil, errors.New("failed to sync informers")
	}

	return c, mockAPI, nil
}

func newControllerWithNamespaceSupport(t *testing.T, ctx context.Context, client dynamic.Interface, opts ...Opts) (*notificationController, map[string]notificationApi.API, error) {
	t.Helper()
	mockCtrl := gomock.NewController(t)
	go func() {
		<-ctx.Done()
		mockCtrl.Finish()
	}()

	resourceClient := client.Resource(testGVR)
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (object runtime.Object, err error) {
				return resourceClient.List(context.Background(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return resourceClient.Watch(context.Background(), options)
			},
		},
		&unstructured.Unstructured{},
		time.Minute,
		cache.Indexers{},
	)

	go informer.Run(ctx.Done())

	apiMap := make(map[string]notificationApi.API)
	mockAPIDefault := mocks.NewMockAPI(mockCtrl)
	apiMap["default"] = mockAPIDefault

	mockAPISelfService := mocks.NewMockAPI(mockCtrl)
	apiMap["selfservice_namespace"] = mockAPISelfService

	c := NewControllerWithNamespaceSupport(resourceClient, informer, &mocks.FakeFactory{ApiMap: apiMap}, opts...)
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return nil, nil, errors.New("failed to sync informers")
	}

	return c, apiMap, nil
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
	api.EXPECT().GetConfig().Return(notificationApi.Config{}).AnyTimes()
	api.EXPECT().RunTrigger("my-trigger", gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil)
	api.EXPECT().Send(mock.MatchedBy(func(obj map[string]interface{}) bool {
		receivedObj = obj
		return true
	}), []string{"test"}, services.Destination{Service: "mock", Recipient: "recipient"}).Return(nil)

	annotations, err := ctrl.processResourceWithAPI(api, app, logEntry, &NotificationEventSequence{})
	if err != nil {
		logEntry.Errorf("Failed to process: %v", err)
	}

	assert.NoError(t, err)

	state := NewState(annotations[notifiedAnnotationKey])
	assert.NotNil(t, state[StateItemKey(false, "", "mock", triggers.ConditionResult{}, services.Destination{Service: "mock", Recipient: "recipient"})])
	assert.Equal(t, app.Object, receivedObj)
}

func TestDoesNotSendNotificationIfAnnotationPresent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	state := NotificationsState{}
	_ = state.SetAlreadyNotified(false, "", "my-trigger", triggers.ConditionResult{}, services.Destination{Service: "mock", Recipient: "recipient"}, true)
	app := newResource("test", withAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
		notifiedAnnotationKey: mustToJson(state),
	}))
	ctrl, api, err := newController(t, ctx, newFakeClient(app))
	assert.NoError(t, err)

	api.EXPECT().GetConfig().Return(notificationApi.Config{}).AnyTimes()
	api.EXPECT().RunTrigger("my-trigger", gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil)

	_, err = ctrl.processResourceWithAPI(api, app, logEntry, &NotificationEventSequence{})
	if err != nil {
		logEntry.Errorf("Failed to process: %v", err)
	}
	assert.NoError(t, err)
}

func TestRemovesAnnotationIfNoTrigger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	state := NotificationsState{}
	_ = state.SetAlreadyNotified(false, "", "my-trigger", triggers.ConditionResult{}, services.Destination{Service: "mock", Recipient: "recipient"}, true)
	app := newResource("test", withAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
		notifiedAnnotationKey: mustToJson(state),
	}))
	ctrl, api, err := newController(t, ctx, newFakeClient(app))
	assert.NoError(t, err)

	api.EXPECT().GetConfig().Return(notificationApi.Config{}).AnyTimes()
	api.EXPECT().RunTrigger("my-trigger", gomock.Any()).Return([]triggers.ConditionResult{{Triggered: false}}, nil)

	annotations, err := ctrl.processResourceWithAPI(api, app, logEntry, &NotificationEventSequence{})
	if err != nil {
		logEntry.Errorf("Failed to process: %v", err)
	}
	assert.NoError(t, err)
	state = NewState(annotations[notifiedAnnotationKey])
	assert.Empty(t, state)
}

func TestUpdatedAnnotationsSavedAsPatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	state := NotificationsState{}
	_ = state.SetAlreadyNotified(false, "", "my-trigger", triggers.ConditionResult{}, services.Destination{Service: "mock", Recipient: "recipient"}, true)

	app := newResource("test", withAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
		notifiedAnnotationKey: mustToJson(state),
	}))

	patchCh := make(chan []byte)

	client := newFakeClient(app)
	client.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patchCh <- action.(kubetesting.PatchAction).GetPatch()
		return true, nil, nil
	})
	ctrl, api, err := newController(t, ctx, client)
	assert.NoError(t, err)
	api.EXPECT().GetConfig().Return(notificationApi.Config{}).AnyTimes()
	api.EXPECT().RunTrigger("my-trigger", gomock.Any()).Return([]triggers.ConditionResult{{Triggered: false}}, nil)

	go ctrl.Run(1, ctx.Done())

	select {
	case <-time.After(time.Second * 5):
		t.Error("application was not patched")
	case patchData := <-patchCh:
		patch := map[string]interface{}{}
		err = json.Unmarshal(patchData, &patch)
		assert.NoError(t, err)
		val, ok, err := unstructured.NestedFieldNoCopy(patch, "metadata", "annotations", notifiedAnnotationKey)
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

func TestWithEventCallback(t *testing.T) {
	const triggerName = "my-trigger"
	destination := services.Destination{Service: "mock", Recipient: "recipient"}
	testCases := []struct {
		description        string
		apiErr             error
		sendErr            error
		expectedDeliveries []NotificationDelivery
		expectedErrors     []error
		expectedWarnings   []error
	}{
		{
			description: "EventCallback should be invoked with nil error on send success",
			sendErr:     nil,
			expectedDeliveries: []NotificationDelivery{
				{
					Trigger:     triggerName,
					Destination: destination,
				},
			},
		},
		{
			description: "EventCallback should be invoked with non-nil error on send failure",
			sendErr:     errors.New("this is a send error"),
			expectedErrors: []error{
				fmt.Errorf("failed to deliver notification my-trigger to {mock recipient}: %w using the configuration in namespace ", errors.New("this is a send error")),
			},
		},
		{
			description: "EventCallback should be invoked with non-nil error on api failure",
			apiErr:      errors.New("this is an api error"),
			expectedErrors: []error{
				fmt.Errorf("this is an api error"),
			},
		},
	}
	var actualSequence *NotificationEventSequence
	mockEventCallback := func(eventSequence NotificationEventSequence) {
		actualSequence = &eventSequence
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actualSequence = nil
			ctx, cancel := context.WithCancel(context.TODO())
			defer cancel()
			app := newResource("test", withAnnotations(map[string]string{
				subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
			}))

			ctrl, api, err := newController(t, ctx, newFakeClient(app), WithEventCallback(mockEventCallback))
			ctrl.namespaceSupport = false
			api.EXPECT().GetConfig().Return(notificationApi.Config{}).AnyTimes()
			assert.NoError(t, err)
			ctrl.apiFactory = &mocks.FakeFactory{Api: api, Err: tc.apiErr}

			if tc.apiErr == nil {
				api.EXPECT().RunTrigger(triggerName, gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil)
				api.EXPECT().Send(mock.MatchedBy(func(obj map[string]interface{}) bool {
					return true
				}), []string{"test"}, destination).Return(tc.sendErr)
			}

			ctrl.processQueueItem()

			assert.Equal(t, app, actualSequence.Resource)

			assert.Equal(t, len(tc.expectedDeliveries), len(actualSequence.Delivered))
			for i, event := range actualSequence.Delivered {
				assert.Equal(t, tc.expectedDeliveries[i].Trigger, event.Trigger)
				assert.Equal(t, tc.expectedDeliveries[i].Destination, event.Destination)
			}

			assert.Equal(t, tc.expectedErrors, actualSequence.Errors)
			assert.Equal(t, tc.expectedWarnings, actualSequence.Warnings)
		})
	}
}

// verify annotations after calling processResourceWithAPI when using self-service
func TestProcessResourceWithAPIWithSelfService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	app := newResource("test", withAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
	}))

	ctrl, api, err := newController(t, ctx, newFakeClient(app))
	assert.NoError(t, err)
	ctrl.namespaceSupport = true

	trigger := "my-trigger"
	namespace := "my-namespace"

	receivedObj := map[string]interface{}{}

	// SelfService API: config has IsSelfServiceConfig set to true
	api.EXPECT().GetConfig().Return(notificationApi.Config{IsSelfServiceConfig: true, Namespace: namespace}).AnyTimes()
	api.EXPECT().RunTrigger(trigger, gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil)
	api.EXPECT().Send(mock.MatchedBy(func(obj map[string]interface{}) bool {
		receivedObj = obj
		return true
	}), []string{"test"}, services.Destination{Service: "mock", Recipient: "recipient"}).Return(nil)

	annotations, err := ctrl.processResourceWithAPI(api, app, logEntry, &NotificationEventSequence{})
	if err != nil {
		logEntry.Errorf("Failed to process: %v", err)
	}

	assert.NoError(t, err)

	state := NewState(annotations[notifiedAnnotationKey])
	assert.NotZero(t, state[StateItemKey(true, namespace, trigger, triggers.ConditionResult{}, services.Destination{Service: "mock", Recipient: "recipient"})])
	assert.Equal(t, app.Object, receivedObj)
}

// verify notification sent to both default and self-service configuration after calling processResourceWithAPI when using self-service
func TestProcessItemsWithSelfService(t *testing.T) {
	const triggerName = "my-trigger"
	destination := services.Destination{Service: "mock", Recipient: "recipient"}

	var actualSequence *NotificationEventSequence
	mockEventCallback := func(eventSequence NotificationEventSequence) {
		actualSequence = &eventSequence
	}

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	app := newResource("test", withAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
	}))

	ctrl, apiMap, err := newControllerWithNamespaceSupport(t, ctx, newFakeClient(app), WithEventCallback(mockEventCallback))
	assert.NoError(t, err)

	ctrl.namespaceSupport = true
	// SelfService API: config has IsSelfServiceConfig set to true
	apiMap["selfservice_namespace"].(*mocks.MockAPI).EXPECT().GetConfig().Return(notificationApi.Config{IsSelfServiceConfig: true, Namespace: "selfservice_namespace"}).Times(3)
	apiMap["selfservice_namespace"].(*mocks.MockAPI).EXPECT().RunTrigger(triggerName, gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil)
	apiMap["selfservice_namespace"].(*mocks.MockAPI).EXPECT().Send(mock.MatchedBy(func(obj map[string]interface{}) bool {
		return true
	}), []string{"test"}, destination).Return(nil).AnyTimes()

	apiMap["default"].(*mocks.MockAPI).EXPECT().GetConfig().Return(notificationApi.Config{IsSelfServiceConfig: false, Namespace: "default"}).Times(3)
	apiMap["default"].(*mocks.MockAPI).EXPECT().RunTrigger(triggerName, gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil)
	apiMap["default"].(*mocks.MockAPI).EXPECT().Send(mock.MatchedBy(func(obj map[string]interface{}) bool {
		return true
	}), []string{"test"}, destination).Return(nil).AnyTimes()

	ctrl.apiFactory = &mocks.FakeFactory{ApiMap: apiMap}

	ctrl.processQueueItem()

	assert.Equal(t, app, actualSequence.Resource)

	expectedDeliveries := []NotificationDelivery{
		{
			Trigger:     triggerName,
			Destination: destination,
		},
		{
			Trigger:     triggerName,
			Destination: destination,
		},
	}
	for i, event := range actualSequence.Delivered {
		assert.Equal(t, expectedDeliveries[i].Trigger, event.Trigger)
		assert.Equal(t, expectedDeliveries[i].Destination, event.Destination)
	}
}
