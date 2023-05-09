package api

import (
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// Settings holds a set of settings required for API creation
type Settings struct {
	// ConfigMapName holds Kubernetes ConfigName name that contains notifications settings
	ConfigMapName string
	// SecretName holds Kubernetes Secret name that contains sensitive information
	SecretName string
	// InitGetVars returns a function that produces notifications context variables
	InitGetVars func(cfg *Config, configMap *v1.ConfigMap, secret *v1.Secret) (GetVars, error)
	// Default namespace for ConfigMap and Secret
	Namespace string
}

// Factory creates an API instance
type Factory interface {
	GetAPI() (API, error)
}

// Factory creates an API instance
type MayFactory interface {
	GetAPIWithNamespace(namespace string) (API, error)
}

// Factory creates an API instance
type MayFactoryWithMultipleAPIs interface {
	GetAPIsWithNamespace(namespace string) (map[string]API, error)
}

type apiFactory struct {
	Settings

	cmLister     v1listers.ConfigMapNamespaceLister
	secretLister v1listers.SecretNamespaceLister
	lock         sync.Mutex
	api          API

	cmInformer      cache.SharedIndexInformer
	secretsInformer cache.SharedIndexInformer
	apiMap          map[string]API
}

func NewFactory(settings Settings, namespace string, secretsInformer cache.SharedIndexInformer, cmInformer cache.SharedIndexInformer) *apiFactory {
	factory := &apiFactory{
		Settings:        settings,
		cmLister:        v1listers.NewConfigMapLister(cmInformer.GetIndexer()).ConfigMaps(namespace),
		secretLister:    v1listers.NewSecretLister(secretsInformer.GetIndexer()).Secrets(namespace),
		cmInformer:      cmInformer,
		secretsInformer: secretsInformer,
		apiMap:          make(map[string]API),
	}

	secretsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			factory.invalidateIfHasName(settings.SecretName, obj)
		},
		DeleteFunc: func(obj interface{}) {
			factory.invalidateIfHasName(settings.SecretName, obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			factory.invalidateIfHasName(settings.SecretName, newObj)
		}})
	cmInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			factory.invalidateIfHasName(settings.ConfigMapName, obj)
		},
		DeleteFunc: func(obj interface{}) {
			factory.invalidateIfHasName(settings.ConfigMapName, obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			factory.invalidateIfHasName(settings.ConfigMapName, newObj)
		}})
	return factory
}

func (f *apiFactory) invalidateIfHasName(name string, obj interface{}) {
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return
	}
	if metaObj.GetName() == name {
		f.invalidateCache()
	}
}

func (f *apiFactory) getConfigMapAndSecret() (*v1.ConfigMap, *v1.Secret, error) {
	cm, err := f.cmLister.Get(f.ConfigMapName)
	if err != nil {
		if errors.IsNotFound(err) {
			cm = &v1.ConfigMap{}
		} else {
			return nil, nil, err
		}
	}

	secret, err := f.secretLister.Get(f.SecretName)
	if err != nil {
		if errors.IsNotFound(err) {
			secret = &v1.Secret{}
		} else {
			return nil, nil, err
		}
	}

	return cm, secret, err
}

func (f *apiFactory) getConfigMapAndSecretWithNamespace(namespace string) (*v1.ConfigMap, *v1.Secret, error) {

	cmLister := v1listers.NewConfigMapLister(f.cmInformer.GetIndexer()).ConfigMaps(namespace)
	secretLister := v1listers.NewSecretLister(f.secretsInformer.GetIndexer()).Secrets(namespace)

	cm, err := cmLister.Get(f.ConfigMapName)
	if err != nil {
		if errors.IsNotFound(err) {
			cm = &v1.ConfigMap{}
		} else {
			return nil, nil, err
		}
	}

	secret, err := secretLister.Get(f.SecretName)
	if err != nil {
		if errors.IsNotFound(err) {
			secret = &v1.Secret{}
		} else {
			return nil, nil, err
		}
	}

	return cm, secret, err
}

