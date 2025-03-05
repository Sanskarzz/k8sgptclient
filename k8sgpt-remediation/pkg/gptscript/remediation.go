package gptscript

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gptscript-ai/go-gptscript"
	"github.com/k8sgpt-ai/k8sgpt/pkg/common"
)

type RemediationGenerator struct {
	agentURL string
	g        *gptscript.GPTScript
}

type PodStatus struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Phase           string            `json:"phase"`
	Conditions      []PodCondition    `json:"conditions"`
	ContainerStatus []ContainerStatus `json:"containerStatus"`
	StartTime       string            `json:"startTime"`
	PodIP           string            `json:"podIP"`
	HostIP          string            `json:"hostIP"`
	ProbeResults    []ProbeResult     `json:"probeResults"`
}

type PodCondition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	LastProbeTime      string `json:"lastProbeTime"`
	LastTransitionTime string `json:"lastTransitionTime"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
}

type ContainerStatus struct {
	Name         string `json:"name"`
	State        State  `json:"state"`
	LastState    State  `json:"lastState"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount"`
	Image        string `json:"image"`
	ImageID      string `json:"imageID"`
	Started      bool   `json:"started"`
}

type State struct {
	Waiting *WaitingState `json:"waiting,omitempty"`
}

type WaitingState struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type ProbeResult struct {
	ContainerName string      `json:"containerName"`
	Liveness      ProbeStatus `json:"liveness"`
	Readiness     ProbeStatus `json:"readiness"`
}

type ProbeStatus struct {
	Status       bool   `json:"status"`
	SuccessCount int32  `json:"successCount"`
	FailureCount int32  `json:"failureCount"`
	Details      string `json:"details"`
}

