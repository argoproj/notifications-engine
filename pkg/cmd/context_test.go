package cmd

import (
	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

var secretYaml = `apiVersion: v1
kind: Secret
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"v1","kind":"Secret","metadata":{"annotations":{},"name":"argocd-notifications-secret","namespace":"argocd"},"type":"Opaque"}
  creationTimestamp: "2021-11-09T12:43:49Z"
  name: argocd-notifications-secret
  namespace: argocd
  resourceVersion: "38019672"
  selfLink: /api/v1/namespaces/argocd/secrets/argocd-notifications-secret
  uid: d41860ec-1a35-46e2-b194-093529554df5
type: Opaque`

func Test_getSecretFromFile(t *testing.T) {
	file, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = os.Remove(file.Name())
	}()

	_, _ = file.WriteString(secretYaml)
	_ = file.Sync()

	if _, err := file.Seek(0, 0); err != nil {
		log.Fatal(err)
	}

	ctx := commandContext{
		secretPath: file.Name(),
		Settings: api.Settings{
			SecretName: "argocd-notifications-secret",
		},
	}

	secret, err := ctx.getSecret()
	assert.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.Equal(t, secret.Name, "argocd-notifications-secret")
}
