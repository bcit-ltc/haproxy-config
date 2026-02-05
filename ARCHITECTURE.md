# HAProxy Operator Architecture

## Overview

The HAProxy Operator is a Kubernetes controller that enables GitOps-based management of HAProxy load balancers using the Dataplane API. It's designed for environments where HAProxy sits behind a corporate firewall, making direct external API access impossible.

## Problem Statement

**Constraint**: HAProxy load balancer behind corporate firewall
- Only ports 80/443 accessible from external networks
- Dataplane API (port 5555) not externally reachable
- GitHub Actions cannot directly apply configuration

**Solution**: Kubernetes operator running inside the firewall
- Has network access to HAProxy Dataplane API
- Watches ConfigMap resources for configuration changes
- Reconciles desired state with HAProxy using Dataplane API

## Architecture Diagram

```
┌──────────────────────────────────────────────────────────────┐
│  External Network                                             │
│                                                                │
│  ┌─────────────────────────────────────┐                     │
│  │  Git Repository (GitHub)             │                     │
│  │  ├── config/                         │                     │
│  │  │   ├── haproxy-config.yaml        │                     │
│  │  │   └── secret.yaml                │                     │
│  │  └── README.md                       │                     │
│  └─────────────────────────────────────┘                     │
│                  │                                             │
└──────────────────│─────────────────────────────────────────────┘
                   │ Git sync (HTTPS)
                   │
┌──────────────────▼─────────────────────────────────────────────┐
│  Corporate Firewall (Ports 80/443 only)                        │
└─────────────────────────────────────────────────────────────────┘
                   │
┌──────────────────▼─────────────────────────────────────────────┐
│  Internal Network (Kubernetes Cluster)                          │
│                                                                  │
│  ┌────────────────────────────────────────────────────────┐   │
│  │  Flux/ArgoCD (GitOps Engine)                           │   │
│  │  - Monitors Git repository                             │   │
│  │  - Syncs ConfigMaps & Secrets to cluster               │   │
│  └────────────────────────────────────────────────────────┘   │
│                  │                                              │
│                  ▼                                              │
│  ┌────────────────────────────────────────────────────────┐   │
│  │  Kubernetes API Server                                 │   │
│  │  ├── ConfigMap: haproxy-config                         │   │
│  │  │   └── labels:                                       │   │
│  │  │       haproxy.operator/config: "true"               │   │
│  │  └── Secret: haproxy-api-credentials                   │   │
│  └────────────────────────────────────────────────────────┘   │
│                  │                                              │
│                  │ watches (Kubernetes API)                    │
│                  ▼                                              │
│  ┌────────────────────────────────────────────────────────┐   │
│  │  HAProxy Operator Pod                                  │   │
│  │  ┌──────────────────────────────────────────────────┐ │   │
│  │  │  Controller Manager                               │ │   │
│  │  │  ├── ConfigMap Reconciler                         │ │   │
│  │  │  │   ├── Watch ConfigMaps                         │ │   │
│  │  │  │   ├── Parse YAML config                        │ │   │
│  │  │  │   ├── Compute SHA256 hash                      │ │   │
│  │  │  │   ├── Compare with last applied                │ │   │
│  │  │  │   └── Reconcile if changed                     │ │   │
│  │  │  ├── HAProxy Client                               │ │   │
│  │  │  │   ├── Create/Update backends                   │ │   │
│  │  │  │   ├── Create/Update frontends                  │ │   │
│  │  │  │   └── Apply via Dataplane API                  │ │   │
│  │  │  └── Status Manager                               │ │   │
│  │  │      └── Update ConfigMap annotations             │ │   │
│  │  └──────────────────────────────────────────────────┘ │   │
│  │                                                         │   │
│  │  Metrics: :8080/metrics (Prometheus)                  │   │
│  │  Health:  :8081/healthz, :8081/readyz                 │   │
│  └────────────────────────────────────────────────────────┘   │
│                  │                                              │
│                  │ HTTP (port 5555, internal network)          │
│                  ▼                                              │
│  ┌────────────────────────────────────────────────────────┐   │
│  │  HAProxy Load Balancer (192.168.2.2)                │   │
│  │  ├── Dataplane API (:5555)                            │   │
│  │  ├── HTTP Frontend (:80)                              │   │
│  │  └── HTTPS Frontends (:443, :4443, :5000)             │   │
│  └────────────────────────────────────────────────────────┘   │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

## Component Architecture

### 1. Main Operator (`main.go`)

**Responsibilities**:
- Initialize controller-runtime manager
- Configure RBAC and permissions
- Setup controller reconciliation
- Enable leader election for HA
- Configure metrics and health endpoints

**Key Features**:
- Uses controller-runtime patterns from FluxCD
- Supports namespace-scoped or cluster-wide watching
- Configurable via command-line flags
- Structured logging with zap

### 2. ConfigMap Reconciler (`internal/controller/configmap_controller.go`)

**Responsibilities**:
- Watch ConfigMaps with label `haproxy.operator/config=true`
- Parse YAML configuration from ConfigMap data
- Compute SHA256 hash of configuration
- Compare with last applied hash (stored in annotations)
- Trigger reconciliation only when configuration changes
- Update status annotations on ConfigMap

**Reconciliation Logic**:

```go
func (r *ConfigMapReconciler) Reconcile(ctx, req) {
    1. Fetch ConfigMap
    2. Validate label exists
    3. Extract config.yaml key
    4. Parse YAML → Config struct
    5. Fetch API credentials from Secret
    6. Compute current hash
    7. Compare with last-applied-hash annotation
    8. If different:
        a. Apply config to HAProxy
        b. Update hash annotation
        c. Update status annotation
    9. Requeue after 5 minutes (for drift detection)
}
```

**Status Tracking**:
- Annotations on ConfigMap:
  - `haproxy.operator/last-applied-hash`: SHA256 of last applied config
  - `haproxy.operator/status`: Current status (Applied, ParseError, etc.)
  - `haproxy.operator/last-applied-time`: Timestamp of last apply
  - `haproxy.operator/status-message`: Error message if failed

### 3. HAProxy Client (`internal/haproxy/client.go`)

**Responsibilities**:
- Parse YAML configuration into Go structs
- Interact with HAProxy Dataplane API v2
- Create/update backends and servers
- Create/update frontends and binds
- Handle API errors and retries

**API Operations**:

```
Backends:
  GET    /services/haproxy/configuration/backends/{name}
  POST   /services/haproxy/configuration/backends
  PUT    /services/haproxy/configuration/backends/{name}

