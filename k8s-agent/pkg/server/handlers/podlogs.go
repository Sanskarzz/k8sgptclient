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
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PodLogs returns a handler for GET /pods/{namespace}/{podName}/logs endpoint
func (h *ClientHandler) PodLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logger := log.FromContext(r.Context()).WithName("pod-logs")

		// Check if the request method is GET
		if r.Method != http.MethodGet {
			err := fmt.Errorf("invalid method: %s, allowed: %s", r.Method, http.MethodGet)
			logger.Error(err, "Method not allowed")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse path parameters
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) != 5 {
			err := fmt.Errorf("invalid path: %s, allowed: /pods/{namespace}/{podName}/logs", r.URL.Path)
			logger.Error(err, "Invalid path")
			http.Error(w, "Invalid path. Expected: /pods/{namespace}/{podName}/logs", http.StatusBadRequest)
			return
		}
		namespace := parts[2]
		podName := parts[3]

		logger = logger.WithValues(
			"namespace", namespace,
			"pod", podName,
		)
		logger.Info("Getting pod logs")

		// Get the REST config
		cfg, err := config.GetConfig()
		if err != nil {
			logger.Error(err, "Failed to get kubeconfig")
			http.Error(w, fmt.Sprintf("Failed to get kubeconfig: %v", err), http.StatusInternalServerError)
			return
		}

		// Create the clientset
		clientset, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			logger.Error(err, "Failed to create clientset")
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
			logger = logger.WithValues("container", containerName)
			logger.V(1).Info("Container specified in request")
		}

		// Request the pod logs
		req := clientset.CoreV1().Pods(namespace).GetLogs(podName, podLogOpts)
		podLogs, err := req.Stream(r.Context())
		if err != nil {
			logger.Error(err, "Failed to get pod logs stream")
			http.Error(w, fmt.Sprintf("Failed to get pod logs: %v", err), http.StatusInternalServerError)
			return
		}
		defer podLogs.Close()

		logger.Info("Successfully started log streaming")

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
