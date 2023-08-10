package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/lol3909/notifications-engine/pkg/services"
)

var (
	settings = Settings{ConfigMapName: "my-config-map", SecretName: "my-secret", InitGetVars: func(cfg *Config, configMap *v1.ConfigMap, secret *v1.Secret) (GetVars, error) {
		return func(obj map[string]interface{}, dest services.Destination) map[string]interface{} {
			return map[string]interface{}{"obj": obj}
		}, nil
	}}
)

func TestGetAPI(t *testing.T) {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-config-map", Namespace: "default"},
		Data: map[string]string{
			"service.slack": `{"token": "abc"}`,
		},
	}
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
	}

	clientset := fake.NewSimpleClientset(cm, secret)
	informerFactory := informers.NewSharedInformerFactory(clientset, time.Minute)

	secrets := informerFactory.Core().V1().Secrets().Informer()
	configMaps := informerFactory.Core().V1().ConfigMaps().Informer()
	factory := NewFactory(settings, "default", secrets, configMaps)

	go informerFactory.Start(context.Background().Done())
	if !cache.WaitForCacheSync(context.Background().Done(), configMaps.HasSynced, secrets.HasSynced) {
		assert.Fail(t, "failed to sync informers")
	}

	api, err := factory.GetAPI()
	require.NoError(t, err)

	svcs := api.GetNotificationServices()
	assert.Len(t, svcs, 1)
	assert.NotNil(t, svcs["slack"])

	_, err = clientset.CoreV1().ConfigMaps("default").Update(context.Background(), &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-config-map", Namespace: "default"},
		Data: map[string]string{
			"service.email": `{"username": "test"}`,
		},
	}, metav1.UpdateOptions{})
	assert.NoError(t, err)

	time.Sleep(1 * time.Second)

	api, err = factory.GetAPI()
	require.NoError(t, err)

	svcs = api.GetNotificationServices()
	assert.Len(t, svcs, 1)
	assert.NotNil(t, svcs["email"])
}
