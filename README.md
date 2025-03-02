# k8sgptclient

## Installation

1. Clone the repository:

```bash
git clone https://github.com/Sanskarzz/k8sgptclient.git
cd k8sgptclient
```

2. Build the application:

```bash
go build -o k8sgptclient
```

## Usage

### Starting the Server

Start the agent server:

```bash
./k8sgptclient serve agent --http-address=:8080
```

### Available Endpoints

1. **List Pods**
   ```bash
   # List pods in default namespace
   curl http://localhost:8080/pods

   # List pods in specific namespace
   curl http://localhost:8080/pods?namespace=kube-system
   ```

2. **Get Pod Status**
   ```bash
   curl http://localhost:8080/pods/{namespace}/{podName}/status
   ```

3. **Stream Pod Logs**
   ```bash
   # Get logs from all containers
   curl http://localhost:8080/pods/{namespace}/{podName}/logs

   # Get logs from specific container
   curl http://localhost:8080/pods/{namespace}/{podName}/logs?container={containerName}
   ```

4. **Apply Resources**
   ```bash
   curl -X POST http://localhost:8080/apply -d @resource.yaml
   ```
   
### Configuration

The application can be configured using command-line flags:

- `--http-address`: HTTP server address (default: ":8080")
- `--kubeconfig`: Path to kubeconfig file
- All standard kubectl configuration flags are supported   