# k8sgptclient

K8sGPT Client is a Kubernetes tool uses k8s-agent and k8sgpt-remediation that combines the power of GPT with Kubernetes to automatically detect and remediate issues in your cluster.

## Architecture

The project consists of two main components:

### 1. [K8s Agent](/k8s-agent/README.md)
A Kubernetes agent that provides REST APIs for:

#### Pod Operations
- List all pods in a namespace
- Get pod status with detailed probe results (liveness and readiness)
- Stream pod logs in real-time
- Retrieve pod YAML configurations
- Monitor pod health and readiness states

#### Deployment Operations
- Get pod names for a deployment
- Retrieve deployment YAML configurations
- Monitor deployment status

#### Resource Management
- Apply Kubernetes resources
- Retrieve resource YAML configurations
- Monitor resource states

#### Health and Lifecycle Management
- Graceful termination handling
- Proper shutdown sequence
- Health check endpoints (/livez, /readyz)
- Connection draining during shutdown
- Signal handling (SIGTERM, SIGINT)

### 2. [K8sGPT Remediation Server](/k8sgpt-remediation/README.md)
A service that:
- Analyzes Kubernetes pod and deployment resources for issues using K8sGPT
- Generates remediation solutions using GPTscript
- Applies fixes automatically using k8s-agent `/apply` endpoint
- Monitors the status of remediated resources

#### How it works

- Analyze Kubernetes cluster and find/detect the for on pods and deployments issues in the cluster
- Generate remediation solutions using K8sGPT which runs after `k8sgpt analyze --explain` command in every 30 seconds
- Using k8s-agent `/pods/{namespace}/{podName}/yaml` and `/deployments/{namespace}/{deploymentName}/yaml` endpoints to get the current yaml of the pod and deployment 
- Which are passed with the prompt to GPTScript to generate the remediation manifest
- Remediation manifest is applied to the cluster using K8s Agent `/apply` endpoint

## Installation

### Prerequisites
- Kubernetes cluster
   - Create a cluster using [Terraform](Terraform/README.md)
   - Kind Cluster
- kubectl CLI

### Create Kind Cluster

```bash
kind create cluster --name k8sgptclient
```

### Create a namespace for k8sgptclient
```bash
kubectl create namespace k8sgptclient
```

### Install K8s Agent
```bash
kubectl create -k manifest/k8sgptclient/agent-resources/
```

```bash
kubectl get all -n k8sgptclient
```

```yaml
# manifest/k8sgptclient/agent-resources/kustomization.yaml
resources:
  - k8s-agent-deploy.yaml
  - service.yaml
  - rbac.yaml
  - namespace.yaml
```

### Configure Remediation Server

#### Create a secret for the remediation server

```bash
kubectl create secret generic k8sgpt-sample-secret --from-literal=openai-api-key=<your-openai-api-key> -n k8sgptclient
```
#### Verify the secret

```bash
kubectl get secrets -n k8sgptclient k8sgpt-sample-secret -o yaml
```

#### Create a configmap for the remediation server perticulerly for K8sGPT

```yaml
# configmap.yaml
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

```yaml
resources:
  - remediation-server-deploy.yaml
  - service.yaml
  - rbac.yaml
  - configmap.yaml ## Pass configmap.yaml 
```

### Install Remediation Server
```bash
kubectl create -k manifest/k8sgptclient/remediation-server-resources/
```

### Verify the installation
```bash
kubectl get all -n k8sgptclient
```

### Create faulty deployment

Create a faulty deployment to test the remediation server
```bash
kubectl create -f manifest/faulty-manifest/faulty-deployment.yaml
```
The image is used in the deployment is `busybox:lat` which is not a valid image and will not be able to start the pod


Check the status which should be in not ready state and pods are not ready for few minutes
```bash
kubectl get all 
```

### Monitor logs of remediation server and k8s-agent

Check logs of remediation server in different terminal
```bash
kubectl -n k8sgptclient logs deployments/remediation-server -f
```
Check logs of k8s-agent in different terminal
```bash
kubectl -n k8sgptclient logs deployments/k8s-agent -f
```

### Verify the remediation of the deployment

Check the status of the faulty deployment which should be in ready state
```bash
kubectl get all 
```

### Create a faulty pod

Create a faulty pod to test the remediation server
```bash
kubectl create -f manifest/faulty-manifest/faulty-pod.yaml
```
The image is used in the deployment is `ngin` which is not a valid image and will not be able to start the pod

Check the status which should be in not ready state and pods are not ready for few minutes
```bash
kubectl get all 
```

### Monitor logs of remediation server and k8s-agent

Check logs of remediation server in different terminal
```bash
kubectl -n k8sgptclient logs deployments/remediation-server -f
```
Check logs of k8s-agent in different terminal
```bash
kubectl -n k8sgptclient logs deployments/k8s-agent -f
```

### Verify the remediation of the pod

Check the status of the faulty pod which should be in ready state
```bash
kubectl get all 
```








