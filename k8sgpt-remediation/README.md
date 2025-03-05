# K8sGPT Remediation

K8sGPT Remediation is a service that uses K8sGPT command analyze to analyze Kubernetes cluster find/detect the issues in the cluster and generate remediation solutions and apply them to the cluster. 

## How it works

- Analyze Kubernetes cluster and find/detect the for on pods and deployments issues in the cluster
- Generate remediation solutions using K8sGPT which runs after `k8sgpt analyze --explain` command in every 30 seconds
- Using k8s-agent `/pods/{namespace}/{podName}/yaml` and `/deployments/{namespace}/{deploymentName}/yaml` endpoints to get the current yaml of the pod and deployment 
- Which are passed with the prompt to GPTScript to generate the remediation manifest
- Remediation manifest is applied to the cluster using K8s Agent `/apply` endpoint
- After applying the remediation manifest, the remediation server monitors the status of the remediated resource using k8s-agent `/pods/{namespace}/{podName}/status` and `/deployments/{namespace}/{deploymentName}/status` endpoints.

### Configuration

The remediation server is configured using a configmap.yaml file.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: k8sgpt-config
  namespace: k8sgptclient
data:
  k8sgpt.yaml: |
    ai:
      providers:
        - name: openai
          model: gpt-4o-mini
          password: ### Your OpenAI API Key to configure k8sgpt ###
          temperature: 0.7
          topp: 0.5
          topk: 50
          maxtokens: 2048
          customheaders: []
      defaultprovider: openai
    commit: unknown
    date: unknown
    kubeconfig: ""
    kubecontext: ""
    version: 0.3.50
```

