package subscriptions

import (
	"testing"

	"github.com/argoproj/notifications-engine/pkg/services"

	"github.com/stretchr/testify/assert"
)

func TestIterate(t *testing.T) {
	tests := []struct {
		annotations map[string]string
		trigger     string
		service     string
		recipients  []string
		key         string
	}{
		{
			annotations: map[string]string{
				"notifications.argoproj.io/subscribe.my-trigger.slack": "my-channel",
			},
			trigger:    "my-trigger",
			service:    "slack",
			recipients: []string{"my-channel"},
			key:        "notifications.argoproj.io/subscribe.my-trigger.slack",
		},
		{
			annotations: map[string]string{
				"notifications.argoproj.io/subscribe..slack": "my-channel",
			},
			trigger:    "",
			service:    "slack",
			recipients: []string{"my-channel"},
			key:        "notifications.argoproj.io/subscribe..slack",
		},
		{
			annotations: map[string]string{
				"notifications.argoproj.io/subscribe.slack": "my-channel",
			},
			trigger:    "",
			service:    "slack",
			recipients: []string{"my-channel"},
			key:        "notifications.argoproj.io/subscribe.slack",
		},
	}

	for _, tt := range tests {
		a := Annotations(tt.annotations)
		a.iterate(func(trigger, service string, recipients []string, key string) {
			assert.Equal(t, tt.trigger, trigger)
			assert.Equal(t, tt.service, service)
			assert.Equal(t, tt.recipients, recipients)
			assert.Equal(t, tt.key, key)
		})
	}
}

func TestGetDestinations(t *testing.T) {
	tests := []struct {
		subscriptions         Annotations
		defaultTrigger        []string
		serviceDefaultTrigger map[string][]string
		result                services.Destinations
	}{
		{
			subscriptions: Annotations(map[string]string{
				"notifications.argoproj.io/subscribe.my-trigger.slack": "my-channel",
			}),
			defaultTrigger: []string{},
			result: services.Destinations{
				"my-trigger": []services.Destination{{
					Service:   "slack",
					Recipient: "my-channel",
				}},
			},
		},
		{
			subscriptions: Annotations(map[string]string{
				"notifications.argoproj.io/subscribe.my-trigger.slack": "my-channel",
			}),
			defaultTrigger: []string{
				"trigger-a",
				"trigger-b",
				"trigger-c",
			},
			result: services.Destinations{
				"my-trigger": []services.Destination{{
					Service:   "slack",
					Recipient: "my-channel",
				}},
			},
		},
		{
			subscriptions: Annotations(map[string]string{
				"notifications.argoproj.io/subscribe.slack": "my-channel",
			}),
			defaultTrigger: []string{
				"trigger-a",
				"trigger-b",
				"trigger-c",
			},
			result: services.Destinations{
				"trigger-a": []services.Destination{{
					Service:   "slack",
					Recipient: "my-channel",
				}},
				"trigger-b": []services.Destination{{
					Service:   "slack",
					Recipient: "my-channel",
				}},
				"trigger-c": []services.Destination{{
					Service:   "slack",
					Recipient: "my-channel",
				}},
			},
		},
		{
			subscriptions: Annotations(map[string]string{
				"notifications.argoproj.io/subscribe.slack": "my-channel",
			}),
			defaultTrigger: []string{
				"trigger-a",
				"trigger-b",
				"trigger-c",
			},
			serviceDefaultTrigger: map[string][]string{
				"slack": {
					"trigger-d",
					"trigger-e",
				},
			},
			result: services.Destinations{
				"trigger-d": []services.Destination{{
					Service:   "slack",
					Recipient: "my-channel",
				}},
				"trigger-e": []services.Destination{{
					Service:   "slack",
					Recipient: "my-channel",
				}},
			},
		},
	}

	for _, tt := range tests {
		dests := tt.subscriptions.GetDestinations(tt.defaultTrigger, tt.serviceDefaultTrigger)
		assert.Equal(t, tt.result, dests)
	}
}

func TestSubscribe(t *testing.T) {
	a := Annotations(map[string]string{})
	a.Subscribe("my-trigger", "slack", "my-channel1")

	assert.Equal(t, a["notifications.argoproj.io/subscribe.my-trigger.slack"], "my-channel1")
}

func TestSubscribe_AddSecondRecipient(t *testing.T) {
	a := Annotations(map[string]string{
		"notifications.argoproj.io/subscribe.my-trigger.slack": "my-channel1",
	})
	a.Subscribe("my-trigger", "slack", "my-channel2")

	assert.Equal(t, a["notifications.argoproj.io/subscribe.my-trigger.slack"], "my-channel1;my-channel2")
}

func TestUnsubscribe(t *testing.T) {
	a := Annotations(map[string]string{
		"notifications.argoproj.io/subscribe.my-trigger.slack": "my-channel1;my-channel2",
	})
	a.Unsubscribe("my-trigger", "slack", "my-channel1")
	assert.Equal(t, a["notifications.argoproj.io/subscribe.my-trigger.slack"], "my-channel2")
	a.Unsubscribe("my-trigger", "slack", "my-channel2")
	_, ok := a["notifications.argoproj.io/subscribe.my-trigger.slack"]
	assert.False(t, ok)
}
