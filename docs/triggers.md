# Triggers

The trigger is a named condition that monitors your Kubernetes resource and decides if it is time to send the notification. The trigger definition
includes name, condition, and templates reference.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  trigger.on-sync-status-unknown: |
    - when: app.status.sync.status == 'Unknown'
      send: [app-sync-status]
```

* **trigger.\<name\>** - trigger name
* **when** - a predicate expression that returns true or false. The expression evaluation is powered by [antonmedv/expr](https://github.com/antonmedv/expr).
  The condition language syntax is described at [Language-Definition.md](https://github.com/antonmedv/expr/blob/master/docs/Language-Definition.md).
* **send** - the templates list that should be used to generate a notification.

### oncePer

The notification is sent when the trigger flips from `false` to `true`. If you need to send a notification
when another field changes you might use the `oncePer` field. The `oncePer` filed is supported like as follows.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    example.com/version: v0.1
```

```yaml
oncePer: app.metadata.annotations["example.com/version"]
```
