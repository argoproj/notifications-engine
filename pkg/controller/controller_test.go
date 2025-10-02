package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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
	apiMap["selfservice_namespace"].(*mocks.MockAPI).EXPECT().GetConfig().Return(notificationApi.Config{IsSelfServiceConfig: true, Namespace: "selfservice_namespace"}).AnyTimes()
	apiMap["selfservice_namespace"].(*mocks.MockAPI).EXPECT().RunTrigger(triggerName, gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil)
	apiMap["selfservice_namespace"].(*mocks.MockAPI).EXPECT().Send(mock.MatchedBy(func(obj map[string]interface{}) bool {
		return true
	}), []string{"test"}, destination).Return(nil).AnyTimes()

	apiMap["default"].(*mocks.MockAPI).EXPECT().GetConfig().Return(notificationApi.Config{IsSelfServiceConfig: false, Namespace: "default"}).AnyTimes()
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

func TestNotificationsShouldNotBeBlockedBySlowDestinations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	// Setup: 3 destinations - 1 slow (500ms) and 2 fast (50ms)
	// We verify that fast notifications complete without waiting for the slow one
	app := newResource("test", withAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "webhook-slow"):  "slow-recipient",
		subscriptions.SubscribeAnnotationKey("my-trigger", "webhook-fast1"): "fast-recipient-1",
		subscriptions.SubscribeAnnotationKey("my-trigger", "webhook-fast2"): "fast-recipient-2",
	}))

	ctrl, api, err := newController(t, ctx, newFakeClient(app))
	assert.NoError(t, err)

	api.EXPECT().GetConfig().Return(notificationApi.Config{}).AnyTimes()
	api.EXPECT().RunTrigger("my-trigger", gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil)

	// Track when each notification starts to verify parallel execution
	sendTimes := make([]time.Time, 0)
	var timesLock sync.Mutex

	const slowWebhookDelay = 500 * time.Millisecond
	const fastWebhookDelay = 50 * time.Millisecond
	const parallelStartThreshold = 50 * time.Millisecond

	// Mock slow webhook that times out after 500ms
	api.EXPECT().Send(gomock.Any(), []string{"test"}, services.Destination{Service: "webhook-slow", Recipient: "slow-recipient"}).
		DoAndReturn(func(_ map[string]interface{}, _ []string, _ services.Destination) error {
			timesLock.Lock()
			sendTimes = append(sendTimes, time.Now())
			timesLock.Unlock()
			time.Sleep(slowWebhookDelay)
			return fmt.Errorf("webhook timeout")
		})

	// Mock fast webhooks that complete quickly
	api.EXPECT().Send(gomock.Any(), []string{"test"}, services.Destination{Service: "webhook-fast1", Recipient: "fast-recipient-1"}).
		DoAndReturn(func(_ map[string]interface{}, _ []string, _ services.Destination) error {
			timesLock.Lock()
			sendTimes = append(sendTimes, time.Now())
			timesLock.Unlock()
			time.Sleep(fastWebhookDelay)
			return nil
		})

	api.EXPECT().Send(gomock.Any(), []string{"test"}, services.Destination{Service: "webhook-fast2", Recipient: "fast-recipient-2"}).
		DoAndReturn(func(_ map[string]interface{}, _ []string, _ services.Destination) error {
			timesLock.Lock()
			sendTimes = append(sendTimes, time.Now())
			timesLock.Unlock()
			time.Sleep(fastWebhookDelay)
			return nil
		})

	// Execute and measure total time
	eventSequence := &NotificationEventSequence{}
	start := time.Now()
	_, err = ctrl.processResourceWithAPI(api, app, logEntry, eventSequence)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, 3, len(sendTimes), "All 3 notifications should have been sent")

	// Verify all notifications started in parallel (within threshold)
	if len(sendTimes) >= 2 {
		timeBetweenFirstAndSecond := sendTimes[1].Sub(sendTimes[0])
		assert.Less(t, timeBetweenFirstAndSecond.Milliseconds(), parallelStartThreshold.Milliseconds(),
			"Fast notifications should start in parallel, not wait for slow ones")
	}

	if len(sendTimes) >= 3 {
		timeBetweenFirstAndThird := sendTimes[2].Sub(sendTimes[0])
		assert.Less(t, timeBetweenFirstAndThird.Milliseconds(), parallelStartThreshold.Milliseconds(),
			"All notifications should start in parallel")
	}

	// Total time should be ~500ms (longest webhook), not 600ms (sum of all)
	assert.Less(t, elapsed.Seconds(), 0.7,
		"Total time should be ~0.5s (parallel), not sum of all notifications")

	// Verify error was recorded for slow webhook
	assert.Greater(t, len(eventSequence.Errors), 0, "Slow webhook error should be recorded")

	// Verify 2 fast webhooks succeeded
	successfulDeliveries := 0
	for _, delivery := range eventSequence.Delivered {
		if !delivery.AlreadyNotified {
			successfulDeliveries++
		}
	}
	assert.Equal(t, 2, successfulDeliveries, "Two fast webhooks should have succeeded")
}

