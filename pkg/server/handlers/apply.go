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
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Read the YAML content
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
			return
		}

		// Decode YAML to unstructured object
		decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		obj := &unstructured.Unstructured{}
		_, gvk, err := decoder.Decode(body, nil, obj)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to decode YAML: %v", err), http.StatusBadRequest)
			return
		}

		// Set the GVK
		obj.SetGroupVersionKind(*gvk)

		// Get object metadata
		metadata, err := meta.Accessor(obj)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get object metadata: %v", err), http.StatusBadRequest)
			return
		}

		// If namespace is not set, set it to default
		if metadata.GetNamespace() == "" {
			obj.SetNamespace("default")
		}

		// Set server-side apply field manager
		if err := h.Client.Patch(r.Context(), obj, client.Apply, &client.PatchOptions{
			FieldManager: "k8sgptclient",
			Force:        &[]bool{true}[0],
		}); err != nil {
			http.Error(w, fmt.Sprintf("Failed to apply resource: %v", err), http.StatusInternalServerError)
			return
		}

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
			http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
			return
		}
	}
}
