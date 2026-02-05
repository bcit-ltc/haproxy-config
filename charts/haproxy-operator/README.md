# HAProxy Operator Helm Chart

A Kubernetes operator that manages HAProxy configuration via the Dataplane API. Designed for GitOps workflows and environments where HAProxy sits behind a corporate firewall.

## TL;DR

```bash
helm repo add haproxy-operator https://example.com/charts
helm install haproxy-operator haproxy-operator/haproxy-operator
```

Or install from local chart:

```bash
helm install haproxy-operator ./charts/haproxy-operator
```

## Introduction

This chart deploys the HAProxy Operator on a Kubernetes cluster using Helm. The operator watches ConfigMap resources and reconciles HAProxy configuration using the Dataplane API.

### Key Features

- ✅ **GitOps-Ready**: Works seamlessly with Flux and ArgoCD
- ✅ **Declarative Configuration**: Define HAProxy config in YAML ConfigMaps
- ✅ **Behind Firewall**: Runs inside cluster, accesses HAProxy via internal network
- ✅ **Hash-Based Drift Detection**: Only reconciles when config changes
- ✅ **High Availability**: Supports leader election for multi-replica deployments
- ✅ **Observability**: Prometheus metrics and structured logging

## Prerequisites

- Kubernetes 1.24+
- Helm 3.8+
- HAProxy with Dataplane API enabled and accessible from cluster
- PV provisioner support in the underlying infrastructure (optional)

## Installing the Chart

### Basic Installation

```bash
helm install haproxy-operator ./charts/haproxy-operator \
  --namespace haproxy-operator-system \
  --create-namespace
```

### Installation with Custom Values

```bash
helm install haproxy-operator ./charts/haproxy-operator \
  --namespace haproxy-operator-system \
  --create-namespace \
  --set image.repository=myregistry/haproxy-operator \
  --set image.tag=v1.0.0 \
  --set operator.watchNamespace=production
```

### Installation with Values File

Create `my-values.yaml`:

```yaml
image:
  repository: myregistry/haproxy-operator
  tag: v1.0.0

replicaCount: 3

operator:
  watchNamespace: production
  leaderElection: true

resources:
  limits:
    cpu: 1000m
    memory: 512Mi
  requests:
    cpu: 200m
    memory: 256Mi

serviceMonitor:
  enabled: true

networkPolicy:
  enabled: true
  egress:
    - to:
      - ipBlock:
          cidr: 192.168.2.2/32
      ports:
      - protocol: TCP
        port: 5555
```

Install:

```bash
helm install haproxy-operator ./charts/haproxy-operator \
  -f my-values.yaml \
  --namespace haproxy-operator-system \
  --create-namespace
```

## Uninstalling the Chart

```bash
helm uninstall haproxy-operator -n haproxy-operator-system
```

## Configuration

### Quick Start with Sample Config

Deploy with a sample configuration for testing:

```bash
helm install haproxy-operator ./charts/haproxy-operator \
  --set sampleConfig.enabled=true \
  --set sampleConfig.apiUrl=http://192.168.2.2:5555/v2 \
  --set sampleConfig.apiPassword=your-password
```

### Production Configuration

For production deployments:

```yaml
# production-values.yaml
replicaCount: 3

image:
  repository: myregistry/haproxy-operator
  tag: v1.0.0
  pullPolicy: Always

operator:
  watchNamespace: ""  # Watch all namespaces
  leaderElection: true
  logLevel: info

resources:
  limits:
    cpu: 1000m
    memory: 512Mi
  requests:
    cpu: 200m
    memory: 256Mi

podDisruptionBudget:
  enabled: true
  minAvailable: 1

serviceMonitor:
  enabled: true
  interval: 30s

networkPolicy:
  enabled: true
  egress:
    - to:
      - ipBlock:
          cidr: 192.168.2.2/32  # Your HAProxy IP
      ports:
      - protocol: TCP
        port: 5555
    - to:
      - namespaceSelector:
          matchLabels:
            name: kube-system
      ports:
      - protocol: UDP
        port: 53
    - to:
      - namespaceSelector: {}
      ports:
      - protocol: TCP
        port: 443

affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/name
            operator: In
            values:
            - haproxy-operator
        topologyKey: kubernetes.io/hostname
```

