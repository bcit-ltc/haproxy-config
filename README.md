# HAProxy Operator for Kubernetes

A Kubernetes operator that watches ConfigMap resources and reconciles HAProxy configuration via the Dataplane API. Built using controller-runtime patterns inspired by FluxCD.

## Overview

This operator enables GitOps-style management of HAProxy load balancers from within Kubernetes clusters, even when the HAProxy instance is behind a corporate firewall.

### Architecture

```
┌─────────────────────────────────────┐
│  Git Repository (External)           │
│  └── haproxy-config.yaml             │
└─────────────────────────────────────┘
            │
            │ Flux syncs
            ▼
┌─────────────────────────────────────┐
│  Kubernetes Cluster (Inside Firewall)│
│  ┌───────────────────────────────┐  │
│  │  ConfigMap                    │  │
│  │  haproxy.operator/config=true │  │
│  └───────────────────────────────┘  │
│            │ watches                │
│            ▼                         │
│  ┌───────────────────────────────┐  │
│  │  HAProxy Operator             │  │
│  │  - Detects changes            │  │
│  │  - Parses YAML config         │  │
│  │  - Applies via Dataplane API  │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
            │
            │ Port 5555 (internal)
            ▼
┌─────────────────────────────────────┐
│  HAProxy + Dataplane API             │
│  192.168.2.2:5555                  │
└─────────────────────────────────────┘
```

## Features
## Overview

- ✅ **GitOps-Ready**: Works with Flux, ArgoCD, or any GitOps tool
- ✅ **Declarative**: Define HAProxy configuration in YAML
- ✅ **Secure**: Uses Kubernetes Secrets to keep configuration private
- ✅ **Idempotent**: Only applies changes when configuration differs
- ✅ **Hash-Based Detection**: Uses SHA256 to detect configuration drift
- ✅ **Credential Management**: Stores API credentials in Kubernetes Secrets
- ✅ **Status Tracking**: Annotates Secrets with reconciliation status
- ✅ **Leader Election**: Supports high availability with multiple replicas
- ✅ **Metrics**: Prometheus metrics on port 8080
- ✅ **Public Repo Safe**: Keep operator public while configs remain private

## Prerequisites

