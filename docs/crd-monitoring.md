# Custom Resource Definition (CRD) Monitoring with KSM

## Overview

The New Relic Kubernetes integration supports monitoring Custom Resource Definitions (CRDs) through kube-state-metrics (KSM). This enables you to collect metrics from operator-managed resources like Argo Rollouts, Karpenter NodePools, cert-manager Certificates, and more.

**Architecture**: The integration uses KSM's `customResourceState` feature to expose CRD metrics in Prometheus format, which nri-kubernetes then scrapes and exports to New Relic as dimensional metrics in the Metrics table.

**Data Model**: CRD metrics are stored as dimensional metrics in New Relic's Metrics table with the prefix `kube_customresource_`. Query them using NRQL:
```sql
FROM Metric SELECT * WHERE metricName LIKE 'kube_customresource_%'
```

> **📝 Documentation Status**
> **Tested Configurations**: Karpenter NodePool (fully tested and verified)
> **Theoretical Examples**: Argo Rollouts, cert-manager, External Secrets (provided as templates, not tested)
>
> Always verify field paths against your specific CRD versions using `kubectl get <resource> <name> -o yaml` before deploying untested configurations.

## Prerequisites

- kube-state-metrics v2.0+ (integrated in the newrelic-infrastructure chart)
- Helm chart version that supports customResourceState configuration
- RBAC permissions for KSM to access your CRDs

## Key Learnings

### 1. RBAC Permissions Are Critical

KSM requires explicit ClusterRole permissions to list and watch CRDs. Without proper RBAC, you'll see errors like:

```
failed to list <group>/<version>, Kind=<Kind>: <resources> is forbidden:
User "system:serviceaccount:<namespace>:kube-state-metrics" cannot list resource "<resources>"
```

**Solution**: Create a ClusterRole with list/watch permissions for each CRD you want to monitor.

### 2. CRD Installation Order Matters

When using Helm, CRD definitions must be installed before custom resource instances. Otherwise, you'll encounter:

```
no matches for kind <Kind> in version <group>/<version>
ensure CRDs are installed first
```

**Solution**:
- Place CRD definitions in the `crds/` directory of your Helm chart
- Place custom resource instances in the `templates/` directory
- Helm automatically installs `crds/` before `templates/`

### 3. Field Type Limitations

KSM has specific requirements for different metric types:

- **Gauge metrics**: Require numeric values (integers or floats). Strings will cause parsing errors.
- **Duration strings**: Cannot be parsed as gauges (e.g., "30s", "5m"). If your CRD uses numeric fields (e.g., `ttlSeconds`), Gauge works fine. For duration strings, use Info or StateSet types.
- **Timestamp strings**: RFC3339 timestamps (e.g., "2024-01-29T10:30:00Z") are handled differently by KSM version:
  - **KSM v2.10+**: Can automatically convert some RFC3339 timestamps to Unix timestamps
  - **Older versions**: Use Info type to expose as labels, or use a dedicated exporter
- **String fields**: Should use Info or StateSet metric types.
- **Complex objects**: Cannot be directly extracted. You need to specify the exact leaf field path.

**Example of problematic configuration**:
```yaml
# This WILL FAIL - "30s" is a string, not a number
- name: "consolidation_after_seconds"
  each:
    type: Gauge
    gauge:
      path: [spec, disruption, consolidateAfter]  # Value is "30s"
```

**What you'll see in KSM logs**:
```
strconv.ParseFloat: parsing "30s": invalid syntax
```

### 4. Label Inheritance

Use the wildcard pattern to inherit all labels from resource metadata:

```yaml
labelsFromPath:
  name: [metadata, name]
  "*": [metadata, labels]  # Inherits ALL labels from metadata.labels
```

This automatically adds labels like `environment`, `team`, `managed-by` to all metrics.

## Setup Guide

### Step 1: Configure KSM Custom Resource State

Edit your `values.yaml` to enable customResourceState. The configuration structure depends on which chart you're using:

