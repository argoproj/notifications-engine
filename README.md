# Notifications Engine

Notifications Engine is a configuration-driven Golang library that provides notifications for cloud-native applications.
The project provides integration with dozen of services like Slack, MS Teams, Mattermost, SMTP, Telegram, Netgenie, and the list keeps growing.

<p align="center">
<img width="460" src="https://user-images.githubusercontent.com/426437/115815221-70139a00-a3ab-11eb-8dc9-3e15f6b17804.png">
</p>

## Who Should Use It?

The library might be helpful if you are working on a custom Kubernetes resource (CRD) and want to provide lightweight notifications mechanism to end users of your CRD.
Using the library with nearly no coding you can get a simple controller that "monitors" your CRD and produces notifications defined in the configuration by the end-users.

## Features

The notifications engine provides a flexible mechanism to configure a list of triggers and templates and use it to power notifications for your
project. The core project concepts are configuration-driven triggers, templates, notification services, and subscriptions.

> Note: examples below are using [Argo CD](https://github.com/argoproj/argo-cd) Application CRD as an example.

**Triggers**

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

**Templates**

The notification template is a stateless function that generates the notification content. The templates are leveraging [html/template](https://golang.org/pkg/html/template/) Golang package
and allow you to customize notification messages. Templates are meant to be reusable and can be referenced by multiple triggers:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  template.app-sync-status: |
    message: |
      Application {{.app.metadata.name}} sync is {{.app.status.sync.status}}.
      Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
```

Each template must define "basic" `message` template and optionally includes notification service specific fields: 

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  template.app-sync-status: |
    message: |
      Application {{.app.metadata.name}} sync is {{.app.status.sync.status}}.
      Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
    slack:
      attachments: |
        [{
          "title": "{{.app.metadata.name}}",
          "title_link": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
          "color": "#18be52",
          "fields": [{
            "title": "Sync Status",
            "value": "{{.app.status.sync.status}}",
            "short": true
          }, {
            "title": "Repository",
            "value": "{{.app.spec.source.repoURL}}",
            "short": true
          }]
        }]
```

**Notification Services**

The notification services implements integration with services such as slack, email, custom webhook etc. Services are configured in Kubernetes ConfigMap and Secret:
service configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.slack: |
    token: $slack-token
---
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  slack-token: <my-slack-token>
```

Learn more about supported services [here](./docs/services/overview.md).

**Subscriptions**

Finally, subscriptions allow end-users subscribe to the triggers they are interested in. In order to subscribe the user needs to add
the `notifications.argoproj.io/subscribe/<trigger>/<service>: <recipients>` annotation the monitored Kubernetes resources.

* **trigger** - the name of a trigger that should be evaluated
* **service** - the name of a notification service
* **recipients** - the semicolon separated list of recipients

Example:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-succeeded.slack: my-channel1;my-channel2
```

## Putting It All Together

The sample controller implementation is available at https://github.com/argoproj-labs/argocd-notifications/blob/master/controller/controller.go

## Users

* [Argo CD](https://github.com/argoproj/argo-cd) (implemented by [argocd-notifications](https://github.com/argoproj-labs/argocd-notifications))
* [Argo Rollouts](https://github.com/argoproj/argo-rollouts) (👷 work in progress...)

# Additional Resources

* [Proposal document](https://docs.google.com/document/d/1nw0i7EAehNnjEkbpx-I3BVjfZvRgetUFUZby4iMUSWU/edit)
* [Argoproj notifications blog post](https://blog.argoproj.io/notifications-for-argo-bb7338231604)