- Kubernetes cluster (1.24+)
- HAProxy with Dataplane API enabled
  - [Dataplane API Reference](https://www.haproxy.com/documentation/dataplaneapi/community/?v=v3#overview)
- Network connectivity from cluster to HAProxy API endpoint
- Go 1.21+ (for building)

## Quick Start

### 1. Deploy the Operator

**Using Helm (Recommended):**

```bash
helm install haproxy-operator ./charts/haproxy-operator \
  --namespace haproxy-operator-system \
  --create-namespace
```

**Using kubectl:**

```bash
kubectl apply -f config/deployment.yaml
```

### 2. Create API Credentials Secret

```bash
kubectl create secret generic haproxy-api-credentials \
  --from-literal=username=admin \
  --from-literal=password=your-secure-password \
  --namespace=default
```

### 3. Create HAProxy Configuration Secret

```bash
kubectl create secret generic haproxy-config \
  --from-file=config.yaml=./your-config.yaml \
  --namespace=default

# Add required label
kubectl label secret haproxy-config haproxy.operator/config=true
```

Example configuration:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: haproxy-config
  namespace: default
  labels:
    haproxy.operator/config: "true"  # Required label
type: Opaque
stringData:
  config.yaml: |
    apiConfig:
      url: http://your-haproxy:5555/v2
      secretRef: haproxy-api-credentials
      insecure: false

    backends:
      - name: my-backend
        mode: http
        balance:
          algorithm: roundrobin
        servers:
          - name: server-01
            address: 10.0.1.10
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
```

### 4. Verify Reconciliation

```bash
# Check operator logs
kubectl logs -n haproxy-operator-system -l app.kubernetes.io/name=haproxy-operator -f

# Check Secret status annotations
kubectl get secret haproxy-config -o yaml | grep haproxy.operator
```

## Configuration Format

### Complete Example

```yaml
apiConfig:
  url: http://192.168.2.2:5555/v2
  secretRef: haproxy-api-credentials
  insecure: false

backends:
  - name: cluster01
    mode: http
    balance:
      algorithm: source  # roundrobin, source, leastconn, etc.
    servers:
      - name: server-01
        address: 192.168.1.100
        port: 443
        ssl: true
        check: true
        verify: none

frontends:
  - name: https_frontend
    mode: http
    binds:
      - name: https
        address: "*"
        port: 443
        ssl: true
      - name: http
        address: "*"
        port: 80
    defaultBackend: cluster01
```

### Field Reference

#### apiConfig

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | Yes | HAProxy Dataplane API URL |
| `secretRef` | string | Yes | Name of Secret containing credentials |
| `insecure` | bool | No | Skip TLS verification (default: false) |

#### backends

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Backend name |
| `mode` | string | Yes | Mode: `http` or `tcp` |
| `balance.algorithm` | string | Yes | Load balancing algorithm |
| `servers` | array | Yes | List of backend servers |

#### servers

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Server name |
| `address` | string | Yes | Server IP/hostname |
| `port` | int | Yes | Server port |
| `ssl` | bool | No | Enable SSL (default: false) |
| `check` | bool | No | Enable health checks (default: false) |
| `verify` | string | No | SSL verification: `none`, `required` |

#### frontends

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Frontend name |
| `mode` | string | Yes | Mode: `http` or `tcp` |
| `binds` | array | Yes | List of bind addresses |
| `defaultBackend` | string | Yes | Default backend name |

#### binds

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Bind name |
| `address` | string | Yes | Listen address (`*` for all) |
| `port` | int | Yes | Listen port |
| `ssl` | bool | No | Enable SSL (default: false) |

## Operator Configuration

### Command-Line Flags

```bash
--metrics-bind-address string
    Metrics endpoint address (default ":8080")

--health-probe-bind-address string
    Health probe endpoint address (default ":8081")

--leader-elect
    Enable leader election (recommended for HA)

--namespace string
    Watch specific namespace (empty = all namespaces)

--configmap-name string
    Watch specific ConfigMap by name

--configmap-key string
    ConfigMap key containing configuration (default "config.yaml")
```

### Environment Variables

```bash
# Set via TF_VAR_ prefix for compatibility
TF_VAR_configmap_name=haproxy-config
TF_VAR_configmap_key=config.yaml
```

## GitOps Integration

### Flux Configuration

```yaml
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
---
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
```

### ArgoCD Configuration

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: haproxy-config
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/yourorg/haproxy-config
    targetRevision: HEAD
    path: config
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

## Installation Methods

### Helm Installation (Recommended)

#### Basic Installation

```bash
helm install haproxy-operator ./charts/haproxy-operator \
  --namespace haproxy-operator-system \
  --create-namespace
```

#### Installation with Custom Values

```bash
helm install haproxy-operator ./charts/haproxy-operator \
  --namespace haproxy-operator-system \
  --create-namespace \
  --set image.repository=myregistry/haproxy-operator \
  --set image.tag=v1.0.0 \
  --set replicaCount=3 \
  --set operator.watchNamespace=production
```

#### Installation with Values File

Create `values.yaml`:

```yaml
image:
  repository: myregistry/haproxy-operator
  tag: v1.0.0

replicaCount: 3

operator:
  watchNamespace: production
  leaderElection: true

serviceMonitor:
  enabled: true

networkPolicy:
  enabled: true
```

Install:

```bash
helm install haproxy-operator ./charts/haproxy-operator \
  -f values.yaml \
  -n haproxy-operator-system \
  --create-namespace
```

#### Upgrade

```bash
helm upgrade haproxy-operator ./charts/haproxy-operator \
  -n haproxy-operator-system
```

#### Uninstall

```bash
helm uninstall haproxy-operator -n haproxy-operator-system
```

For detailed Helm chart documentation, see [charts/haproxy-operator/README.md](charts/haproxy-operator/README.md).

### kubectl Installation

```bash
kubectl apply -f config/deployment.yaml
```

## Building and Deploying

### Build from Source

```bash
# Build binary
go build -o bin/manager main.go

# Build container image
docker build -t haproxy-operator:latest .

# Push to registry
docker tag haproxy-operator:latest your-registry/haproxy-operator:v1.0.0
docker push your-registry/haproxy-operator:v1.0.0
```

### Deploy Custom Image

**Using Helm:**

```bash
helm upgrade haproxy-operator ./charts/haproxy-operator \
  --set image.repository=your-registry/haproxy-operator \
  --set image.tag=v1.0.0 \
  -n haproxy-operator-system
```

**Using kubectl:**

Update `config/deployment.yaml`:

```yaml
spec:
  template:
    spec:
      containers:
      - name: manager
        image: your-registry/haproxy-operator:v1.0.0
```

Then apply:

```bash
kubectl apply -f config/deployment.yaml
```

## Monitoring

### Prometheus Metrics

The operator exposes metrics on `:8080/metrics`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: haproxy-operator-metrics
  namespace: haproxy-operator-system
spec:
  selector:
    app.kubernetes.io/name: haproxy-operator
  ports:
  - name: metrics
    port: 8080
```

Add ServiceMonitor for Prometheus:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: haproxy-operator
  namespace: haproxy-operator-system
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: haproxy-operator
  endpoints:
  - port: metrics
    interval: 30s
```

### Status Annotations

The operator adds annotations to ConfigMaps:

```yaml
metadata:
  annotations:
    haproxy.operator/last-applied-hash: "a1b2c3d4..."
    haproxy.operator/status: "Applied"
    haproxy.operator/last-applied-time: "2024-01-15T10:30:00Z"
    haproxy.operator/status-message: ""
```

Status values:
- `Applied` - Successfully applied
- `ParseError` - YAML parsing failed
- `ConfigError` - Configuration validation failed
- `ApplyError` - Failed to apply to HAProxy

## Troubleshooting

### Check Operator Logs

```bash
kubectl logs -n haproxy-operator-system \
  -l app.kubernetes.io/name=haproxy-operator \
  --tail=100 -f
```

### Check Secret Status

```bash
kubectl get secret haproxy-config -o jsonpath='{.metadata.annotations}'
```

### Test HAProxy API Connectivity

```bash
# From within the cluster
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -u admin:password http://192.168.2.2:5555/v2/info
```

### Common Issues

#### "Configuration unchanged, skipping reconciliation"

This is normal. The operator uses hash-based detection and only reconciles when the configuration actually changes.

#### "Failed to get secret"

Ensure the API credentials Secret exists in the same namespace:

```bash
kubectl get secret haproxy-api-credentials -n default
```

#### "Connection refused"

Check network connectivity from the cluster to the HAProxy API endpoint. Verify firewall rules allow traffic on port 5555.

#### "Authentication failed"

Verify credentials in the Secret:

```bash
kubectl get secret haproxy-api-credentials -o jsonpath='{.data.username}' | base64 -d
kubectl get secret haproxy-api-credentials -o jsonpath='{.data.password}' | base64 -d
```

## Development

### Run Locally

```bash
# Export kubeconfig
export KUBECONFIG=~/.kube/config

# Run operator locally
go run main.go \
  --namespace=default \
  --configmap-name=haproxy-config \
  --configmap-key=config.yaml
```

### Run Tests

```bash
go test ./... -v
```

### Generate Mocks

```bash
go install github.com/golang/mock/mockgen@latest
go generate ./...
```

## Security Considerations

1. **Credentials**: Store HAProxy API credentials in Kubernetes Secrets
2. **RBAC**: Operator only needs read access to ConfigMaps and Secrets
3. **Network Policies**: Restrict operator to only communicate with HAProxy API
4. **TLS**: Enable TLS verification for production (`insecure: false`)
5. **Non-root**: Operator runs as non-root user (UID 65532)

## Limitations

- Does not manage HAProxy `global` or `defaults` sections
- Does not support advanced ACL rules (use Terraform for complex rules)
- Assumes Dataplane API v2 format
- Single HAProxy instance per ConfigMap

## Roadmap

- [ ] Support for Custom Resource Definitions (CRDs)
- [ ] Advanced ACL rule support
- [ ] Multi-instance HAProxy management
- [ ] Dry-run mode
- [ ] Configuration validation before apply
- [ ] Rollback capability
- [ ] Webhook for ConfigMap validation

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## License

Apache License 2.0

## References

- [HAProxy Dataplane API](https://www.haproxy.com/documentation/haproxy-data-plane-api/)
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [FluxCD](https://fluxcd.io/)
- [Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
