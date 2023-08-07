package mocks

import "github.com/argoproj/notifications-engine/pkg/api"

type FakeFactory struct {
	Api api.API
	Err error
}

func (f *FakeFactory) GetAPI() (api.API, error) {
	return f.Api, f.Err
}

func (f *FakeFactory) GetAPIsFromNamespace(namespace string) (map[string]api.API, error) {
	apiMap := make(map[string]api.API)
	apiMap[namespace] = f.Api
	return apiMap, f.Err
}

func (f *FakeFactory) GetAPIsFromFactory(resource interface{}) (map[string]api.API, error) {
	return nil, f.Err
}