func TestConcurrentNotificationsLimited(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	// Setup: 10 destinations with a concurrency limit of 3
	// Verify that at most 3 notifications are sent concurrently
	const numNotifications = 10
	const maxConcurrency = 3

	annotations := make(map[string]string)
	for i := 1; i <= numNotifications; i++ {
		annotations[subscriptions.SubscribeAnnotationKey("my-trigger", fmt.Sprintf("webhook-%d", i))] = fmt.Sprintf("recipient-%d", i)
	}
	app := newResource("test", withAnnotations(annotations))

	ctrl, api, err := newController(t, ctx, newFakeClient(app), WithMaxConcurrentNotifications(maxConcurrency))
	assert.NoError(t, err)

	api.EXPECT().GetConfig().Return(notificationApi.Config{}).AnyTimes()
	api.EXPECT().RunTrigger("my-trigger", gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil)

	// Track concurrent execution to verify worker pool limits
	var concurrentCount int32
	var maxConcurrent int32
	var countLock sync.Mutex

	for i := 1; i <= numNotifications; i++ {
		api.EXPECT().Send(gomock.Any(), []string{"test"}, services.Destination{
			Service:   fmt.Sprintf("webhook-%d", i),
			Recipient: fmt.Sprintf("recipient-%d", i),
		}).DoAndReturn(func(_ map[string]interface{}, _ []string, _ services.Destination) error {
			// Increment concurrent count and track maximum
			countLock.Lock()
			concurrentCount++
			if concurrentCount > maxConcurrent {
				maxConcurrent = concurrentCount
			}
			currentCount := concurrentCount
			countLock.Unlock()

			// Verify we never exceed the limit
			assert.LessOrEqual(t, currentCount, int32(maxConcurrency),
				"Concurrent notifications should not exceed configured limit")

			// Simulate work
			time.Sleep(50 * time.Millisecond)

			// Decrement concurrent count
			countLock.Lock()
			concurrentCount--
			countLock.Unlock()

			return nil
		})
	}

	eventSequence := &NotificationEventSequence{}
	_, err = ctrl.processResourceWithAPI(api, app, logEntry, eventSequence)
	assert.NoError(t, err)

	// Verify the worker pool reached the configured limit
	assert.Equal(t, int32(maxConcurrency), maxConcurrent,
		"Worker pool should reach the maximum concurrency limit")
	assert.Equal(t, numNotifications, len(eventSequence.Delivered),
		"All notifications should be delivered")
}

func TestSendNotificationsInParallel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	// Setup: 3 destinations to verify they execute in parallel
	// We use concurrent call counting instead of timing to avoid flaky tests
	const numNotifications = 3

	app := newResource("test", withAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "webhook-1"): "recipient-1",
		subscriptions.SubscribeAnnotationKey("my-trigger", "webhook-2"): "recipient-2",
		subscriptions.SubscribeAnnotationKey("my-trigger", "webhook-3"): "recipient-3",
	}))

	ctrl, api, err := newController(t, ctx, newFakeClient(app))
	assert.NoError(t, err)

	api.EXPECT().GetConfig().Return(notificationApi.Config{}).AnyTimes()
	api.EXPECT().RunTrigger("my-trigger", gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil)

	// Track active concurrent calls and maximum reached
	var activeCalls int32
	var maxConcurrent int32
	var countLock sync.Mutex

	// Channel signals when all notifications have started
	allStarted := make(chan struct{})

	for i := 1; i <= numNotifications; i++ {
		api.EXPECT().Send(gomock.Any(), []string{"test"}, services.Destination{
			Service:   fmt.Sprintf("webhook-%d", i),
			Recipient: fmt.Sprintf("recipient-%d", i),
		}).DoAndReturn(func(_ map[string]interface{}, _ []string, _ services.Destination) error {
			// Track concurrent execution
			countLock.Lock()
			activeCalls++
			if activeCalls > maxConcurrent {
				maxConcurrent = activeCalls
			}
			currentActive := activeCalls

			// Signal when all notifications are active concurrently
			if currentActive == numNotifications {
				close(allStarted)
			}
			countLock.Unlock()

			// Simulate work
			time.Sleep(100 * time.Millisecond)

			countLock.Lock()
			activeCalls--
			countLock.Unlock()

			return nil
		})
	}

	// Execute in background to verify parallel start
	eventSequence := &NotificationEventSequence{}
	done := make(chan struct{})
	go func() {
		_, err = ctrl.processResourceWithAPI(api, app, logEntry, eventSequence)
		close(done)
	}()

	// Wait for all notifications to start concurrently
	select {
	case <-allStarted:
		// Success - all notifications started in parallel
	case <-time.After(time.Second):
		t.Fatal("notifications did not start in parallel within timeout")
	}

	// Wait for completion
	<-done
	assert.NoError(t, err)

	// Verify all notifications ran concurrently
	assert.Equal(t, int32(numNotifications), maxConcurrent,
		"All notifications should have been active concurrently")
	assert.Equal(t, numNotifications, len(eventSequence.Delivered),
		"All notifications should be delivered")
}

