# Change Log

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## Unreleased

---

## 2.4.0

### Added

- Support for multiarch docker images

## 2.3.1

### Fixed

- Correctly identifing k8s server version with characters (#81)

## 2.3.0

### Changed

- The base image of `newrelic/infrastructure-k8s` has been updated to `2.2.3`. 
  More info regarding all the integrations upgraded can be found in the [release notes of the base image](https://github.com/newrelic/infrastructure-bundle/releases/tag/2.2.3).
- Changed scale of node `cpuRequestedCores` to cores from millis
  
### Added

- Added metrics pertaining to Horizontal Pod Autoscaler. More information about the collected metrics can be found in the [official documentation](https://docs.newrelic.com/docs/integrations/kubernetes-integration/understand-use-data/find-use-your-kubernetes-data)

### Fixed

- LoadBalancerIP was not being collected properly. It is now fetched from KSM metric `kube_service_status_load_balancer_ingress`

## 2.2.0

### Changed

- The base image of `newrelic/infrastructure-k8s` has been updated to `2.2.1`. 
  This base image has fixed an issue where `nrjmx` was not properly running due to the bundled java version.
  More info regarding all the integration upgraded can be found in the [release notes of the base image](https://github.com/newrelic/infrastructure-bundle/releases/tag/2.2.1).

## 2.1.0

### Changed

- Added aggregate cpu and memory requests for nodes

## 2.0.0

### Changed

- The base image of `newrelic/infrastructure-k8s` has been updated to `2.0.0`. 
  That base image is bundling the integration `nri-nginx` `3.0.2` that contains a breaking change. 
  More info regarding all the integration upgraded can be found in the [release notes of the base image](https://github.com/newrelic/infrastructure-bundle/releases/tag/2.0.0).


## 1.26.9

### Changed

- Added release pipeline to Github Actions.

## 1.26.8

### Changed

- Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.6.0.
  For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.6.0).

## 1.26.7

### Fixed 

- When configuring API_SERVER_ENDPOINT_URL with https endpoint, the
  api-server component will use that that instead of the default.
  More info at issue #41

## 1.26.6

### Fixed 

- When discovering kube-state-metrics behind a headless service, the
  DNS discovery will return an error. Before it would be considered
  successfull and return "None" as endpoint.

## 1.26.5

### Fixed

- Container id's are correctly parsed when using system driver

## 1.26.4

### Added

- Added `restartCount` to containers in the `wainting` state
  - In case the container is in a crash loop the restart count would not be shown
  
## 1.26.3

### Added

- Added `restartCount` to containers in the `terminated` state
  - In case the container is in a crash loop the restart count would not be shown

## 1.26.2

### Changed

- Upgrade Docker image to use Tini entrypoint solving:
  - reaps orphaned zombie process attached to PID 1
  - correctly forwards signals to CMD process
  Read for more details: https://blog.phusion.nl/2015/01/20/docker-and-the-pid-1-zombie-reaping-problem/
- Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.5.1.
  For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.5.1).

## 1.26.1

### Changed

- Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.5.0.
  For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.5.0).

## 1.26.0

### Changed
- When querying the summary endpoint from Kubelet to get the Node or Pod
  network metrics, if the default network interface is not eth0 then summary
  endpoint for Kubelet doesn't return the metrics as we expect them. We rely on
  them being a direct member of the "network" object. See rxBytes, txBytes and
  rxErrors in the following example metrics:

```
"network": {
 "time": "2020-06-04T10:01:15Z",
 "name": "eth0",
 "rxBytes": 207909096,
 "rxErrors": 0,
 "txBytes": 8970981,
 "txErrors": 0,
 "interfaces": [
  {
   "name": "eth0",
   "rxBytes": 207909096,
   "rxErrors": 0,
   "txBytes": 8970981,
   "txErrors": 0
  },
  {
   "name": "ip6tnl0",
   "rxBytes": 0,
   "rxErrors": 0,
   "txBytes": 0,
   "txErrors": 0
  },
  {
   "name": "tunl0",
   "rxBytes": 0,
   "rxErrors": 0,
   "txBytes": 0,
   "txErrors": 0
  }
 ]
}
```

  This scenario only happens when the default interface is eth0. Kubernetes
  source code has it hardcoded that eth0 is the default. In the following
  example you can see that we only have network metrics inside the interfaces
  list, in this case there is no eth0 on the and the default interface is ens5:

```
"network": {
 "time": "2020-06-04T10:01:15Z",
 "name": "",
 "interfaces": [
  {
   "name": "ens5",
   "rxBytes": 207909096,
   "rxErrors": 42,
   "txBytes": 8970981,
   "txErrors": 24
  },
  {
   "name": "ip6tnl0",
   "rxBytes": 0,
   "rxErrors": 0,
   "txBytes": 0,
   "txErrors": 0
  },
  {
   "name": "tunl0",
   "rxBytes": 0,
   "rxErrors": 0,
   "txBytes": 0,
   "txErrors": 0
  }
 ]
```

  In cases like this, the integration will look for the default interface
  inside the interfaces list and use those values. The default interface name
  is retrieved from the network route file (default /proc/net/route).

  When running the unprivileged version of the integration we don't have access
  to the route file, the integration won't be able to get the default interface
  name and won't send network metrics for the unless there's a network
  interface called eth0.

  For Pods, this issue is mainly present when using hostNetwok since they
  shared the same network interfaces with the Node.

- Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.4.2.
  For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.4.2).
## 1.25.0

### Added
- Support for Kubernetes versions 1.17.X

### Changed
- Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.4.1.
  For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.4.1).
