# Argo Installation and Setup

## Create the Argo CD Namespace
```bash
kubectl create namespace argocd
```

## Install Argo CD
```bash
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

## Verify Argo CD Installation 
```bash
kubectl get pods -n argocd
```

## Install Argo CD Image updater
Apply the installation manifests
```bash
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj-labs/argocd-image-updater/stable/manifests/install.yaml
```

## Access the Argo CD API Server
```bash
kubectl port-forward svc/argocd-server -n argocd 8080:443
```

## Login to Argo CD
```bash
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
```

## Connect Private Git Repo using ArgoCD

First, generate an SSH key pair:

```bash
ssh-keygen -t ed25519 -C "argocd" -f ./argocd-repo-key -N ""
```

Add the public key to your GitHub repository:
- Go to your GitHub repository → Settings → Deploy keys
- Click "Add deploy key"
- Title: "ArgoCD"
- Key: Copy content of argocd-repo-key.pub
- Check "Allow write access"
- Click "Add key"

## Create Github secret 
```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: github-repo-ssh
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: git
  url: git@github.com:Sanskarzz/k8sgptclient.git
  sshPrivateKey: |
$(cat ./argocd-repo-key | sed 's/^/    /')
EOF

```

## Create Dockerhub registory secret
```bash
kubectl create secret docker-registry dockerhub-secret \
  --namespace argocd \
  --docker-server=https://registry.hub.docker.com \
  --docker-username=sanskardevops \
  --docker-password=dckr_pat_L3kM1q-DTZVVu4Qbosk20V05Pf0
```

## Apply to create configmap 
```
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-image-updater-config
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-image-updater-config
    app.kubernetes.io/part-of: argocd-image-updater
data:
  registries.conf: |
    registries:
    - name: Docker Hub
      prefix: docker.io
      api_url: https://registry.hub.docker.com
      credentials: pullsecret:argocd/dockerhub-secret
      default: true
      ping: yes
  log.level: debug
EOF
```

### Restart the argo image updater, server and controller:
```bash
kubectl rollout restart deployment argocd-image-updater -n argocd
kubectl rollout restart deployment argocd-repo-server -n argocd
kubectl rollout restart statefulset argocd-application-controller -n argocd
```

### Check the repo server logs:
```bash
kubectl logs -n argocd -l app.kubernetes.io/name=argocd-repo-server -f
```

### Apply Applications:
```bash
kubectl create -k argocd 
```

### Verify 
```bash
kubectl get application -n argocd
```

# Check the Argo CD image updater logs and check image upgradation:
```bash
kubectl logs -n argocd -l app.kubernetes.io/name=argocd-image-updater -f
```
