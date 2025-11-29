# Teams Workflows

## Overview

The Teams Workflows notification service sends message notifications using Microsoft Teams Workflows (Power Automate). This is the recommended replacement for the legacy Office 365 Connectors service, which will be retired on March 31, 2026.

## Parameters

The Teams Workflows notification service requires specifying the following settings:

* `recipientUrls` - the webhook url map, e.g. `channelName: https://api.powerautomate.com/webhook/...`

## Supported Webhook URL Formats

The service supports the following Microsoft Teams Workflows webhook URL patterns:

- `https://api.powerautomate.com/...`
- `https://api.powerplatform.com/...`
- `https://flow.microsoft.com/...`
- `https://webhook.office.com/workflows/...`
- URLs containing `/powerautomate/` in the path

## Configuration

1. Open Microsoft Teams and navigate to the channel where you want to receive notifications
2. Click **Workflows** in the channel menu
3. Click **Create** and select **When a webhook request is received**
4. Configure the workflow and copy the webhook URL
5. Store the webhook URL in `argocd-notifications-secret` and define it in `argocd-notifications-cm`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.teams-workflows: |
    recipientUrls:
      channelName: $channel-workflows-url
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  channel-workflows-url: https://api.powerautomate.com/webhook/...
```

6. Create subscription for your Teams Workflows integration:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-succeeded.teams-workflows: channelName
```

## Channel Support

- ✅ Standard Teams channels
- ✅ Shared channels (as of December 2025)
- ✅ Private channels (as of December 2025)

Teams Workflows provides enhanced channel support compared to Office 365 Connectors, allowing you to post to shared and private channels in addition to standard channels.

## Card Formats

The Teams Workflows service supports two card formats:

1. **messageCard** (default) - Compatible with Office 365 Connector format for easy migration
2. **Adaptive Card** - Modern card format with enhanced capabilities

### Using messageCard Format (Default)

The default format uses the messageCard schema, which is compatible with Office 365 Connectors. You can use all the same template fields as the Teams service:

```yaml
template.app-sync-succeeded: |
  teams-workflows:
    themeColor: "#000080"
    sections: |
      [{
        "facts": [
          {
            "name": "Sync Status",
            "value": "{{.app.status.sync.status}}"
          },
          {
            "name": "Repository",
            "value": "{{.app.spec.source.repoURL}}"
          }
        ]
      }]
    potentialAction: |-
      [{
        "@type":"OpenUri",
        "name":"Operation Details",
        "targets":[{
          "os":"default",
          "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
        }]
      }]
    title: Application {{.app.metadata.name}} has been successfully synced
    text: Application {{.app.metadata.name}} has been successfully synced at {{.app.status.operationState.finishedAt}}.
    summary: "{{.app.metadata.name}} sync succeeded"
```

### Using Adaptive Cards

Adaptive Cards provide a more flexible and modern card format. You can either:

#### Option 1: Use cardFormat field

The service will automatically convert your messageCard fields to Adaptive Card format:

```yaml
template.app-sync-succeeded: |
  teams-workflows:
    cardFormat: "adaptiveCard"
    title: Application {{.app.metadata.name}} has been successfully synced
    text: Application {{.app.metadata.name}} has been successfully synced.
    facts: |
      [{
        "name": "Sync Status",
        "value": "{{.app.status.sync.status}}"
      }]
```

#### Option 2: Provide custom Adaptive Card JSON

For full control, you can provide a complete Adaptive Card JSON:

```yaml
template.app-sync-succeeded: |
  teams-workflows:
    adaptiveCard: |
      {
        "type": "message",
        "version": "1.4",
        "body": [
          {
            "type": "TextBlock",
            "text": "Application {{.app.metadata.name}} synced successfully",
            "size": "Large",
            "weight": "Bolder"
          },
          {
            "type": "FactSet",
            "facts": [
              {
                "title": "Sync Status",
                "value": "{{.app.status.sync.status}}"
              },
              {
                "title": "Repository",
                "value": "{{.app.spec.source.repoURL}}"
              }
            ]
          }
        ],
        "actions": [
          {
            "type": "Action.OpenUrl",
            "title": "View Details",
            "url": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}"
          }
        ]
      }
```

## Template Fields

The Teams Workflows service supports the same template fields as the Teams service for messageCard format:

- `title` - Message title
- `text` - Message text content
- `summary` - Summary text shown in notifications feed
- `themeColor` - Hex color code for message theme
- `sections` - JSON array of message sections
- `facts` - JSON array of fact key-value pairs
- `potentialAction` - JSON array of action buttons
- `template` - Raw JSON template (overrides other fields)
- `adaptiveCard` - Custom Adaptive Card JSON
- `cardFormat` - Set to `"adaptiveCard"` to auto-convert messageCard fields

## Migration from Office 365 Connectors

If you're currently using the `teams` service with Office 365 Connectors, follow these steps to migrate:

1. **Create a new Workflows webhook** using the configuration steps above

2. **Update your service configuration:**
   - Change from `service.teams` to `service.teams-workflows`
   - Update the webhook URL to your new Workflows webhook URL

3. **Update your templates:**
   - Change `teams:` to `teams-workflows:` in your templates
   - Your existing messageCard format templates will work as-is

4. **Update your subscriptions:**
   ```yaml
   # Old
   notifications.argoproj.io/subscribe.on-sync-succeeded.teams: channelName
   
   # New
   notifications.argoproj.io/subscribe.on-sync-succeeded.teams-workflows: channelName
   ```

5. **Test and verify:**
   - Send a test notification to verify it works correctly
   - Once verified, you can remove the old Office 365 Connector configuration

**Note:** Your existing templates using messageCard format will work without modification. The service maintains compatibility with the same messageCard schema used by Office 365 Connectors.

## Differences from Office 365 Connectors

| Feature | Office 365 Connectors | Teams Workflows |
|---------|----------------------|-----------------|
| Service Name | `teams` | `teams-workflows` |
| Standard Channels | ✅ | ✅ |
| Shared Channels | ❌ | ✅ (Dec 2025+) |
| Private Channels | ❌ | ✅ (Dec 2025+) |
| Card Formats | messageCard only | messageCard + Adaptive Cards |
| Retirement Date | March 31, 2026 | Active |

