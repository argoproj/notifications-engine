package templates

import (
	"fmt"
	texttemplate "text/template"

	"github.com/Masterminds/sprig/v3"

	"github.com/argoproj/notifications-engine/pkg/services"
)

type Service interface {
	FormatNotification(vars map[string]any, extraFuncs texttemplate.FuncMap, templates ...string) (*services.Notification, error)
}

type service struct {
	templates map[string]services.Notification
	baseFuncs texttemplate.FuncMap
}

func NewService(templates map[string]services.Notification) (*service, error) {
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")
	return &service{templates: templates, baseFuncs: f}, nil
}

func (s *service) FormatNotification(vars map[string]any, extraFuncs texttemplate.FuncMap, templates ...string) (*services.Notification, error) {
	f := s.baseFuncs
	if len(extraFuncs) > 0 {
		merged := make(texttemplate.FuncMap, len(s.baseFuncs)+len(extraFuncs))
		for k, v := range s.baseFuncs {
			merged[k] = v
		}
		for k, v := range extraFuncs {
			merged[k] = v
		}
		f = merged
	}

	var notification services.Notification
	for _, templateName := range templates {
		cfg, ok := s.templates[templateName]
		if !ok {
			return nil, fmt.Errorf("template '%s' is not supported", templateName)
		}
		templater, err := cfg.GetTemplater(templateName, f)
		if err != nil {
			return nil, err
		}
		if err := templater(&notification, vars); err != nil {
			return nil, err
		}
	}

	return &notification, nil
}
