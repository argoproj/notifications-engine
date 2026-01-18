package main

import (
	"fmt"
	"os"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/cmd"
	"github.com/argoproj/notifications-engine/pkg/services"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	command := cmd.NewToolsCommand("cli", "cli", schema.GroupVersionResource{
		Group: "cert-manager.io", Version: "v1", Resource: "certificates",
	}, api.Settings{
		ConfigMapName: "cert-manager-notifications-cm",
		SecretName:    "cert-manager-notifications-secret",
		InitGetVars: func(_ *api.Config, _ *corev1.ConfigMap, _ *corev1.Secret) (api.GetVars, error) {
			return func(obj map[string]any, _ services.Destination) map[string]any {
				return map[string]any{"cert": obj}
			}, nil
		},
	})

	if err := command.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
