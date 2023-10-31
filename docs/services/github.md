# GitHub

## Parameters

The GitHub notification service changes commit status using [GitHub Apps](https://docs.github.com/en/developers/apps) and requires specifying the following settings:

* `appID` - the app id
* `installationID` - the app installation id
* `privateKey` - the app private key
* `enterpriseBaseURL` - optional URL, e.g. https://git.example.com/

## Configuration

1. Create a GitHub Apps using https://github.com/settings/apps/new
2. Change repository permissions to enable write commit statuses and/or deployments and/or pull requests comments
![2](https://user-images.githubusercontent.com/18019529/108397381-3ca57980-725b-11eb-8d17-5b8992dc009e.png)
3. Generate a private key, and download it automatically
![3](https://user-images.githubusercontent.com/18019529/108397926-d4a36300-725b-11eb-83fe-74795c8c3e03.png)
4. Install app to account
5. Store privateKey in `argocd-notifications-secret` Secret and configure GitHub integration
in `argocd-notifications-cm` ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.github: |
    appID: <app-id>
    installationID: <installation-id>
    privateKey: $github-privateKey
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  github-privateKey: |
    -----BEGIN RSA PRIVATE KEY-----
    (snip)
    -----END RSA PRIVATE KEY-----
```

6. Create subscription for your GitHub integration

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.<trigger-name>.github: ""
```

## Templates

![](https://user-images.githubusercontent.com/18019529/108520497-168ce180-730e-11eb-93cb-b0b91f99bdc5.png)

```yaml
template.app-deployed: |
  message: |
    Application {{.app.metadata.name}} is now running new version of deployments manifests.
  github:
    repoURLPath: "{{.app.spec.source.repoURL}}"
    revisionPath: "{{.app.status.operationState.syncResult.revision}}"
    status:
      state: success
      label: "continuous-delivery/{{.app.metadata.name}}"
      targetURL: "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
    deployment:
      state: success
      environment: production
      environmentURL: "https://{{.app.metadata.name}}.example.com"
      logURL: "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
      requiredContexts: []
      autoMerge: true
    pullRequestComment:
      content: |
        Application {{.app.metadata.name}} is now running new version of deployments manifests.
        See more here: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true
```

**Notes**:
- If the message is set to 140 characters or more, it will be truncated.
- If `github.repoURLPath` and `github.revisionPath` are same as above, they can be omitted.
- Automerge is optional and `true` by default for github deployments to ensure the requested ref is up to date with the default branch.
  Setting this option to `false` is required if you would like to deploy older refs in your default branch.
  For more information see the [GitHub Deployment API Docs](https://docs.github.com/en/rest/deployments/deployments?apiVersion=2022-11-28#create-a-deployment).
- If `github.pullRequestComment.content` is set to 65536 characters or more, it will be truncated.

# Supported API

## Status
[Api Docs](https://docs.github.com/en/rest/commits/statuses)

### Example
```yaml
template.app-deployed: |
  message: |
    Application {{.app.metadata.name}} is now running new version of deployments manifests.
  github:
    repoURLPath: "{{.app.spec.source.repoURL}}"
    revisionPath: "{{.app.status.operationState.syncResult.revision}}"
    status:
      state: success
      label: "continuous-delivery/{{.app.metadata.name}}"
      targetURL: "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
```
## Deployment
[Api Docs](https://docs.github.com/en/rest/deployments/deployments)
### Example
```yaml
template.app-deployed: |
  github:
    repoURLPath: "{{.app.spec.source.repoURL}}"
    revisionPath: "{{.app.status.operationState.syncResult.revision}}"
    deployment:
      state: success
      environment: production
      environmentURL: "https://{{.app.metadata.name}}.example.com"
      logURL: "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
      requiredContexts: []
      autoMerge: true
```
## PullRequestComment
[Api Docs](https://docs.github.com/en/rest/issues/comments#create-an-issue-comment)
### Example
```yaml
template.app-deployed: |
  github:
    repoURLPath: "{{.app.spec.source.repoURL}}"
    revisionPath: "{{.app.status.operationState.syncResult.revision}}"
    pullRequestComment:
      content: |
        Application {{.app.metadata.name}} is now running new version of deployments manifests.
        See more here: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true
```
## CheckRuns
[Api Docs](https://docs.github.com/en/rest/checks/runs)
### Example

Create new checkrun for the current commit:

```yaml
template.app-deployed: |
  github:
    repoURLPath: "{{.app.spec.source.repoURL}}"
    revisionPath: "{{.app.status.operationState.syncResult.revision}}"
    checkRun:
      name: "Deployment"
      status: "{{if and (eq .app.status.operationState.phase "Succeeded") (eq .app.status.health.status "Healthy")}}"success"{{else}}"failure"{{end}}"
      completed_at: "{{ (call .time.Now).Format "2006-01-02T15:04:05Z07:00" }}",
      conclusion: "completed"
```

Update existing checkrun:

```yaml
template.app-deployed: |
  github:
    repoURLPath: "{{.app.spec.source.repoURL}}"
    revisionPath: "not used"
    checkRun:
      name: "Deployment"
      id: "{{ (call .repo.GetAppDetails).Helm.GetParameterValueByName "git_check_id" }}"
      status: "{{if and (eq .app.status.operationState.phase "Succeeded") (eq .app.status.health.status "Healthy")}}"success"{{else}}"failure"{{end}}"
      details_url": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?",
      completed_at: "{{ (call .time.Now).Format "2006-01-02T15:04:05Z07:00" }}",
      conclusion: "completed"
```