- The API server is queried by default on the Secure Port using the service account's bearer authentication.
  If the query on the Secure Port fails, it will fallback automatically to the non-secure one. This should preserve
  the same behavior as previous versions.

## 1.24.0

### Changed
 - Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.4.0.
   For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.4.0).

## 1.23.1

### Fixed
- Bug that swapped values of node allocatable resources with node capacity
  resources.

## 1.23.0

### Added
 - Kubernetes 1.16 is now officially supported.
    - The minimum supported version of kube-state-metrics for this release is 1.9.5, according to the [KSM compatibility matrix](https://github.com/kubernetes/kube-state-metrics#compatibility-matrix)
 - Added container throttling metrics to the K8sContainerSample:
   - `containerCpuCfsPeriodsDelta`: Delta change of elapsed enforcement period intervals.
   - `containerCpuCfsThrottledPeriodsDelta`: Delta change of throttled period intervals.
   - `containerCpuCfsThrottledSecondsDelta`:  Delta change of duration the container has been throttled.
   - `containerCpuCfsPeriodsTotal`: Number of elapsed enforcement period intervals.
   - `containerCpuCfsThrottledPeriodsTotal`: Number of throttled period intervals.
   - `containerCpuCfsThrottledSecondsTotal`: Total time duration the container has been throttled.
 - Added container mmap byte usage metrics to the K8sContainerSample:
   - `containerMemoryMappedFileBytes`: Size of memory mapped files in bytes.

## 1.22.0

### Changed
 - Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.3.9.
   For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.3.9).

## 1.21.0

### Added
 - Resources allocatable and capacity are retrieved from the apiserver and
   added to the `K8sNodeSample` as `capacity<ResourceName>` and
   `allocatable<ResourceName>`.
 - The Kubernetes server version is retrieved from the apiserver and cached
   with the `APIServerCacheK8SVersionTTL` config option. The Kubernetes version
   is added to the `K8sClusterSample` as `clusterK8sVersion` and to inventory.
 - Add support for static pods status for Kubernetes server versions 1.15 or
   newer.

### Changed
 - Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.3.8.
   For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.3.8).

## 1.20.0

### Changed
 - Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.3.5.
   For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.3.5).

## 1.19.0

### Added
 - New label combination to discover the Kubernetes controller manager:
    * `app=controller-manager`
    * `controller-manager=true`

### Changed
 - Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.3.4.
   For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.3.4).

## 1.18.0

### Changed
 - Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.3.2.
   For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.3.2).

## 1.17.0

### Added
 - Added the necesary files for building a windows image of the integration.
   The windows image needs to be manually created and it's still not in our
   CI/CD pipeline. We have the files for building it but we are not publishing
   it. The latest supported image for Windows, at the time of writing, is
   1.16.0.
 - Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.3.0.
   For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.3.0).

## 1.16.0

