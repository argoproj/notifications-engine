package subscriptions

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/notifications-engine/pkg/services"
)

var (
	annotationPrefix = "notifications.argoproj.io"
)

// SetAnnotationPrefix sets the annotationPrefix to the provided string.
// defaults to "notifications.argoproj.io"
func SetAnnotationPrefix(prefix string) {
	annotationPrefix = prefix
}

func NotifiedAnnotationKey() string {
	return fmt.Sprintf("notified.%s", annotationPrefix)
}

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
	return fmt.Sprintf("%s/subscribe.%s.%s", annotationPrefix, trigger, service)
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
	prefix := annotationPrefix + "/subscribe."
	altPrefix := annotationPrefix + "/subscriptions"
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
			var source []byte
			if v != "" {
				source = []byte(v)
			} else {
				log.Errorf("Subscription is not defined")
				callback("", "", recipients, k)
			}
			err := yaml.Unmarshal(source, &subscriptions)
			if err != nil {
				log.Errorf("Notification subscription unmarshal error: %v", err)
				callback("", "", recipients, k)
			}
			for _, v := range subscriptions {
				triggers := v.Trigger
				destinations := v.Destinations
				if len(triggers) == 0 && len(destinations) == 0 {
					trigger := ""
					destination := ""
					recipients = []string{}
					log.Printf("Notification triggers and destinations are not configured")
					callback(trigger, destination, recipients, k)
				} else if len(triggers) == 0 && len(destinations) != 0 {
					trigger := ""
					log.Printf("Notification triggers are not configured")
					for _, destination := range destinations {
						log.Printf("trigger: %v, service: %v, recipient: %v \n", trigger, destination.Service, destination.Recipients)
						callback(trigger, destination.Service, destination.Recipients, k)
					}
				} else if len(triggers) != 0 && len(destinations) == 0 {
					service := ""
					recipients = []string{}
					log.Printf("Notification destinations are not configured")
					for _, trigger := range triggers {
						log.Printf("trigger: %v, service: %v, recipient: %v \n", trigger, service, recipients)
						callback(trigger, service, recipients, k)
					}
				} else {
					for _, trigger := range triggers {
						for _, destination := range destinations {
							log.Printf("Notification trigger: %v, service: %v, recipient: %v \n", trigger, destination.Service, destination.Recipients)
							callback(trigger, destination.Service, destination.Recipients, k)
						}
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
