# GitLab

## Parameters

The GitLab notification service sets commit statuses, creates deployments and comments on the merge requests
associated with a commit, and requires specifying the following settings:

- `token` - the access token used to authenticate against the GitLab API
- `baseURL` - optional URL of the GitLab instance, e.g. https://gitlab.example.com. Defaults to https://gitlab.com
- `insecureSkipVerify` - optional bool, true or false
- `maxIdleConns` - optional, maximum number of idle (keep-alive) connections across all hosts.
- `maxIdleConnsPerHost` - optional, maximum number of idle (keep-alive) connections per host.
- `maxConnsPerHost` - optional, maximum total connections per host.
- `idleConnTimeout` - optional, maximum amount of time an idle (keep-alive) connection will remain open before closing.

The `/api/v4` suffix is appended automatically, so `baseURL` takes the instance root.

## Configuration

1. Create an [access token](https://docs.gitlab.com/user/profile/personal_access_tokens/) with the `api` scope, granted
   a role that may write commit statuses, deployments and merge request comments on the project. A
   [project](https://docs.gitlab.com/user/project/settings/project_access_tokens/) or
   [group](https://docs.gitlab.com/user/group/settings/group_access_tokens/) access token also works
2. Store the token in `argocd-notifications-secret` Secret and configure the GitLab integration
   in `argocd-notifications-cm` ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-notifications-cm
data:
  service.gitlab: |
    token: $gitlab-token
    baseURL: https://gitlab.example.com
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  gitlab-token: <access-token>
```

3. Create subscription for your GitLab integration

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    notifications.argoproj.io/subscribe.<trigger-name>.gitlab: ""
```

## Templates

```yaml
template.app-deployed: |
  message: |
    Application {{.app.metadata.name}} is now running new version of deployments manifests.
  gitlab:
    repoURLPath: "{{.app.spec.source.repoURL}}"
    revisionPath: "{{.app.status.operationState.syncResult.revision}}"
    status:
      state: success
      label: "continuous-delivery/{{.app.metadata.name}}"
      targetURL: "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
    deployment:
      state: success
      environment: production
      reference: v1.0.0
      tag: true
    mergeRequestComment:
      content: |
        Application {{.app.metadata.name}} is now running new version of deployments manifests.
        See more here: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true
```

**Notes**:

- If `gitlab.repoURLPath` and `gitlab.revisionPath` are same as above, they can be omitted.
- `gitlab.status.state` accepts `pending`, `running`, `success`, `failed` or `canceled`.
- The commit status description is taken from the message and is truncated to 255 characters, the maximum GitLab
  accepts.
- `gitlab.deployment.state` accepts `created`, `running`, `success`, `failed` or `canceled`.
- `gitlab.deployment.reference` is optional. When set, it is used as the ref to deploy, and `gitlab.deployment.tag`
  should be `true` if that ref is a tag. If not set, the revision is used as the ref.
- The environment named by `gitlab.deployment.environment` is created by GitLab if it does not already exist.
- `gitlab.mergeRequestComment.content` is posted to every open merge request associated with the revision.
