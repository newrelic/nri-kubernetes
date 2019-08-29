# Change Log

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## Unreleased 

## 1.10.0
### Added
- Node labes are added to the `K8sNodeSample`. They are retrieved from the k8s
  api and cached.
- The integration now uses the infrastructure agent v1.5.51. For more
  information refer to the [infrastructure agent release notes](https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/)
  between versions v1.5.31 and v1.5.51.

## 1.9.5
### Changed

- The integration now uses the Infrastructu Agent v1.5.31. The biggest changes were major improvements to logging and
  to the StorageSampler. For more information refer to the [infrastructure agent release notes](https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/) between versions v1.3.18 and v1.5.31.

## 1.9.4
### Fixed

- No code changes have been made. This fixes a regression at Docker image level related to https://github.com/moby/moby/issues/35443.

## 1.9.3
### Added
- Support for discovering KSMs when running with the label `app.kubernetes.io/name`.

## 1.9.2
### Fixed
- No code changes has been made. The fix is at docker image level. We got affected by https://github.com/moby/moby/issues/35443.

## 1.9.1
### Fixed

- The unprivileged integration runs always as `nri-agent` user. Fixes https://github.com/kubernetes/kubernetes/issues/78308.
- Infrastructure agent is now behaving in secure-forwarder mode.
- Autodiscovery cache directory permissions got changed from 644 to 744 in order to let the nri-agent user write inside.

## 1.9.0

### Changed

- The integraion now uses the infrastructure agent v1.3.18 instead of 1.1.14. Refer to the
  [infrastructure agent release notes](https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/new-relic-infrastructure-agent-1318)
  for more information about all the changes from this upgrade.

## 1.8.0

### Added
- The integration reports the name of the cluster as Infrastructure inventory.
- The integration reports a new event type `K8sClusterSample`. At this moment,
  these events contain only the cluster name as an attribute.

## 1.7.0

### Added
- Support for kube-state-metrics v1.5.
- Pod's status reason and status message are now sent in the `K8sPodSample` as `reason` and `message` fields.
- Container's `memory_working_set_bytes` is now sent in the `K8sContainerSample` as `workingSetBytes`.

### Changed
- Always request metrics from kube-state-metrics in the text format. In kube-state-metrics v1.5 this is the default
regardless of the format requested.

## 1.6.0
### Added
- `namespaceName` metric attribute was added to all the samples where `namespace` attribute is present.

### Deprecated
- `namespace` metric attribute will be removed soon. Please use `namespaceName` from now on.

## 1.5.0

### Changed
- Due to an issue in Kubelet, we stopped reporting the Status of static pods. See https://github.com/kubernetes/kubernetes/issues/61717.

## 1.4.0

### Changed
- Update base image in the dockerfile to use latest newrelic/infrastructure
  version: 0.0.62 (Infrastructure agent v1.1.14, released at: 2018-12-20)

## 1.3.1

### Added
- Add clusterName custom attribute to manifest file. This helps users correlate
  Kubernetes integration data with Infrastructure agent data.

### Changed
- `KUBE_STATE_METRICS_URL` environment variable can be specified containing only host & port
  or it can be the complete URL including also the `/metrics` path (ex:
  `http://my-service.my-ns.svc.cluster.local:8080/metrics`).

### Fixed
- Fix how the usage percentage is calculated for container filesystem metrics.
- Fix how the usage percentage is calculated for volumes.

## 1.3.0

### Added
- Add metrics for volumes (persistent and non-persistent volumes).
- Add container filesystem metrics.

## 1.2.0

### Added
- Add `reason` metric for terminated containers

## 1.1.0

### Added
- Support for specifying the K8s API Host and Port by setting the `KUBERNETES_SERVICE_HOST` and `KUBERNETES_SERVICE_PORT` env vars.

### Changed
- Improve readability of log messages, when verbose mode is enabled.

### Fixed
- Kubernetes API url discovery failed sometimes giving errors like "error trying to connect to...". Now this should be fixed.

## 1.0.0

### Changed
- The agent tag installed within the integration docker image is now fixed to 0.0.24.

## 1.0.0-beta2.4

### Added
- Add `hostNetwork: true` option and the required dns policy to daemonset file. This is a requirement for the Infrastructure Agent to report the proper hostname in New Relic.

### Changed
- Update newrelic-infra.yaml to force our objects to be deployed in `default` namespace.
- Add NoExecute toleration ensuring that our pod is being deployed when the NoExecute node taint is set.

### Fixed
- Add missing metric: `podsMaxUnavailable` for deployment
- Fix some of the metrics for pods in pending status
  - Adding missing metrics: `startTime`, `isReady`
  - Unifying `isScheduled` and `isReady` to be reported as `1` and `0` for `true` and `false` respectively.