func NewRemediationGenerator(apiKey string, agentURL string) (*RemediationGenerator, error) {
	log.Printf("Initializing RemediationGenerator with agent URL: %s", agentURL)
	g, err := gptscript.NewGPTScript(gptscript.GlobalOptions{
		OpenAIAPIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GPTScript: %v", err)
	}

	return &RemediationGenerator{
		agentURL: agentURL,
		g:        g,
	}, nil
}

func (r *RemediationGenerator) GenerateRemediation(ctx context.Context, result common.Result) (string, error) {
	log.Printf("Starting remediation generation for resource: Kind=%s, Name=%s", result.Kind, result.Name)
	// Get resource YAML from k8s agent
	resourceYAML, err := r.getResourceYAML(result)
	if err != nil {
		log.Printf("Error getting resource YAML: %v", err)
		return "", fmt.Errorf("failed to get resource YAML: %v", err)
	}
	log.Printf("Successfully retrieved resource YAML")

	// Prepare error messages
	var errorMsgs string
	for _, err := range result.Error {
		errorMsgs += err.Text + "\n"
	}
	log.Printf("Collected error messages:\n%s", errorMsgs)
	// Create GPTScript tool
	log.Printf("Creating GPTScript tool for remediation")

	prompt := fmt.Sprintf(`Given the following Kubernetes %s YAML and issues:

Current YAML:
%s

Issues Detected:
%s

Analysis Solution:
%s

Please only provide the corrected YAML.

Format the response as valid Kubernetes YAML.

Do not include any triple backticks and yaml word in the output. Just provide correct YAML`,
		result.Kind, resourceYAML, errorMsgs, result.Details)

	// Run GPTScript evaluation
	log.Printf("Starting GPTScript evaluation")
	tool := gptscript.ToolDef{
		Name:         "kubernetes-remediation",
		Description:  "Generates remediation YAML for Kubernetes resources",
		Instructions: prompt,
	}
	// Run GPTScript evaluation
	log.Printf("Starting GPTScript evaluation")
	run, err := r.g.Evaluate(ctx, gptscript.Options{}, tool)
	if err != nil {
		log.Printf("Error during GPTScript evaluation: %v", err)
		return "", fmt.Errorf("failed to evaluate GPTScript: %v", err)
	}

	remediationYAML, err := run.Text()
	if err != nil {
		log.Printf("Error getting GPTScript result: %v", err)
		return "", fmt.Errorf("failed to get GPTScript result: %v", err)
	}

	log.Printf("Successfully generated remediation YAML")
	log.Printf("Generated remediation YAML:\n%s\n", remediationYAML)

	if err := r.applyRemediationYAML(ctx, remediationYAML); err != nil {
		log.Printf("Failed to apply remediation YAML: %v", err)
		return remediationYAML, fmt.Errorf("failed to apply remediation: %v", err)
	}
	log.Printf("Successfully applied remediation YAML") // Write remediation YAML to file

	return remediationYAML, nil
}

func (r *RemediationGenerator) applyRemediationYAML(ctx context.Context, yaml string) error {
	url := fmt.Sprintf("%s/apply", r.agentURL)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(yaml))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/yaml")

	// Send request
	log.Printf("Sending apply request to: %s", url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("apply failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var applyResp struct {
		Kind      string `json:"kind"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Action    string `json:"action"`
	}

	if err := json.Unmarshal(body, &applyResp); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	log.Printf("Apply response: Kind=%s, Name=%s/%s, Action=%s",
		applyResp.Kind, applyResp.Namespace, applyResp.Name, applyResp.Action)

	// Wait for pod status
	if err := r.waitForPodStatus(ctx, applyResp.Namespace, applyResp.Name, applyResp.Kind); err != nil {
		return fmt.Errorf("pod status check failed: %v", err)
	}

	return nil
}

func (r *RemediationGenerator) waitForPodStatus(ctx context.Context, namespace, name, kind string) error {
	log.Printf("Starting pod status check for %s: %s/%s", kind, namespace, name)
	// For deployments, we need to get the pod name from the deployment
	if kind == "Deployment" {
		return r.waitForDeploymentPods(ctx, namespace, name)
	}

	// For pods, directly check the pod status
	return r.waitForPod(ctx, namespace, name)
}

func (r *RemediationGenerator) waitForDeploymentPods(ctx context.Context, namespace, deployName string) error {
	log.Printf("Checking status of pods for deployment %s/%s", namespace, deployName)

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")
		case <-timeout:
			return fmt.Errorf("timeout waiting for deployment pods")
		case <-ticker.C:
			// Get pod names for deployment
			url := fmt.Sprintf("%s/deployments/%s/%s/pods", r.agentURL, namespace, deployName)
			resp, err := http.Get(url)
			if err != nil {
				log.Printf("Error getting deployment pods: %v", err)
				continue
			}

			var deployPods struct {
				Name      string   `json:"name"`
				Namespace string   `json:"namespace"`
				PodNames  []string `json:"podNames"`
			}

			body, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Printf("Error reading response: %v", err)
				continue
			}

			if err := json.Unmarshal(body, &deployPods); err != nil {
				log.Printf("Error parsing deployment pods: %v", err)
				continue
			}

			if len(deployPods.PodNames) == 0 {
				log.Printf("No pods found for deployment %s/%s", namespace, deployName)
				continue
			}

			// Check each pod's status
			allPodsReady := true
			for _, podName := range deployPods.PodNames {
				if err := r.waitForPod(ctx, namespace, podName); err != nil {
					// If pod check fails, log and continue checking other pods
					log.Printf("Pod %s status check failed: %v", podName, err)
					continue
				}
				// If pod is ready, we can return success
				log.Printf("Pod %s is ready", podName)
				return nil
			}

			if allPodsReady {
				log.Printf("All pods for deployment %s/%s are ready", namespace, deployName)
				return nil
			}
		}
	}
}

func (r *RemediationGenerator) waitForPod(ctx context.Context, namespace, podName string) error {
	log.Printf("Checking status for pod %s/%s", namespace, podName)

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")
		case <-timeout:
			return fmt.Errorf("timeout waiting for pod")
		case <-ticker.C:
			url := fmt.Sprintf("%s/pods/%s/%s/status", r.agentURL, namespace, podName)
			resp, err := http.Get(url)
			if err != nil {
				log.Printf("Error getting pod status: %v", err)
				continue
			}

			body, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Printf("Error reading pod status: %v", err)
				continue
			}

			var status PodStatus
			if err := json.Unmarshal(body, &status); err != nil {
				log.Printf("Error parsing pod status: %v", err)
				continue
			}

			// Log detailed status
			log.Printf("Pod %s status:", podName)
			log.Printf("  Phase: %s", status.Phase)

			// Check container statuses
			for _, container := range status.ContainerStatus {
				log.Printf("  Container %s:", container.Name)
				log.Printf("    Ready: %v", container.Ready)
				if container.State.Waiting != nil {
					log.Printf("    Waiting: %s - %s",
						container.State.Waiting.Reason,
						container.State.Waiting.Message)
				}
			}
			//log.Printf("Pod %s is still pending, waiting...", podName)

			// If pod is running and all containers are ready, we're done
			if status.Phase == "Running" {
				allContainersReady := true
				for _, container := range status.ContainerStatus {
					if !container.Ready {
						allContainersReady = false
						break
					}
				}
				if allContainersReady {
					log.Printf("Pod %s is ready and running", podName)
					return nil
				}
			}

			// If pod is in a terminal failed state, return error
			if status.Phase == "Failed" {
				return fmt.Errorf("pod failed: %s", status.Phase)
			}

			// For other states (Pending, ContainerCreating, etc.), continue polling
			log.Printf("Pod %s is in %s state, waiting...", podName, status.Phase)
		}
	}
}

func (r *RemediationGenerator) getResourceYAML(result common.Result) (string, error) {
	var url string
	if result.ParentObject != "" {
		// It's a deployment issue
		log.Printf("Processing deployment resource with ParentObject: %s", result.ParentObject)
		// Get namespace from pod name (format: "namespace/pod-name")
		parts := strings.Split(result.Name, "/")
		if len(parts) != 2 {
			log.Printf("Invalid pod name format: %s", result.Name)
			return "", fmt.Errorf("invalid pod name format: %s", result.Name)
		}
		namespace := parts[0]

		// Get deployment name from ParentObject (format: "Deployment/name")
		deployParts := strings.Split(result.ParentObject, "/")
		if len(deployParts) != 2 {
			log.Printf("Invalid parent object format: %s", result.ParentObject)
			return "", fmt.Errorf("invalid parent object format: %s", result.ParentObject)
		}
		deployName := deployParts[1]

		url = fmt.Sprintf("%s/deployments/%s/%s/yaml", r.agentURL, namespace, deployName)
		log.Printf("Fetching deployment YAML from: %s", url)
	} else {
		// It's a standalone pod
		log.Printf("Processing standalone pod: %s", result.Name)
		url = fmt.Sprintf("%s/pods/%s/yaml", r.agentURL, result.Name)
	}
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error fetching YAML from agent: %v", err)
		return "", fmt.Errorf("failed to get resource YAML from agent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Agent returned non-200 status code: %d", resp.StatusCode)
		return "", fmt.Errorf("agent returned status %d", resp.StatusCode)
	}

	yaml, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading agent response: %v", err)
		return "", fmt.Errorf("failed to read agent response: %v", err)
	}

	log.Printf("Successfully retrieved YAML from agent")
	return string(yaml), nil
}

func (r *RemediationGenerator) Close() {
	log.Printf("Closing RemediationGenerator")
	if r.g != nil {
		r.g.Close()
	}
}