Servers:
  GET    /services/haproxy/configuration/servers/{name}?backend={backend}
  POST   /services/haproxy/configuration/servers?backend={backend}
  PUT    /services/haproxy/configuration/servers/{name}?backend={backend}

Frontends:
  GET    /services/haproxy/configuration/frontends/{name}
  POST   /services/haproxy/configuration/frontends
  PUT    /services/haproxy/configuration/frontends/{name}

Binds:
  GET    /services/haproxy/configuration/binds/{name}?frontend={frontend}
  POST   /services/haproxy/configuration/binds?frontend={frontend}
  PUT    /services/haproxy/configuration/binds/{name}?frontend={frontend}
```

**Idempotency**:
- Check if resource exists before create/update
- Use PUT for updates, POST for creates
- Handle 404 errors gracefully

## Data Flow

### Configuration Application Flow

```
1. Developer commits to Git
   └── git push origin main

2. Flux/ArgoCD detects change
   └── git pull
   └── kubectl apply configmap

3. Kubernetes API updates ConfigMap
   └── Triggers watch event

4. Operator receives event
   └── Reconcile() called

5. Operator processes ConfigMap
   ├── Parse YAML
   ├── Validate structure
   ├── Compute hash
   └── Compare with last-applied-hash

6. If changed:
   ├── Fetch credentials from Secret
   ├── Create HAProxy client
   ├── For each backend:
   │   ├── Check if exists
   │   ├── Create or update backend
   │   └── For each server:
   │       ├── Check if exists
   │       └── Create or update server
   └── For each frontend:
       ├── Check if exists
       ├── Create or update frontend
       └── For each bind:
           ├── Check if exists
           └── Create or update bind

7. Update ConfigMap annotations
   ├── last-applied-hash = new_hash
   ├── status = "Applied"
   └── last-applied-time = now()

8. Requeue after 5 minutes
   └── For drift detection