Install:

```bash
helm install haproxy-operator ./charts/haproxy-operator \
  -f production-values.yaml \
  -n haproxy-operator-system \
  --create-namespace
```

## Parameters

### Global Parameters

| Name | Description | Value |
|------|-------------|-------|
| `nameOverride` | Override chart name | `""` |
| `fullnameOverride` | Override fully qualified app name | `""` |
| `imagePullSecrets` | Image pull secrets for private registries | `[]` |

### Image Parameters

| Name | Description | Value |
|------|-------------|-------|
| `image.registry` | Container image registry | `docker.io` |
| `image.repository` | Container image repository | `yourorg/haproxy-operator` |
| `image.tag` | Container image tag (defaults to chart appVersion) | `""` |
| `image.pullPolicy` | Container image pull policy | `IfNotPresent` |

### Operator Configuration

| Name | Description | Value |
|------|-------------|-------|
| `replicaCount` | Number of operator replicas | `1` |
| `operator.watchNamespace` | Namespace to watch (empty = all) | `""` |
| `operator.configMapName` | Specific ConfigMap to watch | `""` |
| `operator.configMapKey` | Key containing config in ConfigMap | `config.yaml` |
| `operator.leaderElection` | Enable leader election | `true` |
| `operator.logLevel` | Log level (debug, info, error) | `info` |
| `operator.logDevelopment` | Enable development logging | `false` |

### ServiceAccount Parameters

