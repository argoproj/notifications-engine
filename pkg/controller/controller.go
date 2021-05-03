package controller

import (
	"context"
	"encoding/json"
	"reflect"
	"runtime/debug"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/argoproj/notifications-engine/pkg/subscriptions"
)

type NotificationController interface {
	Run(ctx context.Context, processors int)
}

type Opts func(ctrl *notificationController)

func WithMetricsRegistry(r *MetricsRegistry) Opts {
	return func(ctrl *notificationController) {
		ctrl.metricsRegistry = r
	}
}

func WithAdditionalDestinations(f func(obj *unstructured.Unstructured, cfg api.Config) services.Destinations) Opts {
	return func(ctrl *notificationController) {
		ctrl.additionalDestinations = f
	}
}

func WithSkipProcessing(f func(obj *unstructured.Unstructured) (bool, string)) Opts {
	return func(ctrl *notificationController) {
		ctrl.skipProcessing = f
	}
}

func NewController(
	client dynamic.ResourceInterface,
	informer cache.SharedIndexInformer,
	apiFactory api.Factory,
	opts ...Opts,
) *notificationController {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					queue.Add(key)
				}
			},
			UpdateFunc: func(old, new interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(new)
				if err == nil {
					queue.Add(key)
				}
			},
		},
	)

	ctrl := &notificationController{client: client, informer: informer, queue: queue, metricsRegistry: NewMetricsRegistry(""), apiFactory: apiFactory}
	for i := range opts {
		opts[i](ctrl)
	}
	return ctrl
}

type notificationController struct {
	client                 dynamic.ResourceInterface
	informer               cache.SharedIndexInformer
	queue                  workqueue.RateLimitingInterface
	apiFactory             api.Factory
	metricsRegistry        *MetricsRegistry
	additionalDestinations func(obj *unstructured.Unstructured, cfg api.Config) services.Destinations
	skipProcessing         func(obj *unstructured.Unstructured) (bool, string)
}

func (c *notificationController) Run(ctx context.Context, processors int) {
	defer runtimeutil.HandleCrash()
	defer c.queue.ShutDown()

	log.Warn("Controller is running.")
	for i := 0; i < processors; i++ {
		go wait.Until(func() {
			for c.processQueueItem() {
			}
		}, time.Second, ctx.Done())
	}
	<-ctx.Done()
	log.Warn("Controller has stopped.")
}

func (c *notificationController) processResource(resource *unstructured.Unstructured, logEntry *log.Entry) error {
	notificationsState := NewStateFromRes(resource)
	api, err := c.apiFactory.GetAPI()
	if err != nil {
		return err
	}

	for trigger, destinations := range c.getDestinations(resource, api.GetConfig()) {
		res, err := api.RunTrigger(trigger, resource.Object)
		if err != nil {
			logEntry.Debugf("Failed to execute condition of trigger %s: %v", trigger, err)
		}
		logEntry.Infof("Trigger %s result: %v", trigger, res)

		for _, cr := range res {
			c.metricsRegistry.IncTriggerEvaluationsCounter(trigger, cr.Triggered)

			if !cr.Triggered {
				for _, to := range destinations {
					notificationsState.SetAlreadyNotified(trigger, cr, to, false)
				}
				continue
			}

			for _, to := range destinations {
				if changed := notificationsState.SetAlreadyNotified(trigger, cr, to, true); !changed {
					logEntry.Infof("Notification about condition '%s.%s' already sent to '%v'", trigger, cr.Key, to)
				} else {
					logEntry.Infof("Sending notification about condition '%s.%s' to '%v'", trigger, cr.Key, to)
					if err := api.Send(resource.Object, cr.Templates, to); err != nil {
						logEntry.Errorf("Failed to notify recipient %s defined in resource %s/%s: %v",
							to, resource.GetNamespace(), resource.GetName(), err)
						notificationsState.SetAlreadyNotified(trigger, cr, to, false)
						c.metricsRegistry.IncDeliveriesCounter(trigger, to.Service, false)
					} else {
						logEntry.Debugf("Notification %s was sent", to.Recipient)
						c.metricsRegistry.IncDeliveriesCounter(trigger, to.Service, true)
					}
				}
			}
		}
	}

	return notificationsState.Persist(resource)
}

func (c *notificationController) getDestinations(resource *unstructured.Unstructured, cfg api.Config) services.Destinations {
	res := cfg.GetGlobalDestinations(resource.GetLabels())
	res.Merge(subscriptions.Annotations(resource.GetAnnotations()).GetDestinations(cfg.DefaultTriggers, cfg.ServiceDefaultTriggers))
	if c.additionalDestinations != nil {
		res.Merge(c.additionalDestinations(resource, cfg))
	}
	return res.Dedup()
}

func (c *notificationController) processQueueItem() (processNext bool) {
	key, shutdown := c.queue.Get()
	if shutdown {
		processNext = false
		return
	}
	processNext = true
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
		}
		c.queue.Done(key)
	}()

	obj, exists, err := c.informer.GetIndexer().GetByKey(key.(string))
	if err != nil {
		log.Errorf("Failed to get resource '%s' from informer index: %+v", key, err)
		return
	}
	if !exists {
		// This happens after resource was deleted, but the work queue still had an entry for it.
		return
	}
	resource, ok := obj.(*unstructured.Unstructured)
	if !ok {
		log.Errorf("Failed to get resource '%s' from informer index: %+v", key, err)
		return
	}
	resourceCopy := resource.DeepCopy()
	logEntry := log.WithField("resource", key)
	logEntry.Info("Start processing")
	if c.skipProcessing != nil {
		if skipProcessing, reason := c.skipProcessing(resourceCopy); skipProcessing {
			logEntry.Infof("Processing skipped: %s", reason)
			return
		}
	}

	err = c.processResource(resourceCopy, logEntry)
	if err != nil {
		logEntry.Errorf("Failed to process: %v", err)
		return
	}

	if !mapsEqual(resource.GetAnnotations(), resourceCopy.GetAnnotations()) {
		annotationsPatch := make(map[string]interface{})
		for k, v := range resourceCopy.GetAnnotations() {
			annotationsPatch[k] = v
		}
		for k := range resource.GetAnnotations() {
			if _, ok = resourceCopy.GetAnnotations()[k]; !ok {
				annotationsPatch[k] = nil
			}
		}

		patchData, err := json.Marshal(map[string]map[string]interface{}{
			"metadata": {"annotations": annotationsPatch},
		})
		if err != nil {
			logEntry.Errorf("Failed to marshal resource patch: %v", err)
			return
		}
		resource, err = c.client.Patch(context.Background(), resource.GetName(), types.MergePatchType, patchData, v1.PatchOptions{})
		if err != nil {
			logEntry.Errorf("Failed to patch resource: %v", err)
			return
		}
		if err := c.informer.GetStore().Update(resource); err != nil {
			logEntry.Warnf("Failed to store update resource in informer: %v", err)
		}
	}
	logEntry.Info("Processing completed")

	return
}

func mapsEqual(first, second map[string]string) bool {
	if first == nil {
		first = map[string]string{}
	}

	if second == nil {
		second = map[string]string{}
	}

	return reflect.DeepEqual(first, second)
}