- Fix pod metrics (`status` and `isReady`): non-scheduled or pending pods were not reported correctly.

## 1.0.0-beta2.3

### Added
- Add configurable flag for kube-state-metrics endpoint (only HTTP).
- Add additional label `app` for discovering kube-state-metrics endpoint.

### Changed
- Kubelet discovery process fetches now the nodeName directly from the spec using downward API.

## 1.0.0-beta2.2

### Fixed
- Fix bug in error handling where recoverable errors made the integration to panic.

## 1.0.0-beta2.1

### Added
- Allow direct connection to cAdvisor by specifying the port.

### Fixed
- Call to CAdvisor was failing when Kubelet was secure.

## 1.0.0-beta2.0

### Added
- nodes/metrics resource was added to the newrelic cluster role.

### Changed
- CAdvisor call is now bypassing Kubelet endpoint talking then directoy to CAdvisor port

## 1.0.0-beta1.0

Initial public beta release.

## 1.0.0-alpha5.1

### Changed
- TransformFunc now handles errors.
- Add checks for missing data coming from kube-state-metrics.
- Boolean values have changed from `"true"` and `"false"` to `1` and `0` respectively from the following metrics:
  1. isReady and isScheduled for pods.
  2. isReady for containers.
- Update metrics
  1. `errorCountPerSecond` to `errorsPerSecond` for pods and nodes.
  2. `usageCoreSeconds` to `cpuUsedCoreMilliseconds` for nodes.
  3. `memoryMajorPageFaults` to `memoryMajorPageFaultsPerSecond` for nodes.

### Fixed
- Calculate properly RATE metrics.

## 1.0.0-alpha5

### Added
- TypeGenerator for entities.
- Caching discovered endpoints on disk.
- Implementation of Time-To-Live (TTL) cache expiry functionality.
- Added the concept of Leader and Follower roles.
  - Leader represents the node where Kube State Metrics is installed (so only 1 by cluster).
  - Follower represents any other node.
- Both Follower and Leader call kubelet /pods endpoint in order to get metrics that were previously fetched from KSM.
- Fetch metrics from KSM about pods with status "Pending".
- Prometheus TextToProtoHandleFunc as http.HandlerFunc.
  Useful for serving a Prometheus payload in protobuf format from a plain text reader.
- Both Follower and Leader call kubelet /metrics/cadvisor endpoint in order to fill some missing metrics coming from Kubelet.

### Changed
- Rename `endpoints` package to `client` package.
- Moved a bunch of functions related to `Prometheus` from `ksm` package to `prometheus` one.
- Renamed the recently moved `Prometheus` functions. Removed **Prometheus** word as it is considered redundant.
- Containers objects reported as their own entities (not as part of pod entities).
- NewRelic infra Daemonset updateStrategy set to RollingUpdate in newrelic-infra.yaml.
- Prometheus CounterValue type changed from uint to float64.
- Change our daemonset file to deploy the integration in "default" namespace.
- Prometheus queries now require to use an operator.
- Prometheus Do method now requires a metrics endpoint.

### Removed
- Follower does not call KSM endpoints anymore.
- Config package with default unknown namespace value
- Removed legacy Kubernetes spec files.

### Fixed
- Replace `log.Fatal()` by `log.Panic()` in order to call all defer statements.
- Skip missing data from /stats/summary endpoint, instead of reporting them as zero values.
- Entities not reported in case of problem with setting their name or type.

## 1.0.0-alpha4

### Added
- Adding node metrics. Data is fetched from Kubelet and kube-state-metrics.
- Adding toleration for the "NoSchedule" taint, so the integration is deployed on all nodes.
- Adding new autodiscovery flow with authentication and authorization mechanisms.

### Removed
- Custom arguments for kubelet and kube-state-metrics endpoints.

### Fixed
- Integration stops on KSM or Kubelet connection error, instead of continuing.

## 1.0.0-alpha3

### Changed
- `updatedAt` metric was renamed to `podsUpdated`.
- `cpuUsedCores` has been divided by 10^9, to show actual cores instead of nanocores.
- Update configurable timeout flag using it to connect to kubelet and kube-state-metrics.

### Fixed
- Fix debug log level when verbose. Some parts of the code didn't log debug information.

## 1.0.0-alpha2

### Added
- Metrics for unscheduled Pods.

### Fixed
- Fix format of inherited labels. Remove unnecessary prefix `label_` included by kube-state-metrics.
- Fix labels inheritance. Labels weren't propagating between "entities" correctly.

## 1.0.0-alpha

### Added
- Initial version reporting metrics about Namespaces, Deployments, ReplicaSets,
  Pods and Containers. This data is fetched from two different sources: Kubelet
  and kube-state-metrics.
