# Templates

**Templates**

The notification template is a stateless function that generates the notification content. The templates leverage the [text/template](https://golang.org/pkg/text/template/)
and [Masterminds/Sprig](https://pkg.go.dev/github.com/masterminds/sprig) Golang packages
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

Learn more about service-specific fields in the respective service [documentation](./services/overview.md).