func (f *apiFactory) invalidateCache() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.api = nil
	for namespace := range f.apiMap {
		f.apiMap[namespace] = nil
	}
}

func (f *apiFactory) GetAPI() (API, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if f.api == nil {
		cm, secret, err := f.getConfigMapAndSecret()
		if err != nil {
			return nil, err
		}
		cfg, err := ParseConfig(cm, secret)
		if err != nil {
			return nil, err
		}
		getVars, err := f.InitGetVars(cfg, cm, secret)
		if err != nil {
			return nil, err
		}
		api, err := NewAPI(*cfg, getVars)
		if err != nil {
			return nil, err
		}
		f.api = api
	}
	return f.api, nil
}

func (f *apiFactory) GetAPIWithNamespace(namespace string) (API, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	namespaceHasConfig := namespace

	if f.apiMap[namespaceHasConfig] != nil {
		return f.apiMap[namespaceHasConfig], nil
	}

	cm, secret, err := f.getConfigMapAndSecretWithNamespace(namespaceHasConfig)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, err
		}
		// If could not find it in namespace, try the namespace from settings
		namespaceHasConfig = f.Settings.Namespace
		if f.apiMap[namespaceHasConfig] != nil {
			return f.apiMap[namespaceHasConfig], nil
		}
		cm, secret, err = f.getConfigMapAndSecretWithNamespace(namespaceHasConfig)
		if err != nil {
			return nil, err
		}
	}
	cfg, err := ParseConfig(cm, secret)
	if err != nil {
		return nil, err
	}
	getVars, err := f.InitGetVars(cfg, cm, secret)
	if err != nil {
		return nil, err
	}
	api, err := NewAPI(*cfg, getVars)
	if err != nil {
		return nil, err
	}
	f.apiMap[namespaceHasConfig] = api

	return f.apiMap[namespaceHasConfig], nil
}

// Returns a map of api in the namespace and api in the setting's namespace
func (f *apiFactory) GetAPIsWithNamespace(namespace string) (map[string]API, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	apis := make(map[string]API)

	if f.apiMap[namespace] != nil && f.apiMap[f.Settings.Namespace] != nil {
		apis[namespace] = f.apiMap[namespace]
		apis[f.Settings.Namespace] = f.apiMap[f.Settings.Namespace]
		return apis, nil
	}

	if f.apiMap[namespace] != nil {
		apis[namespace] = f.apiMap[namespace]
		api, err := f.getApiFromNamespace(f.Settings.Namespace)
		if err == nil {
			apis[f.Settings.Namespace] = api
			f.apiMap[f.Settings.Namespace] = api
		}
		return apis, nil
	}

	if f.apiMap[f.Settings.Namespace] != nil {
		apis[f.Settings.Namespace] = f.apiMap[f.Settings.Namespace]
		api, err := f.getApiFromNamespace(namespace)
		if err == nil {
			apis[namespace] = api
			f.apiMap[namespace] = api
		}

		return apis, nil
	}

	//Where is nothing in cache, then we retrieve them
	apiFromNamespace, err := f.getApiFromNamespace(namespace)
	if err == nil {
		apis[namespace] = apiFromNamespace
		f.apiMap[namespace] = apiFromNamespace
	}
	apiFromSettings, err := f.getApiFromNamespace(f.Settings.Namespace)
	if err == nil {
		apis[f.Settings.Namespace] = apiFromSettings
		f.apiMap[f.Settings.Namespace] = apiFromSettings
	}

	return apis, nil

}

func (f *apiFactory) getApiFromNamespace(namespace string) (API, error) {
	cm, secret, err := f.getConfigMapAndSecretWithNamespace(namespace)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, err
		}
	}
	cfg, err := ParseConfig(cm, secret)
	if err != nil {
		return nil, err
	}
	getVars, err := f.InitGetVars(cfg, cm, secret)
	if err != nil {
		return nil, err
	}
	api, err := NewAPI(*cfg, getVars)
	if err != nil {
		return nil, err
	}
	return api, nil
}
