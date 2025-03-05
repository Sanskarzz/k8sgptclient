package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ProbeStatus represents the status of a probe
type ProbeStatus struct {
	LastProbeTime      string `json:"lastProbeTime,omitempty"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Status             bool   `json:"status"`
	Failure            string `json:"failure,omitempty"`
	SuccessCount       int32  `json:"successCount"`
	FailureCount       int32  `json:"failureCount"`
	Details            string `json:"details"`
}

// ContainerProbes represents the probe status for a container
type ContainerProbes struct {
	ContainerName string      `json:"containerName"`
	Liveness      ProbeStatus `json:"liveness,omitempty"`
	Readiness     ProbeStatus `json:"readiness,omitempty"`
}

// PodStatus represents the status response structure
type PodStatus struct {
	Name            string                   `json:"name"`
	Namespace       string                   `json:"namespace"`
	Phase           corev1.PodPhase          `json:"phase"`
	Conditions      []corev1.PodCondition    `json:"conditions"`
	ContainerStatus []corev1.ContainerStatus `json:"containerStatus"`
	StartTime       string                   `json:"startTime,omitempty"`
	PodIP           string                   `json:"podIP,omitempty"`
	HostIP          string                   `json:"hostIP,omitempty"`
	ProbeResults    []ContainerProbes        `json:"probeResults,omitempty"`
}

// formatProbeDetails formats probe configuration details
func formatProbeDetails(probe *corev1.Probe) string {
	if probe == nil {
		return ""
	}

	var details string

	// Add handler details
	if probe.Exec != nil {
		details = fmt.Sprintf("exec %v", probe.Exec.Command)
	} else if probe.HTTPGet != nil {
		details = fmt.Sprintf("http-get %v:%v%v", probe.HTTPGet.Host, probe.HTTPGet.Port.String(), probe.HTTPGet.Path)
	} else if probe.TCPSocket != nil {
		details = fmt.Sprintf("tcp-socket %v", probe.TCPSocket.Port.String())
	}

	// Add timing details
	details += fmt.Sprintf(" delay=%ds timeout=%ds period=%ds",
		probe.InitialDelaySeconds,
		probe.TimeoutSeconds,
		probe.PeriodSeconds)

	return details
}

// PodStatus returns a handler for GET /pods/{namespace}/{podName}/status endpoint
func (h *ClientHandler) PodStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logger := log.FromContext(r.Context()).WithName("pod-status")

		if r.Method != http.MethodGet {
			err := fmt.Errorf("invalid method: %s, allowed: %s", r.Method, http.MethodGet)
			logger.Error(err, "Method not allowed")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse path parameters
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) != 5 {
			err := fmt.Errorf("invalid path: %s, allowed: /pods/{namespace}/{podName}/status", r.URL.Path)
			logger.Error(err, "Invalid path")
			http.Error(w, "Invalid path. Expected: /pods/{namespace}/{podName}/status", http.StatusBadRequest)
			return
		}
		namespace := parts[2]
		podName := parts[3]

		logger = logger.WithValues(
			"namespace", namespace,
			"pod", podName,
		)
		logger.Info("Getting pod status")

		// Get pod
		var pod corev1.Pod
		if err := h.Client.Get(r.Context(), types.NamespacedName{
			Namespace: namespace,
			Name:      podName,
		}, &pod); err != nil {
			logger.Error(err, "Failed to get pod")
			http.Error(w, fmt.Sprintf("Failed to get pod: %v", err), http.StatusInternalServerError)
			return
		}

		// Create status response
		status := PodStatus{
			Name:            pod.Name,
			Namespace:       pod.Namespace,
			Phase:           pod.Status.Phase,
			Conditions:      pod.Status.Conditions,
			ContainerStatus: pod.Status.ContainerStatuses,
			PodIP:           pod.Status.PodIP,
			HostIP:          pod.Status.HostIP,
		}

		if pod.Status.StartTime != nil {
			status.StartTime = pod.Status.StartTime.String()
		}

		// Add probe results for each container
		// Iterate through each container in the pod spec
		for _, container := range pod.Spec.Containers {
			// Initialize probe info for this container
			probes := ContainerProbes{
				ContainerName: container.Name,
			}

			// Find matching container status from pod status
			var containerStatus *corev1.ContainerStatus
			for i := range pod.Status.ContainerStatuses {
				if pod.Status.ContainerStatuses[i].Name == container.Name {
					containerStatus = &pod.Status.ContainerStatuses[i]
					break
				}
			}

			if containerStatus != nil {
				// Liveness probe status
				if container.LivenessProbe != nil {
					probes.Liveness = ProbeStatus{
						Status:       containerStatus.Ready,
						Details:      formatProbeDetails(container.LivenessProbe),
						SuccessCount: container.LivenessProbe.SuccessThreshold,
						FailureCount: container.LivenessProbe.FailureThreshold,
					}

					// Add failure information
					if containerStatus.LastTerminationState.Terminated != nil {
						probes.Liveness.Failure = containerStatus.LastTerminationState.Terminated.Message
						if containerStatus.LastTerminationState.Terminated.FinishedAt.Time.Unix() > 0 {
							probes.Liveness.LastProbeTime = containerStatus.LastTerminationState.Terminated.FinishedAt.String()
						}
					}

					// Add restart count information
					if containerStatus.RestartCount > 0 {
						probes.Liveness.FailureCount = containerStatus.RestartCount
					}
				}

				// Readiness probe status
				if container.ReadinessProbe != nil {
					probes.Readiness = ProbeStatus{
						Status:       containerStatus.Ready,
						Details:      formatProbeDetails(container.ReadinessProbe),
						SuccessCount: container.ReadinessProbe.SuccessThreshold,
						FailureCount: container.ReadinessProbe.FailureThreshold,
					}
				}
			}

			status.ProbeResults = append(status.ProbeResults, probes)
		}

		// Set response headers
		w.Header().Set("Content-Type", "application/json")

		// Write response
		if err := json.NewEncoder(w).Encode(status); err != nil {
			logger.Error(err, "Failed to encode response")
			http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
			return
		}
		logger.V(1).Info("Response sent successfully")
	}
}
