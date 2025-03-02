package handlers

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// PodLogs returns a handler for GET /pods/{namespace}/{podName}/logs endpoint
func (h *ClientHandler) PodLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if the request method is GET
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse path parameters
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) != 5 {
			http.Error(w, "Invalid path. Expected: /pods/{namespace}/{podName}/logs", http.StatusBadRequest)
			return
		}
		namespace := parts[2]
		podName := parts[3]

		// Get the REST config
		cfg, err := config.GetConfig()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get kubeconfig: %v", err), http.StatusInternalServerError)
			return
		}

		// Create the clientset
		clientset, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create clientset: %v", err), http.StatusInternalServerError)
			return
		}

		// Set up the pod logs options
		podLogOpts := &corev1.PodLogOptions{
			Follow:    false,
			Previous:  false,
			TailLines: nil, // Get all logs
		}

		// Get container name from query parameter
		if containerName := r.URL.Query().Get("container"); containerName != "" {
			podLogOpts.Container = containerName
		}

		// Request the pod logs
		req := clientset.CoreV1().Pods(namespace).GetLogs(podName, podLogOpts)
		podLogs, err := req.Stream(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get pod logs: %v", err), http.StatusInternalServerError)
			return
		}
		defer podLogs.Close()

		// Set headers for streaming response
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Transfer-Encoding", "chunked")

		// Stream the logs
		reader := bufio.NewReader(podLogs)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				fmt.Fprintf(w, "Error reading logs: %v\n", err)
				return
			}
			_, err = w.Write(line)
			if err != nil {
				fmt.Fprintf(w, "Error writing logs: %v\n", err)
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}