**For nri-bundle chart** (most common):
```yaml
ksm:
  enabled: true
  customResourceState:
    enabled: true
    config:
      kind: CustomResourceStateMetrics
      spec:
        resources:
          - groupVersionKind:
              group: <crd-group>      # e.g., karpenter.sh
              version: <version>       # e.g., v1beta1
              kind: <kind>             # e.g., NodePool
            labelsFromPath:
              name: [metadata, name]
              "*": [metadata, labels]
            metrics:
              # Define your metrics here
```

**For standalone newrelic-infrastructure chart**:
```yaml
kube-state-metrics:
  customResourceState:
    enabled: true
    config:
      kind: CustomResourceStateMetrics
      spec:
        resources:
          - groupVersionKind:
              group: <crd-group>
              version: <version>
              kind: <kind>
            labelsFromPath:
              name: [metadata, name]
              "*": [metadata, labels]
            metrics:
              # Define your metrics here
```

**Note**: This guide uses the `kube-state-metrics:` syntax for clarity in examples. Adjust the top-level key based on your chart.

### Step 2: Create RBAC Permissions

Create a ClusterRole and ClusterRoleBinding for KSM:

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ksm-<resource>-reader
rules:
  - apiGroups: ["<crd-group>"]
    resources: ["<resource-plural>"]
    verbs: ["list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ksm-<resource>-reader
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ksm-<resource>-reader
subjects:
  - kind: ServiceAccount
    name: <release-name>-kube-state-metrics  # Typically matches your Helm release name
    namespace: <namespace>
```

**Important**: After applying RBAC changes, KSM may require a restart or a resync period (typically a few minutes) before it starts watching the new resources. If metrics don't appear immediately, restart the KSM pod:
```bash
kubectl rollout restart deployment/<release-name>-kube-state-metrics -n <namespace>
```

### Step 3: Deploy and Verify

```bash
# Deploy with Helm
helm upgrade --install newrelic-bundle newrelic/nri-bundle \
  -f values.yaml

# Verify KSM is exposing CRD metrics
kubectl port-forward -n newrelic svc/<release-name>-kube-state-metrics 8080:8080
curl localhost:8080/metrics | grep kube_customresource

# Check KSM logs for errors
kubectl logs -n newrelic deployment/<release-name>-kube-state-metrics
```

### Step 4: Query Metrics in New Relic

Once deployed, CRD metrics appear in New Relic's Metrics table with the `kube_customresource_` prefix. Query them using NRQL:

**Basic Query**:
```sql
FROM Metric
SELECT latest(kube_customresource_nodepool_nodes_count)
FACET name, environment
TIMESERIES
```

**List All CRD Metrics**:
```sql
FROM Metric
SELECT uniques(metricName)
WHERE metricName LIKE 'kube_customresource_%'
```

**Query with Labels**:
```sql
FROM Metric
SELECT latest(kube_customresource_nodepool_limit_cpu)
WHERE team = 'platform'
  AND environment = 'production'
FACET name
TIMESERIES
```

**Note**: All labels from `labelsFromPath` (including inherited labels via `"*"`) are available as dimensional attributes for filtering and faceting.

## Popular CRD Examples

**Important**: Only the Karpenter NodePool example has been tested and verified. Other examples are provided as starting points and should be validated against your specific CRD versions. Always verify field paths using `kubectl get <resource> <name> -o yaml` before deploying.

### 1. Karpenter NodePool (✅ Tested and Verified)

**Use Case**: Monitor Karpenter node provisioning and capacity management

**CRD Group**: `karpenter.sh/v1beta1`

**⚠️ Version Note**: This example uses `v1beta1`, which is supported but deprecated in Karpenter v1.1+. For newer Karpenter versions, update to `v1` and verify field paths as the structure may have changed (e.g., `spec.disruption.consolidationPolicy` path).

**Configuration**:
```yaml
kube-state-metrics:
  customResourceState:
    enabled: true
    config:
      kind: CustomResourceStateMetrics
      spec:
        resources:
          - groupVersionKind:
              group: karpenter.sh
              version: v1beta1
              kind: NodePool
            labelsFromPath:
              name: [metadata, name]
              "*": [metadata, labels]
            metrics:
              - name: "nodepool_consolidation_policy"
                help: "Consolidation policy setting"
                each:
                  type: StateSet
                  stateSet:
                    labelName: policy
                    path: [spec, disruption, consolidationPolicy]
                    list:
                      - "WhenEmpty"
                      - "WhenUnderutilized"
              - name: "nodepool_limit_cpu"
                help: "CPU limit for the NodePool"
                each:
                  type: Gauge
                  gauge:
                    path: [spec, limits, cpu]
              - name: "nodepool_limit_memory"
                help: "Memory limit for the NodePool"
                each:
                  type: Gauge
                  gauge:
                    path: [spec, limits, memory]
              - name: "nodepool_nodes_count"
                help: "Number of nodes managed by this NodePool"
                each:
                  type: Gauge
                  gauge:
                    path: [status, resources, nodes]
              - name: "nodepool_pods_count"
                help: "Number of pods on nodes from this NodePool"
                each:
                  type: Gauge
                  gauge:
                    path: [status, resources, pods]
              - name: "nodepool_conditions"
                help: "NodePool conditions"
                each:
                  type: Info
                  info:
                    labelsFromPath:
                      type: [type]
                      status: [status]
                    path: [status, conditions]
```

**RBAC** (requires both ClusterRole and ClusterRoleBinding):
```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ksm-nodepool-reader
rules:
  - apiGroups: ["karpenter.sh"]
    resources: ["nodepools"]
    verbs: ["list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ksm-nodepool-reader
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ksm-nodepool-reader
subjects:
  - kind: ServiceAccount
    name: <release-name>-kube-state-metrics
    namespace: <namespace>
```

**Expected Metrics** (simplified - actual metrics will include additional labels):
```prometheus
kube_customresource_nodepool_consolidation_policy{name="default",policy="WhenUnderutilized",...} 1
kube_customresource_nodepool_limit_cpu{name="default",...} 1000
kube_customresource_nodepool_limit_memory{name="default",...} 1.073741824e+12
kube_customresource_nodepool_nodes_count{name="default",...} 3
kube_customresource_nodepool_pods_count{name="default",...} 25
kube_customresource_nodepool_conditions{name="default",type="Ready",status="True",...} 1
```

**Note**: Actual metrics include additional labels like `customresource_group`, `customresource_kind`, `customresource_version`, and any labels inherited from the resource metadata (e.g., `environment`, `team`, `managed_by`).

### 2. Argo Rollouts

**Use Case**: Monitor progressive delivery rollout status

**CRD Group**: `argoproj.io/v1alpha1`

**⚠️ Note**: This configuration is theoretical and not tested. Verify field paths against your Argo Rollouts version.

**Configuration**:
```yaml
- groupVersionKind:
    group: argoproj.io
    version: v1alpha1
    kind: Rollout
  labelsFromPath:
    name: [metadata, name]
    namespace: [metadata, namespace]
    "*": [metadata, labels]
  metrics:
    - name: "rollout_replicas"
      help: "Number of desired replicas"
      each:
        type: Gauge
        gauge:
          path: [spec, replicas]
    - name: "rollout_replicas_updated"
      help: "Number of updated replicas"
      each:
        type: Gauge
        gauge:
          path: [status, updatedReplicas]
    - name: "rollout_replicas_ready"
      help: "Number of ready replicas"
      each:
        type: Gauge
        gauge:
          path: [status, readyReplicas]
    - name: "rollout_replicas_available"
      help: "Number of available replicas"
      each:
        type: Gauge
        gauge:
          path: [status, availableReplicas]
    - name: "rollout_phase"
      help: "Current rollout phase"
      each:
        type: StateSet
        stateSet:
          labelName: phase
          path: [status, phase]
          list:
            - "Progressing"
            - "Paused"
            - "Healthy"
            - "Degraded"
    - name: "rollout_conditions"
      help: "Rollout conditions"
      each:
        type: Info
        info:
          labelsFromPath:
            type: [type]
            status: [status]
            reason: [reason]
          path: [status, conditions]
```

**RBAC** (ClusterRole only shown - add ClusterRoleBinding as shown in Step 2):
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ksm-rollout-reader
rules:
  - apiGroups: ["argoproj.io"]
    resources: ["rollouts"]
    verbs: ["list", "watch"]
```

### 3. cert-manager Certificate

**Use Case**: Monitor SSL/TLS certificate status and expiry

**CRD Group**: `cert-manager.io/v1`

**⚠️ Note**: This configuration is theoretical and not tested. Verify field paths against your cert-manager version.

**Configuration**:
```yaml
- groupVersionKind:
    group: cert-manager.io
    version: v1
    kind: Certificate
  labelsFromPath:
    name: [metadata, name]
    namespace: [metadata, namespace]
    "*": [metadata, labels]
  metrics:
    - name: "certificate_info"
      help: "Certificate information"
      each:
        type: Info
        info:
          labelsFromPath:
            issuer_name: [spec, issuerRef, name]
            issuer_kind: [spec, issuerRef, kind]
            secret_name: [spec, secretName]
            common_name: [spec, commonName]
    - name: "certificate_renewal_time"
      help: "Certificate renewal time as info label"
      each:
        type: Info
        info:
          labelsFromPath:
            renewal_time: [status, renewalTime]
            not_after: [status, notAfter]
    - name: "certificate_conditions"
      help: "Certificate conditions"
      each:
        type: Info
        info:
          labelsFromPath:
            type: [type]
            status: [status]
            reason: [reason]
            message: [message]
          path: [status, conditions]
```

**Notes**:
- Timestamp fields like `notAfter` are RFC3339 strings. With **KSM v2.10+**, you may be able to use Gauge type for automatic conversion to Unix timestamp. For older KSM versions, use Info type.
- To filter specific condition types, iterate over all conditions and filter in your queries
- Verify the exact field paths with: `kubectl get certificate <name> -o yaml`
- For production certificate monitoring, consider the cert-manager Prometheus exporter which provides additional metrics

**RBAC** (ClusterRole only shown - add ClusterRoleBinding as shown in Step 2):
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ksm-certificate-reader
rules:
  - apiGroups: ["cert-manager.io"]
    resources: ["certificates"]
    verbs: ["list", "watch"]
```

### 4. External Secrets

**Use Case**: Monitor external secret synchronization status

**CRD Group**: `external-secrets.io/v1beta1`

**⚠️ Note**: This configuration is theoretical and not tested. Verify field paths against your External Secrets Operator version.

**Configuration**:
```yaml
- groupVersionKind:
    group: external-secrets.io
    version: v1beta1
    kind: ExternalSecret
  labelsFromPath:
    name: [metadata, name]
    namespace: [metadata, namespace]
    "*": [metadata, labels]
  metrics:
    - name: "externalsecret_info"
      help: "External secret information"
      each:
        type: Info
        info:
          labelsFromPath:
            store_name: [spec, secretStoreRef, name]
            store_kind: [spec, secretStoreRef, kind]
            target_name: [spec, target, name]
            refresh_interval: [spec, refreshInterval]
    - name: "externalsecret_conditions"
      help: "External secret conditions"
      each:
        type: Info
        info:
          labelsFromPath:
            type: [type]
            status: [status]
            reason: [reason]
            message: [message]
          path: [status, conditions]
```

**RBAC** (ClusterRole only shown - add ClusterRoleBinding as shown in Step 2):
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ksm-externalsecret-reader
rules:
  - apiGroups: ["external-secrets.io"]
    resources: ["externalsecrets"]
    verbs: ["list", "watch"]
```

## Complete Helm Values Example

Here's a complete `values.yaml` example for monitoring Karpenter NodePool (tested and verified):

```yaml
# values.yaml
kube-state-metrics:
  customResourceState:
    enabled: true
    config:
      kind: CustomResourceStateMetrics
      spec:
        resources:
          # Karpenter NodePool (tested configuration)
          - groupVersionKind:
              group: karpenter.sh
              version: v1beta1
              kind: NodePool
            labelsFromPath:
              name: [metadata, name]
              "*": [metadata, labels]
            metrics:
              - name: "nodepool_consolidation_policy"
                help: "Consolidation policy setting"
                each:
                  type: StateSet
                  stateSet:
                    labelName: policy
                    path: [spec, disruption, consolidationPolicy]
                    list:
                      - "WhenEmpty"
                      - "WhenUnderutilized"
              - name: "nodepool_limit_cpu"
                help: "CPU limit for the NodePool"
                each:
                  type: Gauge
                  gauge:
                    path: [spec, limits, cpu]
              - name: "nodepool_limit_memory"
                help: "Memory limit for the NodePool"
                each:
                  type: Gauge
                  gauge:
                    path: [spec, limits, memory]
              - name: "nodepool_nodes_count"
                help: "Number of nodes managed by this NodePool"
                each:
                  type: Gauge
                  gauge:
                    path: [status, resources, nodes]
              - name: "nodepool_pods_count"
                help: "Number of pods on nodes from this NodePool"
                each:
                  type: Gauge
                  gauge:
                    path: [status, resources, pods]
              - name: "nodepool_conditions"
                help: "NodePool conditions"
                each:
                  type: Info
                  info:
                    labelsFromPath:
                      type: [type]
                      status: [status]
                    path: [status, conditions]
```

**RBAC Configuration**: Create separate RBAC resources (see Step 2 above for template). Note that RBAC must be created separately - it cannot be configured via `values.yaml` in most chart configurations.

## Metric Types Reference

KSM supports several metric types for custom resources:

### Gauge
Numeric values that can go up or down (counts, sizes, limits).

```yaml
- name: "my_gauge_metric"
  each:
    type: Gauge
    gauge:
      path: [spec, fieldName]  # Must be numeric
```

### StateSet
Represents a field that can have one of multiple string values.

```yaml
- name: "my_state_metric"
  each:
    type: StateSet
    stateSet:
      labelName: state
      path: [status, phase]
      list:
        - "Running"
        - "Pending"
        - "Failed"
```

Produces metrics like:
```prometheus
kube_customresource_my_state_metric{state="Running"} 1
kube_customresource_my_state_metric{state="Pending"} 0
kube_customresource_my_state_metric{state="Failed"} 0
```

### Info
Exposes multiple string fields as labels (no numeric value).

```yaml
- name: "my_info_metric"
  each:
    type: Info
    info:
      labelsFromPath:
        field1: [spec, field1]
        field2: [status, field2]
```

Produces:
```prometheus
kube_customresource_my_info_metric{field1="value1",field2="value2"} 1
```

### Info for Arrays
Use `path` to iterate over array fields like conditions.

```yaml
- name: "my_conditions"
  each:
    type: Info
    info:
      labelsFromPath:
        type: [type]
        status: [status]
        reason: [reason]
      path: [status, conditions]
```

## Troubleshooting

### No Metrics Appearing

**Check KSM is running**:
```bash
kubectl get pods -n newrelic -l app.kubernetes.io/name=kube-state-metrics
```

**Check KSM logs for errors**:
```bash
kubectl logs -n newrelic deployment/<release>-kube-state-metrics
```

**Verify RBAC permissions**:
```bash
# Check if KSM can list your CRD
kubectl auth can-i list <resource> \
  --as=system:serviceaccount:<namespace>:<release>-kube-state-metrics
```

**Test metrics endpoint directly**:
```bash
kubectl port-forward -n newrelic svc/<release>-kube-state-metrics 8080:8080
curl localhost:8080/metrics | grep kube_customresource
```

### RBAC Errors in Logs

**Error**: `is forbidden: User "system:serviceaccount:..." cannot list resource`

**Solution**: Ensure ClusterRole includes the CRD's API group and resource plural name:
```yaml
rules:
  - apiGroups: ["<crd-group>"]
    resources: ["<plural-name>"]  # e.g., "nodepools", not "NodePool"
    verbs: ["list", "watch"]
```

### Parsing Errors

**Error**: `strconv.ParseFloat: parsing "30s": invalid syntax`

**Solution**: Don't use Gauge for duration strings. Either:
- Use Info type to expose as label
- Convert to numeric seconds in your CRD controller
- Omit the field from monitoring

### Metrics Not Updating

**Issue**: Metrics show old values after resource updates

**Solution**: KSM caches resources. Check:
- KSM is receiving watch events (check logs)
- RBAC includes "watch" permission (not just "list")
- No network policy blocking KSM access to API server

## Scale and Performance Considerations

### Memory Usage

Monitoring CRDs with many instances can significantly increase KSM's memory footprint. Each CRD resource monitored adds to KSM's memory consumption.

**Warning Signs**:
- Monitoring thousands of Certificate resources
- High cardinality labels (unique values per resource)
- Many metrics per CRD (10+ metrics per resource type)

**Mitigation Strategies**:

1. **Use metric-allowlist** to limit which metrics are exposed:
   ```yaml
   kube-state-metrics:
     metricAllowlist:
       - kube_customresource_nodepool_*
       - kube_customresource_rollout_replicas*
   ```

2. **Monitor specific namespaces only** if applicable:
   ```yaml
   kube-state-metrics:
     namespaces: "production,staging"
   ```

3. **Increase KSM resources** if monitoring many CRDs:
   ```yaml
   kube-state-metrics:
     resources:
       requests:
         memory: 512Mi
         cpu: 200m
       limits:
         memory: 1Gi
         cpu: 500m
   ```

4. **Use sampling** for high-volume CRDs - monitor a representative subset rather than all instances.

### Cardinality Considerations

Be cautious with labels that have high cardinality (many unique values):
- Avoid exposing UUIDs, timestamps, or IP addresses as labels
- Limit inherited labels using explicit `labelsFromPath` instead of `"*"`
- Use Info metrics sparingly for high-cardinality data

## Best Practices

1. **Verify Field Paths First**: Before configuring a CRD, inspect an actual resource:
   ```bash
   kubectl get <resource-type> <name> -o yaml
   ```
   This shows the exact field structure for your CRD version.

2. **Start Small**: Begin with 2-3 critical metrics per CRD, expand as needed

3. **Use Label Inheritance**: Always include `"*": [metadata, labels]` to inherit custom labels

4. **Monitor Status First**: Status fields are more useful than spec fields for alerting

5. **Avoid High-Cardinality Labels**: Don't include UUIDs or timestamps as labels

6. **Test Locally First**: Use port-forward to verify metrics before deploying:
   ```bash
   kubectl port-forward -n newrelic svc/<release>-kube-state-metrics 8080:8080
   curl localhost:8080/metrics | grep kube_customresource
   ```

7. **Check KSM Logs**: After deployment, verify KSM successfully registered your metrics:
   ```bash
   kubectl logs -n newrelic deployment/<release>-kube-state-metrics | grep "Custom resource"
   ```

8. **Match Field Types**:
   - Numeric fields → Gauge
   - String enums → StateSet
   - Multiple string fields or timestamps → Info
   - Arrays/conditions → Info with path

9. **Document Your Metrics**: Add meaningful `help` text to each metric

10. **Version Your CRDs**: Include version in groupVersionKind for API stability

## References

- [KSM Custom Resource State Documentation](https://github.com/kubernetes/kube-state-metrics/blob/main/docs/customresourcestate-metrics.md)
- [KSM Metric Types](https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/README.md)
- [Prometheus Metric Types](https://prometheus.io/docs/concepts/metric_types/)
- [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [New Relic Kubernetes Integration](https://docs.newrelic.com/docs/kubernetes-pixie/kubernetes-integration/get-started/introduction-kubernetes-integration/)
