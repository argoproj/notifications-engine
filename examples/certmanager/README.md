# Cert Manager Notifications

The example demonstrates how Notifications Engine can be used to implement notifications for [Cert-Manager](https://cert-manager.io/) Certificate CRD.

## Controller

The machinery required for controller implementation is provided by [pkg/controller](../../pkg/controller) and [pkg/api](../../pkg/api) packages.

* The first step is to write the boilerplate code required to get Kubernetes REST config so we can talk to API server.

* Next create `ConfigMap` and `Secret` informers and use it to initialize `api.Factory`:

```golang
informersFactory := informers.NewSharedInformerFactoryWithOptions(
	kubernetes.NewForConfigOrDie(restConfig),
	time.Minute,
	informers.WithNamespace(namespace))
secrets := informersFactory.Core().V1().Secrets().Informer()
configMaps := informersFactory.Core().V1().ConfigMaps().Informer()
notificationsFactory := api.NewFactory(api.Settings{
	ConfigMapName: "cert-manager-notifications-cm",
	SecretName:    "cert-manager-notifications-secret",
	InitGetVars: func(cfg *api.Config, configMap *v1.ConfigMap, secret *v1.Secret) (api.GetVars, error) {
		return func(obj map[string]interface{}, dest services.Destination) map[string]interface{} {
			return map[string]interface{}{"cert": obj}
		}, nil
	},
}, namespace, secrets, configMaps)
```

* Next step is to create the `NotificationController`:

```golang
certClient := dynamic.NewForConfigOrDie(restConfig).Resource(schema.GroupVersionResource{
	Group: "cert-manager.io", Version: "v1", Resource: "certificates",
})
certsInformer := cache.NewSharedIndexInformer(&cache.ListWatch{
	ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
		return certClient.List(context.Background(), options)
	},
	WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
		return certClient.Watch(context.Background(), metav1.ListOptions{})
	},
}, &unstructured.Unstructured{}, time.Minute, cache.Indexers{})
ctrl := controller.NewController(certClient, certsInformer, notificationsFactory)
```

* Finally "start" informers and run the controller:


```golang
go informersFactory.Start(context.Background().Done())
go certsInformer.Run(context.Background().Done())
if !cache.WaitForCacheSync(context.Background().Done(), secrets.HasSynced, configMaps.HasSynced, certsInformer.HasSynced) {
	log.Fatalf("Failed to synchronize informers")
}
ctrl.Run(10, context.Background().Done())
```

Done! Your controller is ready. The full code listing is available in [controller/main.go](controller/main.go). You can
clone this repository and use `go run examples/certmanager/controller/main.go` to start it.

## Configuration

The next step is to create a trigger, notification template, and configure integration with the notification service.
The example below demonstrates `on-cert-ready` trigger which sends a notification using `cert-ready` template when
the `Certificate` is ready:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cert-manager-notifications-cm
data:
  trigger.on-cert-ready: |
    - when: any(cert.status.conditions, {.reason == 'Ready' && .status == 'True'})
      send: [cert-ready]

  template.cert-ready: |
    message: |
      Certificate {{.cert.metadata.name}} is ready!

  service.slack: |
    token: $slack-token
```

Apply the sample configuration using the following command:

```
kubectl apply -f ./examples/certmanager/config.yaml
```

The example configures integration with Slack. Following the steps described in [slack](../../docs/services/slack.md) to get the Slack
token and create `cert-manager-notification-secret` Secret using the following command:

```
kubectl create secret generic cert-manager-notification-secret --from-literal slack-token=<SLACK-TOKEN>
```

Finally annotate the certificate to subscribe to the `on-cert-ready` trigger and get the notification:

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned-issuer
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: notification-test
  annotations:
    notifications.argoproj.io/subscribe.on-cert-ready.slack: <CHANNEL>
spec:
  dnsNames:
    - notification.test
  duration: 438000h0m0s
  issuerRef:
    kind: Issuer
    name: selfsigned-issuer
  renewBefore: 4320h0m0s
  secretName: notification-test
EOF
```

## Debugging Tools

The CRD specific triggers and templates create true value for end-users. However, it might be challenging for the CRD controller administrator
to create more of them. The simplify administrators life the Notification Engine includes [pkg/cmd](../../pkg/cmd) package that provides debugging CLI tool
implementation. CLI tool provides commands to run triggers/templates configured in the live Kubernetes cluster or in the local YAML file. Use the following
snippet to create CLI for Cert Manager configuration debugging:

```golang
func main() {
	command := cmd.NewToolsCommand("cli", "cli", schema.GroupVersionResource{
		Group: "cert-manager.io", Version: "v1", Resource: "certificates",
	}, api.Settings{
		ConfigMapName: "cert-manager-notifications-cm",
		SecretName:    "cert-manager-notifications-secret",
		InitGetVars: func(cfg *api.Config, configMap *v1.ConfigMap, secret *v1.Secret) (api.GetVars, error) {
			return func(obj map[string]interface{}, dest services.Destination) map[string]interface{} {
				return map[string]interface{}{"cert": obj}
			}, nil
		},
	})

	if err := command.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
```

The full source code is available in [cli/main.go](./cli/main.go). Use the following command:

* to list avilable templates:

```
go run examples/certmanager/cli/main.go template get --config-map ./examples/certmanager/config.yaml --secret :empty
```

* to "run" `on-cert-ready` trigger:

```
go run examples/certmanager/cli/main.go trigger run on-cert-ready <MY-CERT> --config-map ./examples/certmanager/config.yaml --secret :empty
```

* to see what else is available:


```
go run examples/certmanager/cli/main.go --help
```
