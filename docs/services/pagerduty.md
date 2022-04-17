# Pagerduty

The [PagerDuty Events API v2](https://developer.pagerduty.com/docs/ZG9jOjExMDI5NTgw-events-api-v2-overview#events-api-v2-overview) is a highly available asynchronous API that routes events sent to a [PagerDuty service](https://support.pagerduty.com/docs/services-and-integrations) for processing. can ingest multiple types of events. Each event type is described below

| API | Type | Description | Example |
|---|---|---|---|
| PagerDuty v2 Events API | Alert Event | A problem in a machine monitored system.   Follow up events can be sent to acknowledge or resolve an existing alert. | - High error rate - CPU usage exceeded limit - Deployment failed |
| PagerDuty v2 Events API | Change Event | A change in a system that does not represent a problem. | Pull request merged   Secret successfully rotated   Configuration update applied |

## Parameters

The Pagerduty notification service is used to create pagerduty incidents and requires specifying the following settings:

* `pagerdutyToken` - the pagerduty auth token
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
  pagerdutyToken: <pd-api-token>
```

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-map-name>
data:
  service.pagerduty: |
    token: $pagerdutyToken
    from: <emailid>
```

## Template

Notification templates support specifying subject for pagerduty notifications:

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

## Annotation

Annotation sample for pagerduty notifications:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-rollout-aborted.pagerduty: "<serviceID for Pagerduty>"
```