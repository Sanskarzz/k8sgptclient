package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type DeploymentPods struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	PodNames  []string `json:"podNames"`
}

// DeploymentPodNames returns a handler for GET /deployments/{namespace}/{deploymentName}/pods endpoint
func (h *ClientHandler) DeploymentPodNames() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := log.FromContext(r.Context()).WithName("deployment-pods")

		// Parse path parameters
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) != 5 {
			err := fmt.Errorf("invalid path: %s, expected: /deployments/{namespace}/{deploymentName}/pods", r.URL.Path)
			logger.Error(err, "Invalid path")
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		namespace := parts[2]
		deploymentName := parts[3]

		logger = logger.WithValues(
			"namespace", namespace,
			"deployment", deploymentName,
		)
		logger.Info("Getting deployment pod names")

		// Get deployment
		var deployment appsv1.Deployment
		if err := h.Client.Get(r.Context(), types.NamespacedName{
			Namespace: namespace,
			Name:      deploymentName,
		}, &deployment); err != nil {
			logger.Error(err, "Failed to get deployment")
			http.Error(w, fmt.Sprintf("Failed to get deployment: %v", err), http.StatusInternalServerError)
			return
		}

		// Get pods for this deployment
		var podList corev1.PodList
		labelSelector := labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)
		if err := h.Client.List(r.Context(), &podList, &client.ListOptions{
			Namespace:     namespace,
			LabelSelector: labelSelector,
		}); err != nil {
			logger.Error(err, "Failed to list pods")
			http.Error(w, fmt.Sprintf("Failed to list pods: %v", err), http.StatusInternalServerError)
			return
		}

		// Create response
		response := DeploymentPods{
			Name:      deploymentName,
			Namespace: namespace,
			PodNames:  make([]string, 0, len(podList.Items)),
		}

		// Add pod names
		for _, pod := range podList.Items {
			response.PodNames = append(response.PodNames, pod.Name)
		}

		// Set response headers
		w.Header().Set("Content-Type", "application/json")

		// Write response
		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Error(err, "Failed to encode response")
			http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
			return
		}

		logger.V(1).Info("Response sent successfully")
	}
}
