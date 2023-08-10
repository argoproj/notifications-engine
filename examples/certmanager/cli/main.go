package main

import (
	"fmt"
	"os"

	"github.com/lol3909/notifications-engine/pkg/api"
	"github.com/lol3909/notifications-engine/pkg/cmd"
	"github.com/lol3909/notifications-engine/pkg/services"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	command := cmd.NewToolsCommand("cli", "cli", schema.GroupVersionResource{
		Group: "cert-manager.io", Version: "v1", Resource: "certificates",
	}, api.Settings{
		ConfigMapName: "cert-manager-notifications-cm",
		SecretName:    "cert-manager-notifications-secret",
		InitGetVars: func(cfg *api.Config, configMap *v1.ConfigMap, secret *v1.Secret) (api.GetVars, error) {
			return func(obj map[string]interface{}, dest services.Destination) map[string]interface{} {
				return map[string]interface{}{"cert": obj}
			}, nil
		},
	})

	if err := command.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
