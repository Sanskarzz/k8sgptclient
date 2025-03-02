package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ApplyResponse represents the response structure for apply operation
type ApplyResponse struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Action    string `json:"action"` // "created" or "updated"
}

func (h *ClientHandler) Apply() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logger := log.FromContext(r.Context()).WithName("apply")

		if r.Method != http.MethodPost {
			err := fmt.Errorf("invalid method: %s, allowed: %s", r.Method, http.MethodGet)
			logger.Error(err, "Method not allowed")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Read the YAML content
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error(err, "Failed to read request body")
			http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
			return
		}

		// Log YAML content at debug level
		logger.V(2).Info("Received YAML content", "yaml", string(body))

		// Decode YAML to unstructured object
		logger.V(1).Info("Decoding YAML content")
		decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		obj := &unstructured.Unstructured{}
		_, gvk, err := decoder.Decode(body, nil, obj)
		if err != nil {
			logger.Error(err, "Failed to decode YAML")
			http.Error(w, fmt.Sprintf("Failed to decode YAML: %v", err), http.StatusBadRequest)
			return
		}

		// Set the GVK
		obj.SetGroupVersionKind(*gvk)
		logger = logger.WithValues(
			"kind", obj.GetKind(),
			"apiVersion", obj.GetAPIVersion(),
		)

		// Get object metadata
		metadata, err := meta.Accessor(obj)
		if err != nil {
			logger.Error(err, "Failed to get object metadata")
			http.Error(w, fmt.Sprintf("Failed to get object metadata: %v", err), http.StatusBadRequest)
			return
		}

		logger = logger.WithValues(
			"name", metadata.GetName(),
			"namespace", metadata.GetNamespace(),
		)

		// If namespace is not set, set it to default
		if metadata.GetNamespace() == "" {
			obj.SetNamespace("default")
		}

		logger.Info("Applying resource")

		// Set server-side apply field manager
		if err := h.Client.Patch(r.Context(), obj, client.Apply, &client.PatchOptions{
			FieldManager: "k8sgptclient",
			Force:        &[]bool{true}[0],
		}); err != nil {
			logger.Error(err, "Failed to apply resource")
			http.Error(w, fmt.Sprintf("Failed to apply resource: %v", err), http.StatusInternalServerError)
			return
		}

		logger.Info("Successfully applied resource")

		// Prepare response
		response := ApplyResponse{
			Kind:      obj.GetKind(),
			Name:      metadata.GetName(),
			Namespace: metadata.GetNamespace(),
			Action:    "applied",
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
