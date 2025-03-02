package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ListPods returns a handler for GET /pods endpoint
func (h *ClientHandler) ListPods() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := log.FromContext(r.Context()).WithName("list-pods")

		// Check if the request method is GET
		if r.Method != http.MethodGet {
			err := fmt.Errorf("invalid method: %s, allowed: %s", r.Method, http.MethodGet)
			logger.Error(err, "Method not allowed")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Get namespace from query parameter, default to default namespace
		namespace := r.URL.Query().Get("namespace")
		if namespace == "" {
			namespace = "default"
			logger.V(1).Info("No namespace provided, using default")
		}

		logger = logger.WithValues("namespace", namespace)
		logger.Info("Listing pods")

		// List pods in the namespace
		var podList corev1.PodList
		if err := h.Client.List(r.Context(), &podList, &client.ListOptions{
			Namespace: namespace,
		}); err != nil {
			logger.Error(err, "Failed to list pods")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Log pod count and details
		logger.Info("Successfully listed pods",
			"count", len(podList.Items),
		)

		// Log detailed pod information at debug level
		for _, pod := range podList.Items {
			logger.V(1).Info("Pod details",
				"name", pod.Name,
				"status", pod.Status.Phase,
				"containers", len(pod.Spec.Containers),
			)
		}

		// Set response headers
		w.Header().Set("Content-Type", "application/json")

		// Write response
		if err := json.NewEncoder(w).Encode(podList); err != nil {
			logger.Error(err, "Failed to encode response")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		logger.V(1).Info("Response sent successfully")
	}
}