```

### Credential Management Flow

```
1. Secret created in Kubernetes
   apiVersion: v1
   kind: Secret
   data:
     username: YWRtaW4=
     password: c2VjcmV0

2. ConfigMap references Secret
   apiConfig:
     secretRef: haproxy-api-credentials

3. Operator reads Secret
   ├── Fetch Secret from Kubernetes API
   ├── Extract username (base64 decode)
   ├── Extract password (base64 decode)
   └── Create HAProxy client with credentials

4. HAProxy client uses Basic Auth
   └── Authorization: Basic <base64(username:password)>
```

## Configuration Schema

### ConfigMap Structure

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: haproxy-config
  labels:
    haproxy.operator/config: "true"  # Required for operator to watch
data:
  config.yaml: |
    # HAProxy API configuration
    apiConfig:
      url: http://192.168.2.2:5555/v2
      secretRef: haproxy-api-credentials
      insecure: false
    
    # Backend definitions
    backends:
      - name: backend-name
        mode: http|tcp
        balance:
          algorithm: roundrobin|source|leastconn
        servers:
          - name: server-name
            address: ip-or-hostname
            port: 443
            ssl: true|false
            check: true|false
            verify: none|required
    
    # Frontend definitions
    frontends:
      - name: frontend-name
        mode: http|tcp
        binds:
          - name: bind-name
            address: "*"|ip-address
            port: 80
            ssl: true|false
        defaultBackend: backend-name
```

## Security Model

### RBAC Permissions

The operator requires minimal permissions:

```yaml
ClusterRole:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "watch"]
```

**Security Boundaries**:
- Read-only access to Secrets (cannot modify)
- Can only update ConfigMap annotations (status tracking)
- Cannot create or delete ConfigMaps
- No access to other Kubernetes resources

### Credential Storage

**Best Practices**:
1. Store HAProxy API credentials in Kubernetes Secrets
2. Use separate Secrets per environment (dev, staging, prod)
3. Enable Secret encryption at rest in etcd
4. Rotate credentials regularly
5. Use RBAC to restrict Secret access

### Network Security

**Recommendations**:
1. Use NetworkPolicies to restrict operator egress:
   ```yaml
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: haproxy-operator-egress
   spec:
     podSelector:
       matchLabels:
         app.kubernetes.io/name: haproxy-operator
     policyTypes:
     - Egress
     egress:
     - to:
       - ipBlock:
           cidr: 192.168.2.2/32  # HAProxy IP
       ports:
       - protocol: TCP
         port: 5555  # Dataplane API
   ```

2. Enable TLS for HAProxy Dataplane API
3. Use certificate validation (`insecure: false`)

## High Availability

### Leader Election

The operator supports leader election for HA deployments:

```yaml
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: manager
        args:
        - --leader-elect
```

**How it works**:
- Multiple replicas run simultaneously
- Only one replica (leader) performs reconciliation
- Other replicas are on standby
- If leader fails, new leader elected automatically
- Uses Kubernetes lease mechanism

### Health Checks

**Liveness Probe** (`:8081/healthz`):
- Checks if operator is running
- Kubernetes restarts pod if unhealthy

**Readiness Probe** (`:8081/readyz`):
- Checks if operator is ready to serve
- Kubernetes removes from service if not ready

## Monitoring and Observability

### Metrics

Prometheus metrics exposed on `:8080/metrics`:

**Controller Metrics** (from controller-runtime):
- `controller_runtime_reconcile_total` - Total reconciliations
- `controller_runtime_reconcile_errors_total` - Reconciliation errors
- `controller_runtime_reconcile_time_seconds` - Reconciliation duration

**Custom Metrics** (can be added):
- `haproxy_operator_config_apply_total` - Total config applications
- `haproxy_operator_config_apply_errors_total` - Failed applications
- `haproxy_operator_backends_total` - Number of managed backends
- `haproxy_operator_frontends_total` - Number of managed frontends

### Logging

Structured logging with zap:

```json
{
  "level": "info",
  "ts": 1705327800,
  "logger": "controller.configmap",
  "msg": "configuration successfully applied to HAProxy",
  "configmap": "default/haproxy-config",
  "currentHash": "a1b2c3...",
  "lastAppliedHash": "d4e5f6..."
}
```

**Log Levels**:
- `debug`: Detailed operation logs
- `info`: Normal operation events
- `error`: Errors requiring attention

