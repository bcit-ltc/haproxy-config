# HAProxy Operator Quick Start

Get up and running with the HAProxy Operator in 5 minutes.

## Prerequisites

- Kubernetes cluster (1.24+)
- HAProxy with Dataplane API accessible from cluster
- `kubectl` configured

## Installation (1 minute)

```bash
# Deploy the operator
kubectl apply -f config/deployment.yaml

# Verify deployment
kubectl get pods -n haproxy-operator-system
```

## Setup Configuration (2 minutes)

### 1. Create API Credentials Secret

```bash
kubectl create secret generic haproxy-api-credentials \
  --from-literal=username=admin \
  --from-literal=password=your-password \
  -n default
```

### 2. Create HAProxy ConfigMap

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: haproxy-config
  namespace: default
  labels:
    haproxy.operator/config: "true"
data:
  config.yaml: |
    apiConfig:
      url: http://192.168.2.2:5555/v2
      secretRef: haproxy-api-credentials
      insecure: false

    backends:
      - name: my-backend
        mode: http
        balance:
          algorithm: roundrobin
        servers:
          - name: server-01
            address: 192.168.1.100
            port: 443
            ssl: true
            check: true
            verify: none

    frontends:
      - name: my-frontend
        mode: http
        binds:
          - name: http
            address: "*"
            port: 80
        defaultBackend: my-backend
EOF
```

## Verify (1 minute)

```bash
# Check operator logs
kubectl logs -n haproxy-operator-system \
  -l app.kubernetes.io/name=haproxy-operator \
  --tail=20

# Check ConfigMap status
kubectl get configmap haproxy-config -o yaml | grep haproxy.operator

# Expected output:
# haproxy.operator/last-applied-hash: "abc123..."
# haproxy.operator/status: "Applied"
```

## Common Operations

### Add a Backend Server

Edit the ConfigMap:

```bash
kubectl edit configmap haproxy-config
```

Add server to the `servers` list:

```yaml
servers:
  - name: server-02
    address: 192.168.1.101
    port: 443
    ssl: true
    check: true
    verify: none
```

Save and exit. The operator automatically reconciles within seconds.

### Update Configuration from File

```bash
# Edit local file
vim haproxy-config.yaml

# Apply changes
kubectl apply -f haproxy-config.yaml

# Watch reconciliation
kubectl logs -n haproxy-operator-system \
  -l app.kubernetes.io/name=haproxy-operator -f
```

### Check Status

```bash
# Get status annotation
kubectl get configmap haproxy-config \
  -o jsonpath='{.metadata.annotations.haproxy\.operator/status}'

# Get last applied time
kubectl get configmap haproxy-config \
  -o jsonpath='{.metadata.annotations.haproxy\.operator/last-applied-time}'
```

## GitOps Integration (Flux)

### 1. Create GitRepository

```bash
cat <<EOF | kubectl apply -f -
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: haproxy-config
  namespace: flux-system
spec:
  interval: 1m
  url: https://github.com/yourorg/haproxy-config
  ref:
    branch: main
EOF
```

### 2. Create Kustomization

```bash
cat <<EOF | kubectl apply -f -
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: haproxy-config
  namespace: flux-system
spec:
  interval: 5m
  sourceRef:
    kind: GitRepository
    name: haproxy-config
  path: ./config
  prune: true
  targetNamespace: default
EOF
```

### 3. Commit ConfigMap to Git

```bash
git add config/haproxy-config.yaml
git commit -m "Update HAProxy configuration"
git push origin main
```

Flux syncs automatically → Operator reconciles → HAProxy updated!

## Troubleshooting

### Operator Not Reconciling

**Check label on ConfigMap:**
```bash
kubectl get configmap haproxy-config -o jsonpath='{.metadata.labels}'
```

Must have: `haproxy.operator/config: "true"`

**Fix:**
```bash
kubectl label configmap haproxy-config haproxy.operator/config=true
```

### Authentication Failed

**Check Secret:**
```bash
kubectl get secret haproxy-api-credentials -o yaml
```

**Test credentials:**
```bash
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -u admin:password http://192.168.2.2:5555/v2/info
```

### Configuration Parse Error

**Check ConfigMap syntax:**
```bash
kubectl get configmap haproxy-config -o yaml
```

Ensure `config.yaml` key exists and contains valid YAML.

**Validate YAML locally:**
```bash
# Extract config
kubectl get configmap haproxy-config \
  -o jsonpath='{.data.config\.yaml}' > /tmp/config.yaml

