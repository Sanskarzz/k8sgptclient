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





