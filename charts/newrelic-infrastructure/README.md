# newrelic-infrastructure

![Version: 3.3.2](https://img.shields.io/badge/Version-3.3.2-informational?style=flat-square) ![AppVersion: 3.1.1](https://img.shields.io/badge/AppVersion-3.1.1-informational?style=flat-square)

A Helm chart to deploy the New Relic Kubernetes monitoring solution

**Homepage:** <https://docs.newrelic.com/docs/kubernetes-pixie/kubernetes-integration/get-started/introduction-kubernetes-integration/>

# Helm installation

You can install this chart using [`nri-bundle`](https://github.com/newrelic/helm-charts/tree/master/charts/nri-bundle) located in the
[helm-charts repository](https://github.com/newrelic/helm-charts) or directly from this repository by adding this Helm repository:

```shell
helm repo add nri-kube-events https://newrelic.github.io/nri-kube-events
helm upgrade --install nri-kube-events/nri-kube-events -f your-custom-values.yaml
```

## Source Code

* <https://github.com/newrelic/nri-kubernetes/>
* <https://github.com/newrelic/nri-kubernetes/tree/master/charts/newrelic-infrastructure>
* <https://github.com/newrelic/infrastructure-agent/>

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| https://helm-charts.newrelic.com | common-library | 1.0.2 |

## Values managed globally

This chart implements the [New Relic's common Helm library](https://github.com/newrelic/helm-charts/tree/master/library/common-library) which
means that is has a seamless UX between things that are configurable across different Helm charts. So there are behaviours that could be
changed globally if you install this chart from `nri-bundle` or your own umbrella chart.

A really broad list of global managed values are `affinity`, `nodeSelector`, `tolerations`, `proxy` and many more.

For more information go to the [user's guide of the common library](https://github.com/newrelic/helm-charts/blob/master/library/common-library/README.md)

## Chart particularities

### Low data mode
There are two mechanisms to reduce the amount of data that this integration sends to New Relic. See this snippet from the `values.yaml` file:
```yaml
common:
  config:
    interval: 15s

lowDataMode: false
```

The `lowDataMode` toggle is the simplest way amd setting it to `true` changes the default scrape interval from 15 seconds (the default) to 30 seconds.

If you need for some reason to fine tune the amount of seconds you can use `common.config.interval` directly. If you take a look to the `values.yaml`
file, the value there is `nil`. If set any value there, the `lowDataMode` toggle is ignored and this value is issued.

Setting this interval above 40 seconds can make you experience issues with the Kubernetes Cluster Explorer so this chart limits setting the interval
inside the range of 10 to 40 seconds.

### Affinities and tolerations

The New Relic common library allows to set affinities, tolerations and nodeSelectors in globally with the resto of the integrations and products. This
integration in particular has affinities and tolerations set to be able to schedule pods in nodes that are tainted as master nodes and to schedule a
pod near the KSM to reduce the inter-node traffic.

Take a look to the [`values.yaml`](values.yaml) so see how to configure them if you are having problems scheduling pods where you want to.

### `hostNetwork` toggle

In versions below v3, changing the `privileged` mode affected the `hostNetwork`. We changed this behavior and now you can set pods to use `hostNetwork`
using the corresponding [flags from the common library](https://github.com/newrelic/helm-charts/blob/master/library/common-library/README.md)
(`.global.hostNetwork` and `.hostNetwork`) but the component that scrapers data from the control plane has always set `hostNetwork` to true (Look in the
[`values.yaml`](values.yaml) for `controlPlane.hostNetwork: true`)

This is because the most common configuration of the control plane components is to be configured to listen only to `localhost`.

If your cluster security policy does not allow to use `hostNetwork`, you can disable it control plane monitoring by setting `controlPlane.enabled` to
`false.`

### `privileged` toggle

The default value for `privileged` [from the common library](https://github.com/newrelic/helm-charts/blob/master/library/common-library/README.md) is
`false`.

In this chart it is set to `true` (Look in the [`values.yaml`](values.yaml) for `privileged: true`) because it set `kubelet` pods (the ones that scrape
metrics from the hosts itself) into privileged mode so it can fetch more fine-grained cpu, memory, process and network metrics for your nodes.

If your cluster security policy does not allow to to have `privileged` in your pod' security context, you can disable it by setting `privileged` to
`false.`

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| common | object | See `values.yaml` | Config that applies to all instances of the solution: kubelet, ksm, control plane and sidecars. |
| common.agentConfig | object | `{}` | Config for the Infrastructure agent. Will be used by the forwarder sidecars and the agent running integrations. See: https://docs.newrelic.com/docs/infrastructure/install-infrastructure-agent/configuration/infrastructure-agent-configuration-settings/ |
| common.config.interval | duration | `15s` (See [Low data mode](README.md#low-data-mode)) | Intervals larger than 40s are not supported and will cause the NR UI to not behave properly. Any non-nil value will override the `lowDataMode` default. |
| containerSecurityContext | string | `nil` | This is still to be decided in a follow-up PR regarding privileged mode and defaults changing in an evil way |
| controlPlane | object | See `values.yaml` | Configuration for the control plane scraper. |
| controlPlane.affinity | object | Deployed only in master nodes. | Affinity for the control plane DaemonSet. |
| controlPlane.config.apiServer | object | Common settings for most K8s distributions. | API Server monitoring configuration |
| controlPlane.config.apiServer.enabled | bool | `true` | Enable API Server monitoring |
| controlPlane.config.controllerManager | object | Common settings for most K8s distributions. | Controller manager monitoring configuration |
| controlPlane.config.controllerManager.enabled | bool | `true` | Enable controller manager monitoring. |
| controlPlane.config.etcd | object | Common settings for most K8s distributions. | ETCD monitoring configuration |
| controlPlane.config.etcd.enabled | bool | `true` | Enable etcd monitoring. Might require manual configuration in some environments. |
| controlPlane.config.retries | int | `3` | Number of retries after timeout expired |
| controlPlane.config.scheduler | object | Common settings for most K8s distributions. | Scheduler monitoring configuration |
| controlPlane.config.scheduler.enabled | bool | `true` | Enable scheduler monitoring. |
| controlPlane.config.timeout | string | `"10s"` | Timeout for the Kubernetes APIs contacted by the integration |
| controlPlane.enabled | bool | `true` | Deploy control plane monitoring component. |
| controlPlane.hostNetwork | bool | `true` | Run Control Plane scraper with `hostNetwork`. `hostNetwork` is required for most control plane configurations, as they only accept connections from localhost. |
| controlPlane.kind | string | `"DaemonSet"` | How to deploy the control plane scraper. If autodiscovery is in use, it should be `DaemonSet`. Advanced users using static endpoints set this to `Deployment` to avoid reporting metrics twice. |
| customAttributes | object | `{}` | Custom attributes to be added to the data reported by all integrations reporting in the cluster. |
| dnsConfig | object | `{}` | Pod dns configuration Ref: https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-dns-config |
| fullnameOverride | string | `""` | Override the full name of the release |
| images | object | See `values.yaml` | Images used by the chart for the integration and agents. |
| images.agent | object | See `values.yaml` | Image for the New Relic Infrastructure Agent plus integrations. |
| images.forwarder | object | See `values.yaml` | Image for the New Relic Infrastructure Agent sidecar. |
| images.integration | object | See `values.yaml` | Image for the New Relic Kubernetes integration. |
| images.pullSecrets | list | `[]` | The secrets that are needed to pull images from a custom registry. |
| integrations | object | `{}` | Config files for other New Relic integrations that should run in this cluster. |
| ksm | object | See `values.yaml` | Configuration for the Deployment that collects state metrics from KSM (kube-state-metrics). |
| ksm.affinity | object | Deployed in the same node as KSM | Affinity for the control plane DaemonSet. |
| ksm.config.retries | int | `3` | Number of retries after timeout expired |
| ksm.config.timeout | string | `"10s"` | Timeout for the ksm API contacted by the integration |
| ksm.enabled | bool | `true` | Enable cluster state monitoring. Advanced users only. Setting this to `false` is not supported and will break the New Relic experience. |
| ksm.resources | object | 100m/150M -/850M | Resources for the KSM scraper pod. Keep in mind that sharding is not supported at the moment, so memory usage for this component ramps up quickly on large clusters. |
| ksm.tolerations | list | Schedules in all tainted nodes | Affinity for the control plane DaemonSet. |
| kubelet | object | See `values.yaml` | Configuration for the DaemonSet that collects metrics from the Kubelet. |
| kubelet.config.retries | int | `3` | Number of retries after timeout expired |
| kubelet.config.timeout | string | `"10s"` | Timeout for the kubelet APIs contacted by the integration |
| kubelet.enabled | bool | `true` | Enable kubelet monitoring. Advanced users only. Setting this to `false` is not supported and will break the New Relic experience. |
| kubelet.tolerations | list | Schedules in all tainted nodes | Affinity for the control plane DaemonSet. |
| lowDataMode | bool | `false` (See [Low data mode](README.md#low-data-mode)) | Send less data by incrementing the interval from `15s` (the default when `lowDataMode` is `false` or `nil`) to `30s`. Non-nil values of `common.config.interval` will override this value. |
| nameOverride | string | `""` | Override the name of the chart |
| podAnnotations | object | `{}` | Annotations to be added to all pods created by the integration. |
| podLabels | object | `{}` | Labels to be added to all pods created by the integration. |
| podSecurityContext | string | `nil` | This is still to be decided in a follow-up PR regarding privileged mode and defaults changing in an evil way |
| priorityClassName | string | `nil` | Pod scheduling priority Ref: https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/ |
| privileged | bool | `true` | Run the integration with full access to the host filesystem and network. Running in this mode allows reporting fine-grained cpu, memory, process and network metrics for your nodes. |
| rbac | object | `{"create":true,"pspEnabled":false}` | Settings controlling RBAC objects creation. |
| rbac.create | bool | `true` | Whether the chart should automatically create the RBAC objects required to run. |
| rbac.pspEnabled | bool | `false` | Whether the chart should create Pod Security Policy objects. |
| serviceAccount | object | See `values.yaml` | Settings controlling ServiceAccount creation. |
| serviceAccount.create | bool | `true` | Whether the chart should automatically create the ServiceAccount objects required to run. |
| updateStrategy | object | See `values.yaml` | Update strategy for the DaemonSets deployed. |
| verboseLog | bool | `false` | Enable verbose logging for all components. |

## Maintainers

* [alvarocabanas](https://github.com/alvarocabanas)
* [carlossscastro](https://github.com/carlossscastro)
* [sigilioso](https://github.com/sigilioso)
* [gsanchezgavier](https://github.com/gsanchezgavier)
* [kang-makes](https://github.com/kang-makes)
* [marcsanmi](https://github.com/marcsanmi)
* [paologallinaharbur](https://github.com/paologallinaharbur)
* [roobre](https://github.com/roobre)

## Past Contributors

Previous iterations of this chart started as a community project in the [stable Helm chart repository](github.com/helm/charts/). New Relic is very thankful for all the 15+ community members that contributed and helped maintain the chart there over the years:

* coreypobrien
* sstarcher
* jmccarty3
* slayerjain
* ryanhope2
* rk295
* michaelajr
* isindir
* idirouhab
* ismferd
* enver
* diclophis
* jeffdesc
* costimuraru
* verwilst
* ezelenka
