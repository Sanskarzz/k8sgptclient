First, generate an SSH key pair:

```
ssh-keygen -t ed25519 -C "argocd" -f ./argocd-repo-key -N ""
```

Add the public key to your GitHub repository:
- Go to your GitHub repository → Settings → Deploy keys
- Click "Add deploy key"
- Title: "ArgoCD"
- Key: Copy content of argocd-repo-key.pub
- Check "Allow write access"
- Click "Add key"


```
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

```
kubectl create secret docker-registry dockerhub-secret \
  --namespace argocd \
  --docker-server=https://index.docker.io/v1/ \
  --docker-username=sanskardevops \
  --docker-password=sanskardevops@123 
```

```
k edit configmaps -n argocd argocd-image-updater-config
```
```
apiVersion: v1
data:
  registries.conf: "registries:\n- name: Docker Hub\n  prefix: docker.io\n  api_url:
    https://registry-1.docker.io\n  credentials: pullsecret:argocd/dockerhub-secret\n
    \ default: true\n  ping: yes \n"
kind: ConfigMap
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"v1","kind":"ConfigMap","metadata":{"annotations":{},"labels":{"app.kubernetes.io/name":"argocd-image-updater-config","app.kubernetes.io/part-of":"argocd-image-updater"},"name":"argocd-image-updater-config","namespace":"argocd"}}
  creationTimestamp: "2025-03-06T08:08:43Z"
  labels:
    app.kubernetes.io/name: argocd-image-updater-config
    app.kubernetes.io/part-of: argocd-image-updater
  name: argocd-image-updater-config
  namespace: argocd
  resourceVersion: "39707"
  uid: 92a478ae-a272-4747-aed1-415eb6bb98e2

```

