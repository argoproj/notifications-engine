# Pagerduty

## Parameters

The Pagerduty notification service is used to create pagerduty incidents and requires specifying the following settings:

* `pagerduty-token` - the pagerduty auth token
* `from` - email address of a valid user associated with the account making the request.
* `serviceID` - The ID of the resource.


## Example

The following snippet contains sample Pagerduty service configuration:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
stringData:
  pagerduty-token: <pd-api-token>
```

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.pagerduty: |
    token: $pagerduty-token
    from: <emailid>
    serviceID: <serviceID of PagerDuty>
    escalationPolicyID: <escalationPolicyID of pagerduty channel>
```

## Template

Notification templates support specifying subject for email notifications:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  template.rollout-aborted: |
    message: Rollout {{.rollout.metadata.name}} is aborted.
    pagerduty:
      title: "Rollout {{.rollout.metadata.name}}"
      urgency: "high"
      body: "Rollout {{.rollout.metadata.name}} aborted "
      priorityID: "<priorityID of incident>"
```

NOTE: A Priority is a label representing the importance and impact of an incident. This is only available on Standard and Enterprise plans of pagerduty.
