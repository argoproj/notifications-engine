package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/argoproj/notifications-engine/pkg"
	"github.com/argoproj/notifications-engine/pkg/services"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Config holds tool command settings
type Config struct {
	// Resource holds group version and name of a Kubernetes resource that produces notifications
	Resource schema.GroupVersionResource
	// CLIName holds command line binary name
	CLIName string
	// ConfigMapName holds Kubernetes ConfigName name that contains notifications settings
	ConfigMapName string
	// SecretName holds Kubernetes Secret name that contains sensitive information
	SecretName string
	// CreateVars is a function that produces notifications context variables
	CreateVars func(obj map[string]interface{}, dest services.Destination, cmdContext CommandContext) (map[string]interface{}, error)
}

// CommandContext encapsulates access to Kubernetes resources and implements config parsing
type CommandContext interface {
	GetK8SClients() (kubernetes.Interface, dynamic.Interface, string, error)
	GetConfig() (*pkg.Config, error)
	GetSecret() (*v1.Secret, error)
	GetConfigMap() (*v1.ConfigMap, error)
}

type commandContext struct {
	Config
	configMapPath string
	secretPath    string
	stdout        io.Writer
	stdin         io.Reader
	stderr        io.Writer
	getK8SClients func() (kubernetes.Interface, dynamic.Interface, string, error)
}

func getK8SClients(clientConfig clientcmd.ClientConfig) (kubernetes.Interface, dynamic.Interface, string, error) {
	ns, _, err := clientConfig.Namespace()
	if err != nil {
		return nil, nil, "", err
	}
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, "", err
	}
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, "", err
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, nil, "", err
	}
	return k8sClient, dynamicClient, ns, nil
}

func splitYAML(yamlData []byte) ([]*unstructured.Unstructured, error) {
	d := kubeyaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlData), 4096)
	var objs []*unstructured.Unstructured
	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if err == io.EOF {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %v", err)
		}
		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}
		u := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(ext.Raw, u); err != nil {
			return objs, fmt.Errorf("failed to unmarshal manifest: %v", err)
		}
		objs = append(objs, u)
	}
	return objs, nil
}

func (c *commandContext) GetK8SClients() (kubernetes.Interface, dynamic.Interface, string, error) {
	return c.getK8SClients()
}

func (c *commandContext) unmarshalFromFile(filePath string, name string, gk schema.GroupKind, result interface{}) error {
	var err error
	var data []byte
	if filePath == "-" {
		data, err = ioutil.ReadAll(c.stdin)
	} else {
		data, err = ioutil.ReadFile(c.configMapPath)
	}
	if err != nil {
		return err
	}
	objs, err := splitYAML(data)
	if err != nil {
		return err
	}

	for _, obj := range objs {
		if obj.GetName() == name && obj.GroupVersionKind().GroupKind() == gk {
			return runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, result)
		}
	}
	return fmt.Errorf("file '%s' does not have '%s/%s/%s'", filePath, gk.Group, gk.Kind, name)
}

func (c *commandContext) GetConfig() (*pkg.Config, error) {
	configMap, err := c.GetConfigMap()
	if err != nil {
		return nil, err
	}

	secret, err := c.GetSecret()
	if err != nil {
		return nil, err
	}
	return pkg.ParseConfig(configMap, secret)
}

func (c *commandContext) GetSecret() (*v1.Secret, error) {
	var secret v1.Secret
	if c.secretPath == ":empty" {
		secret = v1.Secret{}
	} else if c.secretPath == "" {
		k8sClient, _, ns, err := c.getK8SClients()
		if err != nil {
			return nil, err
		}
		s, err := k8sClient.CoreV1().Secrets(ns).Get(context.Background(), c.SecretName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		secret = *s
	} else {
		if err := c.unmarshalFromFile(c.secretPath, c.SecretName, schema.GroupKind{Kind: "Secret"}, &secret); err != nil {
			return nil, err
		}
	}
	return &secret, nil
}

func (c *commandContext) GetConfigMap() (*v1.ConfigMap, error) {
	var configMap v1.ConfigMap
	if c.configMapPath == "" {
		k8sClient, _, ns, err := c.getK8SClients()
		if err != nil {
			return nil, err
		}
		cm, err := k8sClient.CoreV1().ConfigMaps(ns).Get(context.Background(), c.ConfigMapName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		configMap = *cm
	} else {
		if err := c.unmarshalFromFile(c.configMapPath, c.ConfigMapName, schema.GroupKind{Kind: "ConfigMap"}, &configMap); err != nil {
			return nil, err
		}
	}
	return &configMap, nil
}

func (c *commandContext) loadResource(name string) (*unstructured.Unstructured, error) {
	if ext := filepath.Ext(name); ext != "" {
		data, err := ioutil.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var app unstructured.Unstructured
		err = yaml.Unmarshal(data, &app)
		return &app, err
	}
	_, client, ns, err := c.getK8SClients()
	if err != nil {
		return nil, err
	}
	res, err := client.Resource(c.Resource).Namespace(ns).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *commandContext) createVars(obj map[string]interface{}, dest services.Destination) map[string]interface{} {
	vars, err := c.CreateVars(obj, dest, c)
	if err != nil {
		log.Fatalf("Failed to create variables: %v", err)
		return nil
	}
	return vars
}