func TestSendSingleNotification(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	// Setup common test data
	app := newResource("test", withAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "webhook"): "recipient",
	}))

	ctrl, api, err := newController(t, ctx, newFakeClient(app))
	assert.NoError(t, err)

	un, err := ctrl.toUnstructured(app)
	assert.NoError(t, err)

	destination := services.Destination{Service: "webhook", Recipient: "recipient"}
	templates := []string{"template1"}
	trigger := "my-trigger"
	cr := triggers.ConditionResult{Key: "test-condition"}
	apiNamespace := "default"

	t.Run("success case", func(t *testing.T) {
		// Mock successful send
		api.EXPECT().Send(un.Object, templates, destination).Return(nil)

		// Execute
		result := ctrl.sendSingleNotification(api, un, app, trigger, cr, destination, templates, apiNamespace, logEntry)

		// Verify success result
		assert.True(t, result.success, "Notification should succeed")
		assert.Nil(t, result.err, "No error should be returned")
		assert.Equal(t, trigger, result.delivery.Trigger, "Trigger name should match")
		assert.Equal(t, destination, result.delivery.Destination, "Destination should match")
		assert.False(t, result.delivery.AlreadyNotified, "Should not be marked as already notified")
	})

	t.Run("error case", func(t *testing.T) {
		// Mock failed send
		sendErr := fmt.Errorf("network timeout")
		api.EXPECT().Send(un.Object, templates, destination).Return(sendErr)

		// Execute
		result := ctrl.sendSingleNotification(api, un, app, trigger, cr, destination, templates, apiNamespace, logEntry)

		// Verify error result
		assert.False(t, result.success, "Notification should fail")
		assert.NotNil(t, result.err, "Error should be returned")
		assert.Contains(t, result.err.Error(), "network timeout", "Error should contain original error message")
		assert.Contains(t, result.err.Error(), "failed to deliver notification", "Error should be wrapped with context")
	})
}

func TestDefaultConcurrencyLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	// Verify default concurrency limit when not explicitly configured
	ctrl, _, err := newController(t, ctx, newFakeClient())
	assert.NoError(t, err)

	assert.Equal(t, notificationApi.DefaultMaxConcurrentNotifications, ctrl.maxConcurrentNotifications,
		"Default concurrency limit should match constant")
}

func TestCustomConcurrencyLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	// Verify custom concurrency limit is applied
	ctrl, _, err := newController(t, ctx, newFakeClient(), WithMaxConcurrentNotifications(25))
	assert.NoError(t, err)

	assert.Equal(t, 25, ctrl.maxConcurrentNotifications,
		"Custom concurrency limit should be applied")
}

func TestInvalidConcurrencyLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	// Verify negative values fall back to default
	ctrl, _, err := newController(t, ctx, newFakeClient(), WithMaxConcurrentNotifications(-5))
	assert.NoError(t, err)

	assert.Equal(t, notificationApi.DefaultMaxConcurrentNotifications, ctrl.maxConcurrentNotifications,
		"Negative concurrency limit should fall back to default")

	// Verify zero values fall back to default
	ctrl2, _, err := newController(t, ctx, newFakeClient(), WithMaxConcurrentNotifications(0))
	assert.NoError(t, err)

	assert.Equal(t, notificationApi.DefaultMaxConcurrentNotifications, ctrl2.maxConcurrentNotifications,
		"Zero concurrency limit should fall back to default")
}

func TestConcurrencyLimitFromConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	// Setup: app with 5 destinations
	// Controller has default limit of 50, but Config sets it to 2
	const numNotifications = 5
	const configMaxConcurrency = 2

	annotations := make(map[string]string)
	for i := 1; i <= numNotifications; i++ {
		annotations[subscriptions.SubscribeAnnotationKey("my-trigger", fmt.Sprintf("webhook-%d", i))] = fmt.Sprintf("recipient-%d", i)
	}
	app := newResource("test", withAnnotations(annotations))

	ctrl, api, err := newController(t, ctx, newFakeClient(app))
	assert.NoError(t, err)

	// Config specifies MaxConcurrentNotifications
	api.EXPECT().GetConfig().Return(notificationApi.Config{
		MaxConcurrentNotifications: configMaxConcurrency,
	}).AnyTimes()
	api.EXPECT().RunTrigger("my-trigger", gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil)

	// Track concurrent execution to verify config value is used
	var concurrentCount int32
	var maxConcurrent int32
	var countLock sync.Mutex

	for i := 1; i <= numNotifications; i++ {
		api.EXPECT().Send(gomock.Any(), []string{"test"}, services.Destination{
			Service:   fmt.Sprintf("webhook-%d", i),
			Recipient: fmt.Sprintf("recipient-%d", i),
		}).DoAndReturn(func(_ map[string]interface{}, _ []string, _ services.Destination) error {
			// Increment and track concurrent count
			countLock.Lock()
			concurrentCount++
			if concurrentCount > maxConcurrent {
				maxConcurrent = concurrentCount
			}
			currentCount := concurrentCount
			countLock.Unlock()

			// Verify we never exceed the config limit
			assert.LessOrEqual(t, currentCount, int32(configMaxConcurrency),
				"Concurrent notifications should not exceed config limit")

			time.Sleep(50 * time.Millisecond)

			countLock.Lock()
			concurrentCount--
			countLock.Unlock()

			return nil
		})
	}

	eventSequence := &NotificationEventSequence{}
	_, err = ctrl.processResourceWithAPI(api, app, logEntry, eventSequence)
	assert.NoError(t, err)

	// Verify the config value was used (not the controller default of 50)
	assert.Equal(t, int32(configMaxConcurrency), maxConcurrent,
		"Should use config's MaxConcurrentNotifications value")
	assert.Equal(t, numNotifications, len(eventSequence.Delivered),
		"All notifications should be delivered")
}

func TestSequentialNotificationsWhenConcurrencyIsOne(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	// --- Arrange ------------------------------------------------------------------
	const (
		triggerName      = "my-trigger"
		numNotifications = 5
		servicePrefix    = "webhook"
		recipientPrefix  = "recipient"
	)

	annotations := make(map[string]string, numNotifications)
	for i := 1; i <= numNotifications; i++ {
		annotations[subscriptions.SubscribeAnnotationKey(triggerName, fmt.Sprintf("%s-%d", servicePrefix, i))] = fmt.Sprintf("%s-%d", recipientPrefix, i)
	}
	app := newResource("test", withAnnotations(annotations))

	ctrl, api, err := newController(t, ctx, newFakeClient(app), WithMaxConcurrentNotifications(1))
	assert.NoError(t, err)

	api.EXPECT().GetConfig().Return(notificationApi.Config{}).AnyTimes()
	api.EXPECT().RunTrigger(triggerName, gomock.Any()).Return([]triggers.ConditionResult{{Triggered: true, Templates: []string{"test"}}}, nil)

	// Concurrency tracking state
	var active int32         // current in-flight sends
	var peakActive int32     // maximum observed simultaneous sends
	var executionOrder []int // order in which destinations began sending (for permutation validation)
	var orderLock sync.Mutex

	for i := 1; i <= numNotifications; i++ {
		idx := i
		api.EXPECT().Send(gomock.Any(), []string{"test"}, services.Destination{
			Service:   fmt.Sprintf("%s-%d", servicePrefix, i),
			Recipient: fmt.Sprintf("%s-%d", recipientPrefix, i),
		}).DoAndReturn(func(_ map[string]interface{}, _ []string, _ services.Destination) error {
			// Track entry
			current := atomic.AddInt32(&active, 1)
			if current > peakActive {
				peakActive = current
			}
			orderLock.Lock()
			executionOrder = append(executionOrder, idx)
			orderLock.Unlock()

			// Simulate delivery time (small to keep test fast)
			time.Sleep(20 * time.Millisecond)

			// Track exit
			atomic.AddInt32(&active, -1)
			return nil
		})
	}

	// --- Act ----------------------------------------------------------------------
	eventSeq := &NotificationEventSequence{}
	_, err = ctrl.processResourceWithAPI(api, app, logEntry, eventSeq)
	assert.NoError(t, err)

	// --- Assert -------------------------------------------------------------------
	assert.Equal(t, int32(1), peakActive, "Concurrency=1 must enforce strictly sequential sends")
	assert.Equal(t, numNotifications, len(eventSeq.Delivered), "All notifications should be delivered")
	assert.Empty(t, eventSeq.Errors, "No delivery errors expected")

	// Validate executionOrder is a permutation of 1..numNotifications
	if len(executionOrder) != numNotifications {
		t.Fatalf("expected %d executions recorded, got %d", numNotifications, len(executionOrder))
	}
	seen := make(map[int]bool, numNotifications)
	for _, v := range executionOrder {
		seen[v] = true
	}
	for i := 1; i <= numNotifications; i++ {
		if !seen[i] {
			t.Fatalf("missing expected index %d in execution order (concurrency=1)", i)
		}
	}
}
