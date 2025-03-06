## K8s Agent

K8s Agent is a Kubernetes agent that provides REST APIs for:
- Pod operations
- Deployment operations
- Resource management

Each endpoint provides detailed information about the requested resource, including:
- Current state and conditions
- Container statuses
- Probe results (liveness/readiness)
- Configuration details
- Error states and messages

Health and Lifecycle Management
- Graceful termination handling
- Proper shutdown sequence
- Health check endpoints (/livez, /readyz)
- Connection draining during shutdown
- Signal handling (SIGTERM, SIGINT)

## API Reference

### K8s Agent APIs

#### List all pods in a namespace
```http
GET /pods
```
Returns list of pods in the specified namespace

#### Stream pod logs
```http
GET /pods/{namespace}/{podName}/logs
```
Returns a stream of pod logs

#### Get pod status with probe results
```http
GET /pods/{namespace}/{podName}/status
```
Returns detailed pod status including:
- Phase
- Conditions
- Container statuses
- Probe results

#### Get pod YAML configuration
```http
GET /pods/{namespace}/{podName}/yaml
```
Returns the YAML configuration of the specified pod

#### Get deployment pod names
```http
GET /deployments/{namespace}/{deploymentName}/pods
```
Returns list of pods belonging to a deployment

#### Get deployment YAML
```http
GET /deployments/{namespace}/{deploymentName}/yaml
```
Returns the YAML configuration of the specified deployment

#### Apply Kubernetes resources
```http
POST /apply
```
Applies the specified Kubernetes resources

## Test with Go binary

### Clone the Repository
```bash
git clone https://github.com/Sanskarzz/k8sgptclient.git
```
```bash
cd k8sgptclient/k8s-agent
```

### Create kind cluster
```bash
kind create cluster --name k8s-agent
```

### Verify cluster created
```bash
kubectl get nodes
```

### Build Source Code
```bash
go build .
```

### Run the Binary
```bash
./k8s-agent serve agent 
```

### Create test pod and deployment to test the agent
```bash
kubectl run nginx --image=nginx
```
```bash
kubectl create deployment nginx --image=nginx
```


### Run command to list all pods in different terminal
```bash
curl http://localhost:8080/pods
```

### Run command to stream logs
```bash
curl http://localhost:8080/pods/default/nginx/logs
```

### Run command to status of the pods with probes results
```bash
curl http://localhost:8080/pods/default/nginx/status
```

### Run command to get Pod yaml configuration 
```bash
curl http://localhost:8080/pods/default/nginx/yaml
```

### Run command to get deployment pod names
```bash
curl http://localhost:8080/deployments/default/nginx/pods

```json
{"name":"nginx","namespace":"default","podNames":["nginx-5869d7778c-snjb7"]}
```

### Run command to get deployment yaml
```bash
curl http://localhost:8080/deployments/default/nginx/yaml
```

### Generate a kubernetes resource to test /apply endpoint 
```bash
kubectl run redis --image=redis --port=6379 --dry-run=client -o yaml > redis.yaml
```

### Run command to Apply Kubernetes resources 
```bash
sanskar@sanskar-HP-Laptop-15s-du1xxx:~/k8sgptdemo/k8sgptclient$ curl -X POST http://localhost:8080/apply \
  -H "Content-Type: application/yaml" \
  --data-binary "@redis.yaml"
```
```json
{"kind":"Pod","name":"redis","namespace":"default","action":"applied"}
```