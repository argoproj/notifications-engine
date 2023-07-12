package api

import (
	log "github.com/sirupsen/logrus"
	"sync"

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
}

// For self-service notification, factory creates a map of APIs that include
// api in the namespace specified in input parameter
// and api in the namespace specified in the Settings
type FactoryWithMultipleAPIs interface {
	GetAPIsWithNamespaceV2(namespace string) (map[string]API, error)
}

type apiFactory struct {
	Settings

	cmLister     v1listers.ConfigMapNamespaceLister
	secretLister v1listers.SecretNamespaceLister
	lock         sync.Mutex
	api          API

	// For self-service notification
	cmInformer      cache.SharedIndexInformer
	secretsInformer cache.SharedIndexInformer
	apiMap          map[string]API
	cacheList       []apisCache
}

type apisCache struct {
	api       API
	namespace string
	refresh   bool
}

func NewFactory(settings Settings, namespace string, secretsInformer cache.SharedIndexInformer, cmInformer cache.SharedIndexInformer) *apiFactory {
	factory := &apiFactory{
		Settings:     settings,
		cmLister:     v1listers.NewConfigMapLister(cmInformer.GetIndexer()).ConfigMaps(namespace),
		secretLister: v1listers.NewSecretLister(secretsInformer.GetIndexer()).Secrets(namespace),

		// For self-service notification
		cmInformer:      cmInformer,
		secretsInformer: secretsInformer,
		apiMap:          make(map[string]API),
		cacheList:       []apisCache{},
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
		f.invalidateCache(metaObj.GetNamespace())
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

	return cm, secret, err
}

func (f *apiFactory) getConfigMapAndSecret(namespace string) (*v1.ConfigMap, *v1.Secret, error) {
	cmLister := v1listers.NewConfigMapLister(f.cmInformer.GetIndexer()).ConfigMaps(namespace)
	secretLister := v1listers.NewSecretLister(f.secretsInformer.GetIndexer()).Secrets(namespace)

	return f.getConfigMapAndSecretWithListers(cmLister, secretLister)
}

func (f *apiFactory) invalidateCache(namespace string) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.api = nil

	f.apiMap[namespace] = nil

	for _, mycache := range f.cacheList {
		if mycache.namespace == namespace {
			mycache.refresh = true
			mycache.api = nil
		}
	}
}

func (f *apiFactory) GetAPI() (API, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if f.api == nil {
		cm, secret, err := f.getConfigMapAndSecretWithListers(f.cmLister, f.secretLister)
		if err != nil {
			return nil, err
		}

		api, err := f.getApiFromConfigmapAndSecret(cm, secret)
		if err != nil {
			return nil, err
		}
		f.api = api
	}
	return f.api, nil
}

// For self-service notification, we need a map of apis which include api in the namespace and api in the setting's namespace
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
		} else {
			log.Warnf("getApiFromNamespace %s got error %s", f.Settings.Namespace, err)
		}
		return apis, nil
	}

	if f.apiMap[f.Settings.Namespace] != nil {
		apis[f.Settings.Namespace] = f.apiMap[f.Settings.Namespace]
		api, err := f.getApiFromNamespace(namespace)
		if err == nil {
			apis[namespace] = api
			f.apiMap[namespace] = api
		} else {
			log.Warnf("getApiFromNamespace %s got error %s", namespace, err)
		}
		return apis, nil
	}

	apiFromNamespace, errApiFromNamespace := f.getApiFromNamespace(namespace)
	apiFromSettings, errApiFromSettings := f.getApiFromNamespace(f.Settings.Namespace)

	if errApiFromNamespace == nil {
		apis[namespace] = apiFromNamespace
		f.apiMap[namespace] = apiFromNamespace
	} else {
		log.Warnf("getApiFromNamespace %s got error %s", namespace, errApiFromNamespace)
	}

	if errApiFromSettings == nil {
		apis[f.Settings.Namespace] = apiFromSettings
		f.apiMap[f.Settings.Namespace] = apiFromSettings
	} else {
		log.Warnf("getApiFromNamespace %s got error %s", f.Settings.Namespace, errApiFromSettings)
	}

	// Only return error when we received error from both namespace provided in the input paremeter and settings' namespace
	if errApiFromNamespace != nil && errApiFromSettings != nil {
		return apis, errApiFromSettings
	} else {
		return apis, nil
	}
}

// For self-service notification, we need a map of apis which include api in the namespace and api in the setting's namespace
func (f *apiFactory) GetAPIsWithNamespaceV2(namespace string) (map[string]API, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	apis := make(map[string]API)

	// namespaces to look for notification configurations
	namespaces := []string{namespace}
	if f.Settings.Namespace != "" && f.Settings.Namespace != namespace {
		namespaces = append(namespaces, f.Settings.Namespace)
	}

	for _, namespace := range namespaces {
		//Look up the cacheList
		//Exist in cacheList and does not need refresh, then use it
		//Exist in cacheList and needs refresh, then retrieve it
		//Doesn't exist in cacheList, get it and put in cacheList
		foundInCache := false
		for _, cache := range f.cacheList {
			if cache.namespace == namespace {
				foundInCache = true
				if !cache.refresh {
					//Found in cache, and no need to refresh
					if cache.api != nil {
						apis[namespace] = cache.api
					}
				} else {
					//Found in cache, and need refresh
					api, err := f.getApiFromNamespace(namespace)
					if err == nil {
						apis[namespace] = api
						cache.api = api
						cache.refresh = false
					} else {
						log.Warnf("getApiFromNamespace %s got error %s", namespace, err)
					}
				}
				break
			}
		}

		if !foundInCache {
			api, err := f.getApiFromNamespace(namespace)
			if err == nil {
				apis[namespace] = api
				myCache := apisCache{refresh: false, api: api, namespace: namespace}
				f.cacheList = append(f.cacheList, myCache)
			} else {
				log.Warnf("getApiFromNamespace %s got error %s", namespace, err)
			}
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
