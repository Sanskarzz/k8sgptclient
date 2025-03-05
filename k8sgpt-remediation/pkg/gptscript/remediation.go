package gptscript

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/gptscript-ai/go-gptscript"
	"github.com/k8sgpt-ai/k8sgpt/pkg/common"
)

type RemediationGenerator struct {
	agentURL string
	g        *gptscript.GPTScript
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

	return nil
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

		url = fmt.Sprintf("%s/deployment/%s/%s/yaml", r.agentURL, namespace, deployName)
		log.Printf("Fetching deployment YAML from: %s", url)
	} else {
		// It's a standalone pod
		log.Printf("Processing standalone pod: %s", result.Name)
		url = fmt.Sprintf("%s/pod/%s/yaml", r.agentURL, result.Name)
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
