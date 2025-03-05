package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

func (h *ClientHandler) PodYaml() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := log.FromContext(r.Context()).WithName("get-pod")
		logger.Info("Getting pod")

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

		namespace := parts[2]
		name := parts[3]

		logger.Info("Getting pod", "namespace", namespace, "name", name)
		if namespace == "" {
			namespace = "default"
			logger.V(1).Info("No namespace provided, using default")
		}

		// Get pod
		pod := &corev1.Pod{}
		err := h.Client.Get(r.Context(), client.ObjectKey{
			Namespace: namespace,
			Name:      name,
		}, pod)
		if err != nil {
			logger.Error(err, "Failed to get pod")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create simplified pod
		simplifiedPod := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			},
			Spec: pod.Spec,
		}

		// Convert to YAML
		jsonData, err := json.Marshal(simplifiedPod)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal pod: %v", err), http.StatusInternalServerError)
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