| Name | Description | Value |
|------|-------------|-------|
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.automount` | Automount service account token | `true` |
| `serviceAccount.annotations` | Service account annotations | `{}` |
| `serviceAccount.name` | Service account name | `""` |

### RBAC Parameters

| Name | Description | Value |
|------|-------------|-------|
| `rbac.create` | Create RBAC resources | `true` |
| `rbac.clusterWide` | Use ClusterRole vs Role | `true` |

### Security Parameters

| Name | Description | Value |
|------|-------------|-------|
| `podSecurityContext.runAsNonRoot` | Run as non-root user | `true` |
| `podSecurityContext.seccompProfile.type` | Seccomp profile | `RuntimeDefault` |
| `securityContext.allowPrivilegeEscalation` | Allow privilege escalation | `false` |
| `securityContext.capabilities.drop` | Drop capabilities | `[ALL]` |
| `securityContext.readOnlyRootFilesystem` | Read-only root filesystem | `true` |
| `securityContext.runAsNonRoot` | Run as non-root | `true` |
| `securityContext.runAsUser` | User ID to run as | `65532` |

### Service Parameters

| Name | Description | Value |
|------|-------------|-------|
| `service.type` | Service type | `ClusterIP` |
| `service.metricsPort` | Metrics port | `8080` |
| `service.annotations` | Service annotations | `{}` |

### Monitoring Parameters

| Name | Description | Value |
|------|-------------|-------|
| `metrics.enabled` | Enable metrics endpoint | `true` |
| `metrics.bindAddress` | Metrics bind address | `:8080` |
| `serviceMonitor.enabled` | Create ServiceMonitor | `false` |
| `serviceMonitor.interval` | Scrape interval | `30s` |
| `serviceMonitor.scrapeTimeout` | Scrape timeout | `10s` |
| `serviceMonitor.additionalLabels` | Additional labels | `{}` |
| `serviceMonitor.namespace` | Namespace for ServiceMonitor | `""` |

### Resource Management

| Name | Description | Value |
|------|-------------|-------|
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `256Mi` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |

### Health Checks

| Name | Description | Value |
|------|-------------|-------|
| `livenessProbe.httpGet.path` | Liveness probe path | `/healthz` |
| `livenessProbe.httpGet.port` | Liveness probe port | `8081` |
| `livenessProbe.initialDelaySeconds` | Initial delay | `15` |
| `livenessProbe.periodSeconds` | Period | `20` |
| `readinessProbe.httpGet.path` | Readiness probe path | `/readyz` |
| `readinessProbe.httpGet.port` | Readiness probe port | `8081` |
| `readinessProbe.initialDelaySeconds` | Initial delay | `5` |
| `readinessProbe.periodSeconds` | Period | `10` |

### High Availability

| Name | Description | Value |
|------|-------------|-------|
| `podDisruptionBudget.enabled` | Enable PDB | `false` |
| `podDisruptionBudget.minAvailable` | Minimum available pods | `1` |
| `autoscaling.enabled` | Enable HPA (not recommended) | `false` |
| `autoscaling.minReplicas` | Minimum replicas | `1` |
| `autoscaling.maxReplicas` | Maximum replicas | `3` |

### Networking

| Name | Description | Value |
|------|-------------|-------|
| `networkPolicy.enabled` | Enable NetworkPolicy | `false` |
| `networkPolicy.egress` | Egress rules | `[...]` |

### Scheduling

| Name | Description | Value |
|------|-------------|-------|
| `nodeSelector` | Node selector | `{}` |
| `tolerations` | Tolerations | `[]` |
| `affinity` | Affinity rules | `{}` |
| `topologySpreadConstraints` | Topology spread constraints | `[]` |
| `priorityClassName` | Priority class name | `""` |

### Sample Configuration (Testing)

| Name | Description | Value |
|------|-------------|-------|
| `sampleConfig.enabled` | Install sample ConfigMap | `false` |
| `sampleConfig.name` | Sample ConfigMap name | `haproxy-config` |
| `sampleConfig.secretName` | Sample Secret name | `haproxy-api-credentials` |
| `sampleConfig.apiUrl` | HAProxy API URL | `http://192.168.2.2:5555/v2` |
| `sampleConfig.apiUsername` | API username | `admin` |
| `sampleConfig.apiPassword` | API password | `changeme` |

## Usage

### Creating HAProxy Configuration

After installing the operator, create a Secret and ConfigMap:

#### 1. Create Secret with API Credentials

```bash
kubectl create secret generic haproxy-api-credentials \
  --from-literal=username=admin \
  --from-literal=password=your-secure-password \
  -n default
```

#### 2. Create ConfigMap with HAProxy Configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: haproxy-config
  namespace: default
  labels:
    haproxy.operator/config: "true"  # Required!
data:
  config.yaml: |
    apiConfig:
      url: http://192.168.2.2:5555/v2
      secretRef: haproxy-api-credentials
      insecure: false

    backends:
      - name: web-backend
        mode: http
        balance:
          algorithm: roundrobin
        servers:
          - name: web-01
            address: 192.168.1.100
            port: 443
            ssl: true
            check: true
            verify: none
          - name: web-02
            address: 192.168.1.101
            port: 443
            ssl: true
            check: true
            verify: none

    frontends:
      - name: https-frontend
        mode: http
        binds:
          - name: https
            address: "*"
            port: 443
            ssl: true
        defaultBackend: web-backend
```

Apply:

```bash
kubectl apply -f haproxy-config.yaml
```

### Verify Operation

```bash
# Check operator logs
kubectl logs -n haproxy-operator-system \
  -l app.kubernetes.io/name=haproxy-operator -f

# Check ConfigMap status
kubectl get configmap haproxy-config -o yaml | grep haproxy.operator

# Expected annotations:
# haproxy.operator/last-applied-hash: "sha256:abc123..."
# haproxy.operator/status: "Applied"
# haproxy.operator/last-applied-time: "2024-01-15T10:30:00Z"
```

## GitOps Integration

### Flux Example

```yaml
# helmrelease.yaml
apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: haproxy-operator
  namespace: haproxy-operator-system