### Added
 - Support for completely avoid querying Kube State Metrics. It's behind the `DISABLE_KUBE_STATE_METRICS` environment variable
   and its default value is `false`. Note that disabling this will imply in missing metrics that are collected from KSM
   and possibly missing features in the Kubernetes Cluster Explorer. Please, refer to our [official documentation on this
   configuration option](https://docs.newrelic.com/docs/integrations/kubernetes-integration/installation/kubernetes-installation-configuration#disable-kube-state-metrics) for more information.

## 1.15.0

### Added
 - Support for querying Kube State Metrics instances behind [kube-rbac-proxy](https://github.com/brancz/kube-rbac-proxy).
   It **ONLY works when the label-based KSM discovery is enabled through the `KUBE_STATE_METRICS_POD_LABEL` environment variable**.
   2 new configuration environment variables are added:
   - KUBE_STATE_METRICS_SCHEME: defaults to `http`. Valid values are `http` and `https`.
   - KUBE_STATE_METRICS_PORT: defaults to `8080`. On a standard setup of **kube-rbac-proxy** this should be set to `8443`.
 - OpenShift Control Plane components are now automatically discovered.
 - Added 4 new environment variables to explicitly set the Control Plane components URLs:
   - SCHEDULER_ENDPOINT_URL
   - ETCD_ENDPOINT_URL
   - CONTROLLER_MANAGER_ENDPOINT_URL
   - API_SERVER_ENDPOINT_URL

### Fixed
 - Fix a bug that was preventing `selector.<key>` type attributes to not be
  added to some of the `K8sServiceSample`.
 - The integration now uses `newrelic/infrastructure-bundle` as the base image. The version used
   is `1.2.0`, for more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.2.0).

## 1.13.2
### Changed
 - The integration now uses the infrastructure agent v1.9.0-bundle. For more
   information refer to the [infrastructure agent release notes](https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/)
   between versions v1.8.32 and v1.9.0.

## 1.13.1
### Added
 - Added daemonsetName field to the K8sDaemonsetSample

## 1.13.0
### Added
 - Added samples for Statefulsets, Daemonsets, Endpoints and Services.
 - API Server metrics can now be queried using the secure port. Configure the port using the `API_SERVER_SECURE_PORT` environment variable. The ClusterRole has been updated to allow this query to happen.
 - The integration now uses the infrastructure agent v1.8.32-bundle. For more
   information refer to the [infrastructure agent release notes](https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/)
   between versions v1.8.23 and v1.8.32.

   The bundle container contains a subset of [On-host integrations](https://docs.newrelic.com/docs/integrations/new-relic-integrations/get-started/introduction-infrastructure-integrations) that are supported by New Relic.
   This also includes the ability to "Auto Discover" services running on Kubernetes in a similar way to our [Container auto-discovery](https://docs.newrelic.com/docs/integrations/host-integrations/installation/container-auto-discovery)
 - The integration has been renamed from `nr-kubernetes` to `nri-kubernetes`.

## 1.12.0
### Changed
 - Control Plane components can now also be discovered using the `tier` and `component` labels, besides `k8s-app`.
 - The integration now uses the infrastructure agent v1.8.23. For more
   information refer to the [infrastructure agent release notes](https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/)
   between versions v1.5.75 and v1.8.23.

## 1.11.0
### Changed
 - The old way of determining Leader/Follower status has been switched to a
   job based architecture. The old Leader/Follower was needed to detect which nri-kubernetes Pod
   should query Kube State Metrics (a.k.a. the Leader), but it was hard to add additional scrape targets (e.g. control plane).
   Important notice: The output logs have been changed. Before the integration logged whether
   it was Follower or a Leader, and this has been changed to show which jobs are executed.

   Following are 2 examples of logs that are being output by the integration before and after this update.

   Before, logs for leaders:
   ```
   Auto-discovered role: Leader
   ```

   After, equivalent example, now using jobs:
   ```
   Running job: kubelet
   Running job: kube state metrics
   ```

   Before, logs for followers:
   ```
   Auto-disovered role: Follower
   ```

   After, equivalent example, now using jobs:
   ```
   Running job: kubelet
   ```
   These 2 before & after examples are identical in the targets & information they scrape.

 - The e2e test package has been updated to work with this refactor.
### Added
 - Control Plane Monitoring: the integration will automatically detect if it's running on a master node using
   its Kubernetes pod's labels, which are retrieved from the API Server. If it finds itself running on a master
   node, these additional jobs will run:
     - ETCD
     - API Server
     - Controller Manager
     - Scheduler

   All jobs, except ETCD, will work out of the box with no further configuration needed.
   ETCD exposes its metrics using Mutual TLS, which can be configured as follows.

   First, create a secret containing the following fields:
   ```
   key: <private_key_data, PEM format>
   cert: <certificate_belonging_to_private_key, PEM format>
   cacert: <optional, the ETCD cacert, PEM format>
   insecureSkipVerify: <optional, bool 'true' or 'false', default: 'false'>
   ```
   Which can be created like this (expecting the files `key`, `cert` and `cacert` to be present):
   ```
   kubectl create secret generic etcd-server-tls --from-file=./key --from-file=./cert --from-file=./cacert
   ```
   Then, configure the integration to use this secret using these environment variables.
   ```
   ETCD_TLS_SECRET_NAME: etcd-server-tls
   ETCD_TLS_SECRET_NAMESPACE: default
   ```

   If everything is configured properly the integration should start collecting ETCD metrics.
 - A new command, called kubernetes-static has been added, which enables the
   integration to be run locally on your machine, without deploying it to k8s.
   It uses a static set of exports from kubelet & KSM.
 - A new way to query a specific Kube State Metrics (KSM) pod  when running multiple redundant pods: by Label Selector.
   If you want to target a certain KSM instance, you can now use the `KUBE_STATE_METRICS_POD_LABEL` environment variable.
   If this variable has been set (and KUBE_STATE_METRICS_URL is unset) the integration will find the KSM pod by this variable.

   For example:
      ```shell script
      # Label a specific KSM pod. Always set the value to the string "true".
      kubectl label pod kube-state-metrics please-use-this-ksm-pod=true
      ```
   Configure `nri-kubernetes` to use this KSM pod:

   ```yaml
    env:
    - name: KUBE_STATE_METRICS_POD_LABEL
      value: please-use-this-ksm-pod
    ```

## 1.10.2
### Added
- The integration now uses the infrastructure agent v1.5.75. For more
  information refer to the [infrastructure agent release notes](https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/)
  between versions v1.5.31 and v1.5.75.

## 1.10.1
### Changed
- Rollback agent version to v1.5.31 because there is an issue with nodes
  reporting inventory using the node ip as entity key, this causes the nodes to
  be indexed as clusters.

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
