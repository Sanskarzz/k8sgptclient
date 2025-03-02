package remediation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
)

type K8sGPTResult struct {
	ApiVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       struct {
		Details string `json:"details"`
	} `json:"spec"`
}

type WebhookHandler struct {
	k8sAgentURL string
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var result K8sGPTResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var YAML string // Want to find correct and fixed YAML form details then pass it to forwordToK8sAgent

	if err := h.forwardToK8sAgent(YAML); err != nil {
		log.Printf("Error forwarding to k8s-agent: %v", err)
		http.Error(w, "Failed to process remediation", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) forwardToK8sAgent(yaml string) error {
	// Forward to k8s-agent's apply endpoint
	resp, err := http.Post(
		fmt.Sprintf("%s/apply", h.k8sAgentURL),
		"application/yaml",
		bytes.NewBufferString(yaml),
	)
	if err != nil {
		return fmt.Errorf("failed to forward to k8s-agent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("k8s-agent returned non-200 status: %d", resp.StatusCode)
	}

	return nil
}

func Command() *cobra.Command {
	var httpAddress string
	var k8sAgentURL string
	command := &cobra.Command{
		Use:   "remediation-server",
		Short: "Run k8sgptclient remediation-server",
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := &WebhookHandler{
				k8sAgentURL: k8sAgentURL,
			}

			http.Handle("/webhook", handler)
			log.Printf("Starting remediation server on %s", httpAddress)
			return http.ListenAndServe(httpAddress, nil)
		},
	}

	command.Flags().StringVar(&httpAddress, "http-address", ":9090", "The address the remediation server binds to")
	command.Flags().StringVar(&k8sAgentURL, "k8s-agent-url", "http://localhost:8080", "URL of the k8s-agent service")
	return command
}
