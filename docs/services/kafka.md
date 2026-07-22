# Kafka

## Parameters

The Kafka notification service publishes the rendered notification message to a [Kafka](https://kafka.apache.org/) topic. The message body is defined entirely by the template, so any envelope format (for example [CloudEvents](https://cloudevents.io/)) can be produced by the template itself.

* `brokers` - list of Kafka broker addresses (`host:port`) to connect to.
* `topic` - optional, the topic to publish to. It can be overridden per subscription by the recipient (e.g. `subscribe.<trigger>.kafka: my-topic`).
* `tls` - optional, TLS settings:
    * `enabled` - enable TLS for the broker connection.
    * `insecureSkipVerify` - optional, skip broker certificate verification (for private/self-signed clusters).
* `sasl` - optional, SASL authentication:
    * `mechanism` - one of `plain`, `scram-sha-256`, `scram-sha-512` (defaults to `plain`).
    * `username`
    * `password`

## Example

Resource Annotation (the recipient is the Kafka topic):

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-succeeded.kafka: "argocd-events"
```

ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.kafka: |
    brokers:
      - kafka-broker-1.kafka.svc.cluster.local:9092
      - kafka-broker-2.kafka.svc.cluster.local:9092
    topic: argocd-events
    tls:
      enabled: true
    sasl:
      mechanism: scram-sha-512
      username: argocd
      password: $kafka-password
  template.app-sync-succeeded: |
    message: |
      Application {{.app.metadata.name}} has been successfully synced.
    kafka:
      # Optional message key. Kafka uses the key to select the partition, so
      # records with the same key keep their relative order.
      key: "{{.app.metadata.name}}"
```

> ℹ️ Store secrets such as the SASL password in the `argocd-notifications-secret` Secret and reference them with `$<key>`.

### Emitting a CloudEvents payload

Because the message body is templated, a CloudEvents record can be produced directly from the template:

```yaml
  template.app-status-changed: |
    message: |
      {
        "specversion": "1.0",
        "type": "com.example.argocd.app.statuschanged.v1",
        "source": "argocd/{{.app.metadata.namespace}}",
        "id": "{{.app.metadata.name}}-{{.app.status.sync.revision}}",
        "datacontenttype": "application/json",
        "data": {
          "application": "{{.app.metadata.name}}",
          "project": "{{.app.spec.project}}",
          "revision": "{{.app.status.sync.revision}}",
          "health": "{{.app.status.health.status}}"
        }
      }
    kafka:
      key: "{{.app.metadata.name}}"
```
