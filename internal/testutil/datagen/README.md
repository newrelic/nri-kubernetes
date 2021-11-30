# `datagen`

Contains a set of dummy deployments and charts that can be used to scrape static test data from a live cluster, which then developers can add to the `data` folder. This data will be used for integration tests as of https://github.com/newrelic/nri-kubernetes/pull/263

## Requirements

* `kubectl` and `helm` installed and configured
* Current `kubectl` context pointed to a preferably freshly-created minikube cluster.

## How it works

`datagen.sh` script will deploy a series of dummy and required workloads to the cluster, making it a cluster containing at least one resource of all the resources we can possibly monitor.

More specifically, it will deploy:
- The KSM chart, for the KSM services/endpoints discovery tests
- The [`e2e-resources`](../../../e2e/charts/e2e-resources) chart, which includes multiple samples of resources we monitor:
  - Dummy Pods, Services, StatefulSets, DaemonSets and Deployments in several states
  - HPA targets
  - If enabled, PersistentVolumes and PersistentVolumeClaims

After this, it will deploy KSM and programmatically hit all the endpoints required for the integration to work, and store them in a directory specified by the user:

- Kubelet endpoints
  - `/pods` 
  - `/stats/summary`
  - `/metrics/cadvisor`
- KSM `/metrics`
- Control plane components
  - api-server `/metrics`
  - controller-manager `/metrics`
  - etcd `/metrics`
  - scheduler `/metrics`

It will do this by spawning a privileged, `hostNetwork` `alpine:latest` pod in the cluster and running itself from inside.

## Usage

```
./datagen.sh <output_folder>
```

Output folder would typically be a version number, e.g.

```shell
./datagen.sh 1_22
```

### Arguments and config

#### Endpoints

Endpoints for the targets to scrape can be overridden using environment variables:

```shell
KSM_ENDPOINT=${KSM_ENDPOINT:-http://ksm-kube-state-metrics.ksm.svc:8080/metrics}
KUBELET_ENDPOINT=${KUBELET_ENDPOINT:-https://localhost:10250/}
ETCD_ENDPOINT=${ETCD_ENDPOINT:-http://localhost:2381/metrics}
APISERVER_ENDPOINT=${APISERVER_ENDPOINT:-https://localhost:8443/metrics}
CONTROLLERMANAGER_ENDPOINT=${CONTROLLERMANAGER_ENDPOINT:-https://localhost:10257/metrics}
SCHEDULER_ENDPOINT=${SCHEDULER_ENDPOINT:-https://localhost:10259/metrics}
```

#### `DISABLE_CONTROLPLANE`

By default, `scraper.sh` will attempt to reach the controlpane endpoints through `localhost`. This will not work on managed K8s environments like EKS or GKE. By setting `DISABLE_CONTROLPLANE` to a non-empty value, `scraper.sh` will not attempt to reach control plane components.

> Note: For scraping EKS/GKE clusters, it's also recommended to set `IS_MINIKUBE` to false (see below). 

#### `IS_MINIKUBE`

Aiming to be extensive by default, `datagen.sh` will assume it runs in a minikube environment. This means it will:
- Try to run `minikube addons enable metrics-server`
- Deploy some PVCs with storageClass set to standard (minikube default)
- Deploy the `LoadBalancer` service with a fake `loadBalancerIP`

This can be overridden by setting `IS_MINIKUBE` to `0`, `false`, or basically anything but `1`.

#### `HELM_E2E_ARGS`

Most resources are installed using the [`e2e-resources`](../../../e2e/charts/e2e-resources) chart. `datagen.sh` will append the contents of `HELM_E2E_ARGS` to the `helm install ...` line that installs the chart, where custom values can be defined using `--set` or `-f` and an external values file.

### Arguments for development

#### `DISABLE_CLEANUP`

By default, `datagen.sh` will destroy the resources it deploys after collection, leaving the cluster in a clean state. If `DISABLE_CLEANUP` is not empty, this step will be skipped, which can be useful for troubleshooting.

#### `SKIP_INSTALL`

If non-empty, `SKIP_INSTALL` will make `datagen.sh` to skip installing the helm charts in the cluster, and instead just jump to copying itself to the scraper pod and collect data. This can be useful for debugging the collection script without having to wait for helm to deploy things.
