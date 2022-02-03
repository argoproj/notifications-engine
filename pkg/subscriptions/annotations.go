package subscriptions

import (
	"fmt"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"strings"

	"github.com/argoproj/notifications-engine/pkg/services"
)

const (
	AnnotationPrefix = "notifications.argoproj.io"
)

func parseRecipients(v string) []string {
	var recipients []string
	for _, recipient := range strings.Split(v, ";") {
		if recipient = strings.TrimSpace(recipient); recipient == "" {
			continue
		}
		recipients = append(recipients, recipient)
	}
	return recipients
}

func SubscribeAnnotationKey(trigger string, service string) string {
	return fmt.Sprintf("%s/subscribe.%s.%s", AnnotationPrefix, trigger, service)
}

type Annotations map[string]string

func NewAnnotations(annotations map[string]string) Annotations {
	if annotations == nil {
		return Annotations(map[string]string{})
	}

	return Annotations(annotations)
}

type Subscription struct {
	Trigger      []string
	Destinations []Destination
}

// Destination holds notification destination details
type Destination struct {
	Service    string   `json:"service"`
	Recipients []string `json:"recipients"`
}

func (a Annotations) iterate(callback func(trigger string, service string, recipients []string, key string)) {
	prefix := AnnotationPrefix + "/subscribe."
	altPrefix := AnnotationPrefix + "/subscribe_yaml"
	var recipients []string
	for k, v := range a {
		switch {
		case strings.HasPrefix(k, prefix):
			parts := strings.Split(k[len(prefix):], ".")
			trigger := parts[0]
			service := ""
			if len(parts) >= 2 {
				service = parts[1]
			} else {
				service = parts[0]
				trigger = ""
			}
			if v == "" {
				recipients = []string{""}
			} else {
				recipients = parseRecipients(v)
			}
			callback(trigger, service, recipients, k)
		case strings.HasPrefix(k, altPrefix):
			var subscriptions []Subscription
			source := []byte(v)
			err := yaml.Unmarshal(source, &subscriptions)
			if err != nil {
				log.Errorf("error: %v", err)
				callback("", "", recipients, k)
			}
			for _, v := range subscriptions {
				if len(v.Trigger) == 0 {
					log.Printf("Triggers are not configured")
				}
				if len(v.Destinations) == 0 {
					log.Printf("Destinaitons are not configured")
				}
				triggers := v.Trigger
				destinations := v.Destinations
				for _, trigger := range triggers {
					for _, destination := range destinations {
						log.Printf("trigger: %v, service: %v, recipient: %v \n", trigger, destination.Service, destination.Recipients)
						callback(trigger, destination.Service, destination.Recipients, k)
					}
				}
			}
		default:
			callback("", "", recipients, k)
		}
	}
}

func (a Annotations) Subscribe(trigger string, service string, recipients ...string) {
	annotationKey := SubscribeAnnotationKey(trigger, service)
	r := parseRecipients(a[annotationKey])
	set := map[string]bool{}
	for _, recipient := range r {
		set[recipient] = true
	}
	for _, recipient := range recipients {
		if !set[recipient] {
			r = append(r, recipient)
		}
	}

	a[annotationKey] = strings.Join(r, ";")
}

func (a Annotations) Unsubscribe(trigger string, service string, recipient string) {
	a.iterate(func(t string, s string, r []string, k string) {
		if trigger != t || s != service {
			return
		}
		for i := range r {
			if r[i] == recipient {
				updatedRecipients := append(r[:i], r[i+1:]...)
				if len(updatedRecipients) > 0 {
					a[k] = strings.Join(updatedRecipients, "")
				} else {
					delete(a, k)
				}
				break
			}
		}
	})
}

func (a Annotations) Has(service string, recipient string) bool {
	has := false
	a.iterate(func(t string, s string, r []string, k string) {
		if s != service {
			return
		}
		for i := range r {
			if r[i] == recipient {
				has = true
				break
			}
		}
	})
	return has
}

func (a Annotations) GetDestinations(defaultTriggers []string, serviceDefaultTriggers map[string][]string) services.Destinations {
	dests := services.Destinations{}
	a.iterate(func(trigger string, service string, recipients []string, v string) {
		for _, recipient := range recipients {
			triggers := defaultTriggers
			if trigger != "" {
				triggers = []string{trigger}
			} else if t, ok := serviceDefaultTriggers[service]; ok {
				triggers = t
			}

			for i := range triggers {
				dests[triggers[i]] = append(dests[triggers[i]], services.Destination{
					Service:   service,
					Recipient: recipient,
				})
			}
		}
	})
	return dests
}
