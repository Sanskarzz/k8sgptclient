package handlers

import (
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ListPods returns a handler for GET /pods endpoint
func (h *ClientHandler) ListPods() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if the request method is GET
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Get namespace from query parameter, default to default namespace
		namespace := r.URL.Query().Get("namespace")
		if namespace == "" {
			namespace = "default"
		}

		// List pods in the namespace
		var podList corev1.PodList
		if err := h.Client.List(r.Context(), &podList, &client.ListOptions{
			Namespace: namespace,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set response headers
		w.Header().Set("Content-Type", "application/json")

		// Write response
		if err := json.NewEncoder(w).Encode(podList); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
