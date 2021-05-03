package mocks

import "github.com/argoproj/notifications-engine/pkg/api"

type FakeFactory struct {
	Api api.API
}

func (f *FakeFactory) GetAPI() (api.API, error) {
	return f.Api, nil
}
