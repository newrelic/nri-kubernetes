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
  - PVs and PVCs
  - Stuck/crashing pods

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

TBD