spec:
  interval: 5m
  chart:
    spec:
      chart: ./charts/haproxy-operator
      sourceRef:
        kind: GitRepository
        name: haproxy-operator
        namespace: flux-system
  values:
    replicaCount: 3
    operator:
      leaderElection: true
    serviceMonitor:
      enabled: true
```

### ArgoCD Example

```yaml
# application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: haproxy-operator
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/yourorg/haproxy-operator
    targetRevision: main
    path: charts/haproxy-operator
    helm:
      values: |
        replicaCount: 3
        operator:
          leaderElection: true
  destination:
    server: https://kubernetes.default.svc
    namespace: haproxy-operator-system
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true
```

## Upgrading

### Upgrade to Latest Version

```bash
helm upgrade haproxy-operator ./charts/haproxy-operator \
  -n haproxy-operator-system
```

### Upgrade with New Values

```bash
helm upgrade haproxy-operator ./charts/haproxy-operator \
  -n haproxy-operator-system \
  --set replicaCount=3 \
  --set operator.leaderElection=true
```

### Upgrade with Values File

```bash
helm upgrade haproxy-operator ./charts/haproxy-operator \
  -n haproxy-operator-system \
  -f my-values.yaml
```

## Troubleshooting

### Check Deployment Status

```bash
helm status haproxy-operator -n haproxy-operator-system
```

### View Operator Logs

```bash
kubectl logs -n haproxy-operator-system \
  -l app.kubernetes.io/name=haproxy-operator \
  --tail=100 -f
```

### Check ConfigMap Status

```bash
kubectl get configmap haproxy-config -o yaml | grep haproxy.operator
```

### Test HAProxy API Connectivity

From within the cluster:

```bash
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -u admin:password http://192.168.2.2:5555/v2/info
```

### Common Issues

#### Operator Not Reconciling

**Problem**: ConfigMap changes not being applied

**Solution**: Ensure ConfigMap has the required label:

```yaml
metadata:
  labels:
    haproxy.operator/config: "true"
```

#### Authentication Failed

**Problem**: Cannot authenticate to HAProxy API

**Solution**: Verify Secret credentials:

```bash
kubectl get secret haproxy-api-credentials -o yaml
kubectl get secret haproxy-api-credentials -o jsonpath='{.data.password}' | base64 -d
```

#### Connection Refused

**Problem**: Cannot reach HAProxy Dataplane API

**Solution**: Check network connectivity and NetworkPolicy:

```bash
# If NetworkPolicy enabled, verify egress rules allow HAProxy IP
kubectl get networkpolicy -n haproxy-operator-system
kubectl describe networkpolicy haproxy-operator -n haproxy-operator-system
```

## Examples

See the `examples/` directory for more configuration examples:

- Basic configuration
- Multi-backend setup
- SSL/TLS configuration
- Network policy examples
- High availability setup

## Development

### Linting

```bash
helm lint ./charts/haproxy-operator
```

### Template Validation

```bash
helm template haproxy-operator ./charts/haproxy-operator \
  --debug \
  --namespace haproxy-operator-system
```

### Dry Run

```bash
helm install haproxy-operator ./charts/haproxy-operator \
  --dry-run \
  --debug \
  --namespace haproxy-operator-system
```

### Package Chart

```bash
helm package ./charts/haproxy-operator
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test with `helm lint` and `helm template`
5. Submit a pull request

## License

Apache License 2.0

## Support

- **Documentation**: [GitHub Repository](https://github.com/example/haproxy-operator)
- **Issues**: [GitHub Issues](https://github.com/example/haproxy-operator/issues)
- **HAProxy Dataplane API**: [Documentation](https://www.haproxy.com/documentation/haproxy-data-plane-api/)

## Maintainers

- HAProxy Operator Team
