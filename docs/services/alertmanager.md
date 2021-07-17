# Alertmanager

## Parameters

The notification service is used to push events to [Alertmanager](https://github.com/prometheus/alertmanager), and the following settings need to be specified:

* `targets` - the alertmanager server address is an array, you can configure multiple
* `scheme` - default is "http", e.g. http or https
* `apiPath` - default is "/api/v2/alerts"
* `insecureSkipVerify` - default is "false", when scheme is https whether to skip the verification of ca
* `basicAuth` - server auth
* `bearerToken` - server auth

## Templates

```yaml
context: |
  argocdUrl: https://example.com/argocd

template.app-deployed: |
  message: Application {{.app.metadata.name}} has been healthy.
  alertmanager:
    labels:
      fault_priority: "P5"
      event_bucket: "deploy"
      event_status: "succeed"
      recipient: "{{.recipient}}"
    annotations:
      application: '<a href="{{.context.argocdUrl}}/applications/{{.app.metadata.name}}">{{.app.metadata.name}}</a>'
      author: "{{(call .repo.GetCommitMetadata .app.status.sync.revision).Author}}"
      message: "{{(call .repo.GetCommitMetadata .app.status.sync.revision).Message}}"
```

You can do targeted push on [Alertmanager](https://github.com/prometheus/alertmanager) according to labels.

## Example

### Prometheus Alertmanager config

```yaml
global:
  resolve_timeout: 5m

route:
  group_by: ['alertname']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'default'
receivers:
- name: 'default'
  webhook_configs:
  - send_resolved: false
    url: 'http://10.5.39.39:10080/api/alerts/webhook'
```

You should turn off "send_resolved" or you will receive unnecessary recovery notifications after "resolve_timeout".

### Send one alertmanager

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.alertmanager: |
    targets:
    - 10.5.39.39:9093
```

### Send alertmanager cluster with auth

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.alertmanager: |
    targets:
    - 10.5.39.39:19093
    - 10.5.39.39:29093
    - 10.5.39.39:39093
    scheme: https
    #apiPath: /api/events
    #insecureSkipVerify: true
    #basicAuth:
    #  username: $alertmanager-username
    #  password: $alertmanager-password   
    bearerToken: $alertmanager-bearer-token
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  alertmanager-username: <username>
  alertmanager-password: <password>
  alertmanager-bearer-token: <token>
```

* "basicAuth" or "bearerToken" are used for authentication, and you can choose one.
* If your alertmanager has changed the default api, you can customize "apiPath".