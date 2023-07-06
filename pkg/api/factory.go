package api

import (
	"sync"

	"k8s.io/utils/strings/slices"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	// Default namespace for ConfigMap and Secret.
	// For self-service notification, we get notification configurations from rollout resource namespace
	// and also the default namespace
	Namespace string
}

// Factory creates an API instance
type Factory interface {
	GetAPI() (API, error)
	GetAPIsWithNamespace(namespace string) (map[string]API, error)
}

type apiFactory struct {
	Settings

	cmLister     v1listers.ConfigMapLister
	secretLister v1listers.SecretLister
	lock         sync.Mutex
	apiMap       map[string]API
}

// NewFactory creates a new API factory if namespace is not empty, it will override the default namespace set in settings
func NewFactory(settings Settings, namespace string, secretsInformer cache.SharedIndexInformer, cmInformer cache.SharedIndexInformer) *apiFactory {
	if namespace != "" {
		settings.Namespace = namespace
	}

	factory := &apiFactory{
		Settings:     settings,
		cmLister:     v1listers.NewConfigMapLister(cmInformer.GetIndexer()),
		secretLister: v1listers.NewSecretLister(secretsInformer.GetIndexer()),
		apiMap:       make(map[string]API),
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
		f.lock.Lock()
		defer f.lock.Unlock()
		f.apiMap[metaObj.GetNamespace()] = nil
	}
}

func (f *apiFactory) getConfigMapAndSecretWithListers(cmLister v1listers.ConfigMapNamespaceLister, secretLister v1listers.SecretNamespaceLister) (*v1.ConfigMap, *v1.Secret, error) {
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

	if errors.IsNotFound(err) {
		return cm, secret, nil
	}
	return cm, secret, err
}

func (f *apiFactory) getConfigMapAndSecret(namespace string) (*v1.ConfigMap, *v1.Secret, error) {
	cmLister := f.cmLister.ConfigMaps(namespace)
	secretLister := f.secretLister.Secrets(namespace)

	return f.getConfigMapAndSecretWithListers(cmLister, secretLister)
}

func (f *apiFactory) GetAPI() (API, error) {
	apis, err := f.GetAPIsWithNamespace(f.Settings.Namespace)
	if err != nil {
		return nil, err
	}
	return apis[f.Settings.Namespace], nil
}

// For self-service notification, we need a map of apis which include api in the namespace and api in the setting's namespace
func (f *apiFactory) GetAPIsWithNamespace(namespace string) (map[string]API, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	apis := make(map[string]API)

	// namespaces to look for notification configurations
	namespaces := []string{namespace}
	if !slices.Contains(namespaces, f.Settings.Namespace) {
		namespaces = append(namespaces, f.Settings.Namespace)
	}

	for _, namespace := range namespaces {
		if f.apiMap[namespace] == nil {
			api, err := f.getApiFromNamespace(namespace)
			if err != nil {
				return nil, err
			}
			f.apiMap[namespace] = api
			apis[namespace] = f.apiMap[namespace]
		} else {
			apis[namespace] = f.apiMap[namespace]
		}
	}

	return apis, nil
}

func (f *apiFactory) getApiFromNamespace(namespace string) (API, error) {
	cm, secret, err := f.getConfigMapAndSecret(namespace)
	if err != nil {
		return nil, err
	}
	return f.getApiFromConfigmapAndSecret(cm, secret)

}

func (f *apiFactory) getApiFromConfigmapAndSecret(cm *v1.ConfigMap, secret *v1.Secret) (API, error) {
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
