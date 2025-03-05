package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

func (h *ClientHandler) DeploymentYaml() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract namespace and deployment name from query parameters
		logger := log.FromContext(r.Context()).WithName("get-deployment")
		logger.Info("Getting deployment")

		if r.Method != http.MethodGet {
			err := fmt.Errorf("invalid method: %s, allowed: %s", r.Method, http.MethodGet)
			logger.Error(err, "Method not allowed")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Extract path parameters
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) != 5 {
			http.Error(w, "invalid path format", http.StatusBadRequest)
			return
		}

		// Extract namespace and deployment name from query parameters
		namespace := parts[2]
		name := parts[3]

		logger.Info("Getting deployment", "namespace", namespace, "name", name)
		if namespace == "" {
			namespace = "default"
			logger.V(1).Info("No namespace provided, using default")
		}

		// Get deployment
		deployment := &appsv1.Deployment{}
		err := h.Client.Get(r.Context(), client.ObjectKey{
			Namespace: namespace,
			Name:      name,
		}, deployment)
		if err != nil {
			logger.Error(err, "Failed to get deployment")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create simplified deployment
		simplifiedDeployment := &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      deployment.Name,
				Namespace: deployment.Namespace,
			},
			Spec: deployment.Spec,
		}

		// Convert to YAML
		jsonData, err := json.Marshal(simplifiedDeployment)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal deployment: %v", err), http.StatusInternalServerError)
			return
		}

		yamlData, err := yaml.JSONToYAML(jsonData)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to convert to yaml: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/yaml")
		w.Write(yamlData)
	}
}
