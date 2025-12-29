package services

import (
	"errors"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

func TestGetTemplater_PagerDutyV2(t *testing.T) {
	t.Run("all parameters specified", func(t *testing.T) {
		n := Notification{
			PagerdutyV2: &PagerDutyV2Notification{
				Summary:   "{{.summary}}",
				Severity:  "{{.severity}}",
				Source:    "{{.source}}",
				Component: "{{.component}}",
				Group:     "{{.group}}",
				Class:     "{{.class}}",
				URL:       "{{.url}}",
				DedupKey:  "{{.dedupKey}}",
			},
		}

		templater, err := n.GetTemplater("", template.FuncMap{})
		if !assert.NoError(t, err) {
			return
		}

		var notification Notification

		err = templater(&notification, map[string]interface{}{
			"summary":   "hello",
			"severity":  "critical",
			"source":    "my-app",
			"component": "test-component",
			"group":     "test-group",
			"class":     "test-class",
			"url":       "http://example.com",
			"dedupKey":  "app-123",
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "hello", notification.PagerdutyV2.Summary)
		assert.Equal(t, "critical", notification.PagerdutyV2.Severity)
		assert.Equal(t, "my-app", notification.PagerdutyV2.Source)
		assert.Equal(t, "test-component", notification.PagerdutyV2.Component)
		assert.Equal(t, "test-group", notification.PagerdutyV2.Group)
		assert.Equal(t, "test-class", notification.PagerdutyV2.Class)
		assert.Equal(t, "http://example.com", notification.PagerdutyV2.URL)
		assert.Equal(t, "app-123", notification.PagerdutyV2.DedupKey)
	})

	t.Run("handle error for summary", func(t *testing.T) {
		n := Notification{
			PagerdutyV2: &PagerDutyV2Notification{
				Summary:   "{{.summary}",
				Severity:  "{{.severity}",
				Source:    "{{.source}",
				Component: "{{.component}",
				Group:     "{{.group}",
				Class:     "{{.class}",
				URL:       "{{.url}",
				DedupKey:  "{{.dedupKey}}",
			},
		}

		_, err := n.GetTemplater("", template.FuncMap{})
		assert.Error(t, err)
	})

	t.Run("handle error for severity", func(t *testing.T) {
		n := Notification{
			PagerdutyV2: &PagerDutyV2Notification{
				Summary:   "{{.summary}}",
				Severity:  "{{.severity}",
				Source:    "{{.source}",
				Component: "{{.component}",
				Group:     "{{.group}",
				Class:     "{{.class}",
				URL:       "{{.url}",
				DedupKey:  "{{.dedupKey}}",
			},
		}

		_, err := n.GetTemplater("", template.FuncMap{})
		assert.Error(t, err)
	})

	t.Run("handle error for source", func(t *testing.T) {
		n := Notification{
			PagerdutyV2: &PagerDutyV2Notification{
				Summary:   "{{.summary}}",
				Severity:  "{{.severity}}",
				Source:    "{{.source}",
				Component: "{{.component}",
				Group:     "{{.group}",
				Class:     "{{.class}",
				URL:       "{{.url}",
				DedupKey:  "{{.dedupKey}}",
			},
		}

		_, err := n.GetTemplater("", template.FuncMap{})
		assert.Error(t, err)
	})

	t.Run("handle error for component", func(t *testing.T) {
		n := Notification{
			PagerdutyV2: &PagerDutyV2Notification{
				Summary:   "{{.summary}}",
				Severity:  "{{.severity}}",
				Source:    "{{.source}}",
				Component: "{{.component}",
				Group:     "{{.group}",
				Class:     "{{.class}",
				URL:       "{{.url}",
				DedupKey:  "{{.dedupKey}}",
			},
		}

		_, err := n.GetTemplater("", template.FuncMap{})
		assert.Error(t, err)
	})

	t.Run("handle error for group", func(t *testing.T) {
		n := Notification{
			PagerdutyV2: &PagerDutyV2Notification{
				Summary:   "{{.summary}}",
				Severity:  "{{.severity}}",
				Source:    "{{.source}}",
				Component: "{{.component}}",
				Group:     "{{.group}",
				Class:     "{{.class}",
				URL:       "{{.url}",
				DedupKey:  "{{.dedupKey}}",
			},
		}

		_, err := n.GetTemplater("", template.FuncMap{})
		assert.Error(t, err)
	})

	t.Run("handle error for class", func(t *testing.T) {
		n := Notification{
			PagerdutyV2: &PagerDutyV2Notification{
				Summary:   "{{.summary}}",
				Severity:  "{{.severity}}",
				Source:    "{{.source}}",
				Component: "{{.component}}",
				Group:     "{{.group}}",
				Class:     "{{.class}",
				URL:       "{{.url}",
				DedupKey:  "{{.dedupKey}}",
			},
		}

		_, err := n.GetTemplater("", template.FuncMap{})
		assert.Error(t, err)
	})

	t.Run("handle error for url", func(t *testing.T) {
		n := Notification{
			PagerdutyV2: &PagerDutyV2Notification{
				Summary:   "{{.summary}}",
				Severity:  "{{.severity}}",
				Source:    "{{.source}}",
				Component: "{{.component}}",
				Group:     "{{.group}}",
				Class:     "{{.class}}",
				URL:       "{{.url}",
				DedupKey:  "{{.dedupKey}}",
			},
		}

		_, err := n.GetTemplater("", template.FuncMap{})
		assert.Error(t, err)
	})

	t.Run("only required parameters specified", func(t *testing.T) {
		n := Notification{
			PagerdutyV2: &PagerDutyV2Notification{
				Summary: "{{.summary}}", Severity: "{{.severity}}", Source: "{{.source}}",
			},
		}

		templater, err := n.GetTemplater("", template.FuncMap{})
		if !assert.NoError(t, err) {
			return
		}

		var notification Notification

		err = templater(&notification, map[string]interface{}{
			"summary":  "hello",
			"severity": "critical",
			"source":   "my-app",
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "hello", notification.PagerdutyV2.Summary)
		assert.Equal(t, "critical", notification.PagerdutyV2.Severity)
		assert.Equal(t, "my-app", notification.PagerdutyV2.Source)
		assert.Equal(t, "", notification.PagerdutyV2.Component)
		assert.Equal(t, "", notification.PagerdutyV2.Group)
		assert.Equal(t, "", notification.PagerdutyV2.Class)
		assert.Equal(t, "", notification.PagerdutyV2.DedupKey)
	})
}

func TestSend_PagerDuty(t *testing.T) {
	t.Run("builds event with full payload", func(t *testing.T) {
		routingKey := "routing-key"
		summary := "test-app failed to deploy"
		severity := "error"
		source := "test-app"
		component := "test-component"
		group := "platform"
		class := "test-class"
		url := "https://www.example.com/"
		dedupKey := "app-123"

		event := buildEvent(routingKey, Notification{
			Message: "message",
			PagerdutyV2: &PagerDutyV2Notification{
				Summary:   summary,
				Severity:  severity,
				Source:    source,
				Component: component,
				Group:     group,
				Class:     class,
				URL:       url,
				DedupKey:  dedupKey,
			},
		})

		assert.Equal(t, routingKey, event.RoutingKey)
		assert.Equal(t, summary, event.Payload.Summary)
		assert.Equal(t, severity, event.Payload.Severity)
		assert.Equal(t, source, event.Payload.Source)
		assert.Equal(t, component, event.Payload.Component)
		assert.Equal(t, group, event.Payload.Group)
		assert.Equal(t, class, event.Payload.Class)
		assert.Equal(t, url, event.ClientURL)
		assert.Equal(t, dedupKey, event.DedupKey)
	})

	t.Run("missing config", func(t *testing.T) {
		service := NewPagerdutyV2Service(PagerdutyV2Options{
			ServiceKeys: map[string]string{
				"test-service": "key",
			},
		})
		err := service.Send(Notification{
			Message: "message",
		}, Destination{
			Service:   "pagerdutyv2",
			Recipient: "test-service",
		})

		if assert.Error(t, err) {
			assert.Equal(t, err, errors.New("no config found for pagerdutyv2"))
		}
	})

	t.Run("missing apiKey", func(t *testing.T) {
		service := NewPagerdutyV2Service(PagerdutyV2Options{})
		err := service.Send(Notification{
			Message: "message",
		}, Destination{
			Service:   "pagerduty",
			Recipient: "test-service",
		})

		if assert.Error(t, err) {
			assert.Equal(t, err, errors.New("no API key configured for recipient test-service"))
		}
	})
}
