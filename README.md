# Notifications Engine

Notifications Engine is a configuration-driven Golang library that provides notifications for cloud-native applications.
The project provides integration with dozen of services like Slack, MS Teams, Mattermost, SMTP, Telegram, Netgenie, and the list keeps growing.

<p align="center">
<img width="460" src="https://user-images.githubusercontent.com/426437/115815221-70139a00-a3ab-11eb-8dc9-3e15f6b17804.png">
</p>

## Why Use It?

The first class notifications support is often eschewed feature in Kubernetes controllers. This is challenging because
notifications are very opinionated by nature. It is hard to predict what kind of events end-users want to be notified
about and especially how the notification should look like. Additionally, there are lots of notification services so it
is hard to decide which one to support first.The Notifications Engine is trying to tackle both challenges:

* provides a flexible configuration-driven mechanism of triggers and templates and allows CRD controller
  administrators to accommodate end-user requirements without making any code changes;
* out of the box integrates with dozen of notifications services (Slack, SMTP, Telegram etc) with many integrations yet to come;

## Features

Using the engine CRD controller administrators can configure a set of [triggers](./docs/triggers.md) and [templates](./docs/templates.md)
and enable end-users to subscribe to the required triggers by just annotating custom resources they care about.

The example below demonstrates the [Argo CD](https://github.com/argoproj/argo-cd) specific configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  trigger.on-sync-status-unknown: |
    - when: app.status.sync.status == 'Unknown'
      send: [app-sync-status]

  template.app-sync-status: |
    message: |
      Application {{.app.metadata.name}} sync is {{.app.status.sync.status}}.
      Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.

  service.slack: |
    token: $slack-token
---
apiVersion: v1
kind: Secret
metadata:
  name: argocd-notifications-secret
stringData:
  slack-token: <my-slack-token>
```

The end-user can subscribe to the triggers they are interested in by adding  `notifications.argoproj.io/subscribe/<trigger>/<service>: <recipients>` annotation:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-succeeded.slack: my-channel1;my-channel2
```

## Getting Started

Ready to add notifications to your project? Check out sample notifications for [cert-manager](./examples/certmanager/README.md)

## Users

* [Argo CD](https://github.com/argoproj/argo-cd) (implemented by [argocd-notifications](https://github.com/argoproj-labs/argocd-notifications))
* [Argo Rollouts](https://github.com/argoproj/argo-rollouts)

# Additional Resources

* [Proposal document](https://docs.google.com/document/d/1nw0i7EAehNnjEkbpx-I3BVjfZvRgetUFUZby4iMUSWU/edit)
* [Argoproj notifications blog post](https://blog.argoproj.io/notifications-for-argo-bb7338231604)