# Validate
yamllint /tmp/config.yaml
```

### Connection Refused

**Check network connectivity:**
```bash
kubectl run -it --rm debug --image=nicolaka/netshoot --restart=Never -- \
  curl -v http://192.168.2.2:5555/v2/info
```

**Check operator logs:**
```bash
kubectl logs -n haproxy-operator-system \
  -l app.kubernetes.io/name=haproxy-operator \
  --tail=50
```

## Configuration Reference

### Minimal Configuration

```yaml
apiConfig:
  url: http://192.168.2.2:5555/v2
  secretRef: haproxy-api-credentials

backends:
  - name: backend-name
    mode: http
    balance:
      algorithm: roundrobin
    servers:
      - name: server-name
        address: 192.168.1.100
        port: 80

frontends:
  - name: frontend-name
    mode: http
    binds:
      - name: bind-name
        address: "*"
        port: 80
    defaultBackend: backend-name
```

### Full Configuration

```yaml
apiConfig:
  url: http://192.168.2.2:5555/v2
  secretRef: haproxy-api-credentials
  insecure: false

backends:
  - name: cluster01
    mode: http
    balance:
      algorithm: source
    servers:
      - name: prod-worker-01
        address: 142.232.110.64
        port: 443
        ssl: true
        check: true
        verify: none
      - name: prod-worker-02
        address: 142.232.110.65
        port: 443
        ssl: true
        check: true
        verify: none

  - name: admin-backend
    mode: http
    balance:
      algorithm: roundrobin
    servers:
      - name: admin-01
        address: 142.232.110.51
        port: 443
        ssl: true
        check: true
        verify: none

frontends:
  - name: http_frontend
    mode: http
    binds:
      - name: http
        address: "*"
        port: 80
    defaultBackend: cluster01

  - name: https_frontend
    mode: http
    binds:
      - name: https
        address: "*"
        port: 443
        ssl: true
      - name: fusion-controlplane
        address: "*"
        port: 4443
        ssl: true
    defaultBackend: cluster01
```

## Useful Commands

```bash
# Install operator
make install

# Install samples
make install-samples

# View logs
make logs

# Check status
make status

# Restart operator
make restart

# Uninstall operator
make uninstall

# Build and deploy
make deploy IMG=your-registry/haproxy-operator:v1.0.0
```

## Next Steps

- Read [README.md](README.md) for detailed documentation
- Review [ARCHITECTURE.md](ARCHITECTURE.md) for design details
- Check [config/samples/](config/samples/) for more examples
- Set up CI/CD pipeline for automated deployments
- Configure Prometheus monitoring
- Enable leader election for HA

## Support

- **Issues**: GitHub Issues
- **Documentation**: [README.md](README.md)
- **HAProxy API**: https://www.haproxy.com/documentation/haproxy-data-plane-api/
- **Controller Runtime**: https://github.com/kubernetes-sigs/controller-runtime

## Cheat Sheet

| Task | Command |
|------|---------|
| Deploy operator | `kubectl apply -f config/deployment.yaml` |
| Create secret | `kubectl create secret generic haproxy-api-credentials --from-literal=username=admin --from-literal=password=pass` |
| Apply config | `kubectl apply -f config/samples/configmap.yaml` |
| Watch logs | `kubectl logs -n haproxy-operator-system -l app.kubernetes.io/name=haproxy-operator -f` |
| Check status | `kubectl get configmap haproxy-config -o yaml \| grep haproxy.operator` |
| Edit config | `kubectl edit configmap haproxy-config` |
| Restart operator | `kubectl rollout restart deployment haproxy-operator -n haproxy-operator-system` |
| Delete operator | `kubectl delete -f config/deployment.yaml` |

**Success!** You now have a GitOps-ready HAProxy operator managing your load balancer from Kubernetes! 🎉