### Status Tracking

ConfigMap annotations provide status visibility:

```yaml
metadata:
  annotations:
    haproxy.operator/status: "Applied"
    haproxy.operator/last-applied-hash: "sha256:a1b2c3..."
    haproxy.operator/last-applied-time: "2024-01-15T10:30:00Z"
    haproxy.operator/status-message: ""
```

## GitOps Integration

### Flux Integration

```yaml
GitRepository (monitors Git)
      ↓
Kustomization (syncs to cluster)
      ↓
ConfigMap (haproxy-config)
      ↓
HAProxy Operator (applies to HAProxy)
```

**Benefits**:
- Git as single source of truth
- Automatic drift correction
- Audit trail via Git history
- Pull request workflow for changes

### Rollback Capability

**Git-based rollback**:
```bash
# Revert to previous commit
git revert HEAD
git push origin main

# Flux detects change and syncs
# Operator applies old configuration
```

**Manual rollback**:
```bash
# Apply previous ConfigMap version
kubectl apply -f backups/haproxy-config-v1.yaml

# Operator reconciles automatically
```

## Comparison with Alternatives

| Feature | Operator | Terraform | Ansible |
|---------|----------|-----------|---------|
| Declarative | ✅ Yes | ✅ Yes | ⚠️ Partial |
| Continuous Reconciliation | ✅ Yes | ❌ No | ❌ No |
| Drift Detection | ✅ Auto | ⚠️ Manual | ⚠️ Manual |
| GitOps Native | ✅ Yes | ⚠️ Via Atlantis | ⚠️ Via Tower |
| Kubernetes Native | ✅ Yes | ❌ No | ❌ No |
| Behind Firewall | ✅ Yes | ❌ Requires API | ⚠️ Requires SSH |
| State Management | ✅ K8s | ⚠️ Remote backend | ❌ None |

## Limitations

### Current Limitations

1. **Scope**: Only manages backends, frontends, servers, and binds
2. **No Global/Defaults**: Cannot manage HAProxy `global` or `defaults` sections
3. **No ACLs**: Advanced ACL rules not supported
4. **Single Instance**: One ConfigMap per HAProxy instance
5. **API Version**: Requires Dataplane API v2+

### Workarounds

For complete HAProxy management, combine with:
- Base configuration file (global, defaults)
- Templating for complex rules
- Direct API calls for advanced features

## Future Enhancements

### Planned Features

1. **Custom Resource Definition (CRD)**:
   ```yaml
   apiVersion: haproxy.example.com/v1alpha1
   kind: HAProxyConfig
   spec:
     backends: [...]
     frontends: [...]
   ```

2. **Advanced ACL Support**:
   ```yaml
   frontends:
     - name: https_frontend
       acls:
         - name: is_api
           criterion: path_beg
           value: /api
       rules:
         - action: use_backend
           backend: api-backend
           condition: if is_api
   ```

3. **Multi-Instance Management**:
   ```yaml
   apiConfig:
     instances:
       - name: primary
         url: http://haproxy-01:5555/v2
       - name: secondary
         url: http://haproxy-02:5555/v2
   ```

4. **Dry-Run Mode**:
   ```yaml
   apiConfig:
     dryRun: true  # Validate but don't apply
   ```

5. **Webhook Validation**:
   - Validate ConfigMap before acceptance
   - Prevent invalid configurations
   - Test HAProxy config syntax

## Development Guidelines

### Adding New Features

1. **Update structs** in `internal/haproxy/client.go`
2. **Implement API calls** in client methods
3. **Update reconciler** in `internal/controller/`
4. **Add tests** for new functionality
5. **Update documentation**

### Testing Strategy

```
Unit Tests:
  - Parse configuration
  - Hash computation
  - API client methods

Integration Tests:
  - Full reconciliation loop
  - ConfigMap watching
  - Status updates

End-to-End Tests:
  - Deploy operator to cluster
  - Apply ConfigMap
  - Verify HAProxy state
```

## Conclusion

The HAProxy Operator provides a Kubernetes-native, GitOps-friendly solution for managing HAProxy load balancers behind corporate firewalls. By leveraging the controller-runtime framework and following FluxCD patterns, it enables declarative, continuous reconciliation of HAProxy configuration while maintaining security and observability.
