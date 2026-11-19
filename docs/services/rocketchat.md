# Rocket.Chat

## Parameters

The Rocket.Chat notification service configuration includes following settings:

* `email` - the Rocker.Chat user's SAMAccountName (login mode only)
* `password` - the Rocker.Chat user's password (login mode only)
* `webhookUrl` - Incoming Webhook URL (webhook mode only, see below). Takes precedence over `email`/`password` if set
* `alias` - optional alias that should be used to post message
* `icon` - optional message icon
* `avatar` - optional message avatar
* `serverUrl` - optional Rocket.Chat server url (login mode only)
* `insecureSkipVerify` - optional, skip TLS certificate verification (webhook mode only)

## Webhook configuration (recommended)

Using an Incoming Webhook avoids storing a bot user's password entirely and is the recommended way to integrate. TLS to a self-hosted instance behind an internal CA is supported the same way as the other webhook-based services (Teams, Mattermost).

1. Login to your Rocket.Chat instance as admin
2. Go to **Administration > Integrations > New > Incoming Webhook**
3. Enable it, pick the default channel, and save
4. *(Optional, only if you need per-application channels)* enable **"Allow to overwrite destination channel in the body parameters"** on the integration. Without this, Rocket.Chat silently ignores the `channel` this library sends and every notification lands on the integration's fixed default channel, regardless of what's set on the `notifications.argoproj.io/subscribe...` annotation. This setting is opt-in because an unrestricted override lets any webhook caller post into channels/DMs it wasn't meant to reach — only enable it if you trust everything that can call this webhook URL.
5. Copy the generated Webhook URL
6. Store it in the `argocd-notifications-secret` Secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  rocketchat-webhook-url: <webhook-url>
```

7. Configure the integration in the `argocd-notifications-cm` config map:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.rocketchat: |
    webhookUrl: $rocketchat-webhook-url
```

8. Create a subscription for your Rocket.Chat integration, same as in login mode (see below). The channel/user set on the subscription only takes effect if step 4 above was done; otherwise it's ignored and the message always goes to the integration's default channel.

## Login configuration (user/password)

1. Login to your RocketChat instance
2. Go to user management

![2](https://user-images.githubusercontent.com/15252187/115824993-7ccad900-a411-11eb-89de-6a0c4438ffdf.png)

3. Add new user with `bot` role. Also note that `Require password change` checkbox mus be not checked

![3](https://user-images.githubusercontent.com/15252187/115825174-b4d21c00-a411-11eb-8f20-cda48cea9fad.png)

4. Copy username and password that you was created for bot user
5. Create a public or private channel, or a team, for this example `my_channel`
6. Add your bot to this channel **otherwise it won't work**
7. Store email and password in argocd-notifications-secret Secret
 
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  rocketchat-email: <email>
  rocketchat-password: <password>
```

8. Finally, use these credentials to configure the RocketChat integration in the `argocd-configmap` config map: 

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.rocketchat: |
    email: $rocketchat-email
    password: $rocketchat-password
```

9. Create a subscription for your Rocket.Chat integration:

*Note: channel, team or user must be prefixed with # or @ elsewhere we will be interpretative destination as a room ID*

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-succeeded.rocketchat: #my_channel
```

## Templates

[Notification templates](../templates.md) can be customized with RocketChat [attachments](https://developer.rocket.chat/api/rest-api/methods/chat/postmessage#attachments-detail).

*Note: Attachments structure in Rocketchat is same with Slack attachments [feature](https://api.slack.com/messaging/composing/layouts).*

<!-- TODO: @sergeyshevch Need to add screenshot with RocketChat attachments -->

The message attachments can be specified in `attachments` string fields under `rocketchat` field:

```yaml
template.app-sync-status: |
  message: |
    Application {{.app.metadata.name}} sync is {{.app.status.sync.status}}.
    Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
  rocketchat:
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
