package api

import (
	"fmt"

	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/argoproj/notifications-engine/pkg/templates"
	"github.com/argoproj/notifications-engine/pkg/triggers"
)

const (
	serviceTypeVarName = "serviceType"
	recipientVarName   = "recipient"
)

//go:generate mockgen -destination=../mocks/api.go -package=mocks github.com/argoproj/notifications-engine/pkg/api API

type GetVars func(obj map[string]any, dest services.Destination) map[string]any

// API provides high level interface to send notifications and manage notification services
type API interface {
	Send(obj map[string]any, templates []string, dest services.Destination) error
	// SendWithAnnotations behaves like Send but additionally returns any annotations that
	// the notification service wants persisted onto the target resource (e.g. Slack thread
	// state), so state survives controller restarts and leader-election failovers.
	SendWithAnnotations(obj map[string]any, templates []string, dest services.Destination) (map[string]string, error)
	RunTrigger(triggerName string, vars map[string]any) ([]triggers.ConditionResult, error)
	AddNotificationService(name string, service services.NotificationService)
	GetNotificationServices() map[string]services.NotificationService
	GetConfig() Config
}

type api struct {
	notificationServices map[string]services.NotificationService
	templatesService     templates.Service
	triggersService      triggers.Service
	getVars              GetVars
	config               Config
}

func (n *api) GetConfig() Config {
	return n.config
}

// AddService adds new service with the specified name
func (n *api) AddNotificationService(name string, service services.NotificationService) {
	n.notificationServices[name] = service
}

// GetServices returns map of registered services
func (n *api) GetNotificationServices() map[string]services.NotificationService {
	return n.notificationServices
}

// Send sends notification using specified service and template to the specified destination
func (n *api) Send(obj map[string]any, templates []string, dest services.Destination) error {
	_, err := n.SendWithAnnotations(obj, templates, dest)
	return err
}

// SendWithAnnotations behaves like Send but additionally returns any annotations that the
// notification service wants persisted onto the target resource.
func (n *api) SendWithAnnotations(obj map[string]any, templates []string, dest services.Destination) (map[string]string, error) {
	notificationService, ok := n.notificationServices[dest.Service]
	if !ok {
		return nil, fmt.Errorf("notification service '%s' is not supported", dest.Service)
	}

	vars := n.getVars(obj, dest)

	in := make(map[string]any)
	for k := range vars {
		in[k] = vars[k]
	}
	in[serviceTypeVarName] = dest.Service
	in[recipientVarName] = dest.Recipient
	notification, err := n.templatesService.FormatNotification(in, templates...)
	if err != nil {
		return nil, err
	}

	if annotating, ok := notificationService.(services.AnnotatingNotificationService); ok {
		return annotating.SendWithAnnotations(*notification, dest)
	}
	return nil, notificationService.Send(*notification, dest)
}

func (n *api) RunTrigger(triggerName string, obj map[string]any) ([]triggers.ConditionResult, error) {
	vars := n.getVars(obj, services.Destination{})
	return n.triggersService.Run(triggerName, vars)
}

// NewAPI creates new api instance using provided config
func NewAPI(cfg Config, getVars GetVars) (*api, error) {
	notificationServices := map[string]services.NotificationService{}
	for k, v := range cfg.Services {
		svc, err := v()
		if err != nil {
			return nil, err
		}
		notificationServices[k] = svc
	}
	triggersService, err := triggers.NewService(cfg.Triggers)
	if err != nil {
		return nil, err
	}
	templatesService, err := templates.NewService(cfg.Templates)
	if err != nil {
		return nil, err
	}

	return &api{notificationServices, templatesService, triggersService, getVars, cfg}, nil
}
