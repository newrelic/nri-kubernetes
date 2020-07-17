[![Community Project header](https://github.com/newrelic/opensource-website/raw/master/src/images/categories/Community_Project.png)](https://opensource.newrelic.com/oss-category/#community-project)

# New Relic Integration for Kubernetes

New Relic Integration for Kubernetes instruments the container orchestration layer by reporting metrics from Kubernetes objects. It gives you visibility about Kubernetes namespaces, deployments, replica sets, nodes, pods, and containers. Metrics are collected from different sources.
* [kube-state-metrics service](https://github.com/kubernetes/kube-state-metrics) provides information about state of Kubernetes objects like namespace, replicaset, deployments and pods (when they are not in running state)
* `/stats/summary` kubelet endpoint gives information about network, errors, memory and CPU usage
* `/pods` kubelet endpoint provides information about state of running pods and containers
* `/metrics/cadvisor` cAdvisor endpoint provides missing data that is not included in the previous sources.
* Node labels are retrieved from the k8s API server.

Check [documentation](https://docs.newrelic.com/docs/kubernetes-integration-new-relic-infrastructure) in order to find out more how to install and configure the integration, learn what metrics are captured and how to view them.

## Table of Contents

- [Table of Contents](#table-of-contents)
- [Installation](#installation)
- [Usage](#usage)
- [Running the integration against a static data set](#running-the-integration-against-a-static-data-set)
- [In cluster development](#in-cluster-development)
  - [Prerequisites](#prerequisites)
  - [Configuration](#configuration)
  - [Run](#run)
  - [Tests](#tests)
- [Running OpenShift Locally Using CodeReady Containers](#running-openshift-locally-using-codeready-containers)
  - [Using CodeReady Containers](#using-codeready-containers)
  - [Accessing and Exposing the Internal Openshift Image Registry](#accessing-and-exposing-the-internal-openshift-image-registry)
  - [CRC Configuration](#crc-configuration)
  - [Skaffold Deployment](#skaffold-deployment)
  - [Manual Deployment](#manual-deployment)
  - [Tips](#tips)
- [Support](#support)
- [Contributing](#contributing)
- [License](#license)

## Installation

Firstly check [compatibility and requirements](https://docs.newrelic.com/docs/kubernetes-monitoring-integration#compatibility) and then follow the
[installation steps](https://docs.newrelic.com/docs/kubernetes-monitoring-integration#install).
For troubleshooting help, see [Not seeing data](https://docs.newrelic.com/docs/integrations/host-integrations/troubleshooting/kubernetes-integration-troubleshooting-not-seeing-data), or [Error messages](https://docs.newrelic.com/docs/integrations/host-integrations/troubleshooting/kubernetes-integration-troubleshooting-error-messages).

## Usage

Check how to [find and use data](https://docs.newrelic.com/docs/kubernetes-monitoring-integration#view-data) and description of all [captured data](https://docs.newrelic.com/docs/kubernetes-monitoring-integration#metrics).

## Running the integration against a static data set

See [cmd/kubernetes-static/readme.md](./cmd/kubernetes-static/readme.md) for more details.

## In cluster development

### Prerequisites
For in cluster development process [Minikube](https://kubernetes.io/docs/getting-started-guides/minikube) and [Skaffold](https://github.com/GoogleCloudPlatform/skaffold) tools are used.
* [Install Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/).
* [Install Skaffold](https://github.com/GoogleCloudPlatform/skaffold#installation).

### Configuration

* Copy the daemonset file `deploy/newrelic-infra.yaml` to `deploy/local.yaml`.
* Edit the file and set the following value as container image: `newrelic/infrastructure-k8s-dev`.

```yaml
  containers:
    - name: newrelic-infra
      image: newrelic/infrastructure-k8s-dev
      resources:
```

* Edit the file and specify the following `CLUSTER_NAME` and `NRIA_LICENSE_KEY` on the `env` section.

 ```yaml
 env:
 - name: "CLUSTER_NAME"
   value: "<YOUR_CLUSTER_NAME>"
 - name: "NRIA_LICENSE_KEY"
   value: "<YOUR_LICENSE_KEY>"
 ```

### Run

Run `make deploy-dev`. This will compile your integration binary with compatibility for the container OS architecture, build a temporary docker image and finally deploy it to your Minikube.

Then you can [view your data](#usage) or run the integration standalone. To do so follow the steps:

* Run

```bash
NR_POD_NAME=$(kubectl get pods -l name=newrelic-infra -o jsonpath='{.items[0].metadata.name}')
```
This will retrieve the name of a pod where the Infrastructure agent and Kuberntetes Infrastructure Integration are installed.

* Enter to the pod

```bash
kubectl exec -it $NR_POD_NAME -- /bin/bash
```

* Execute the Kubernetes integration

```bash
/var/db/newrelic-infra/newrelic-integrations/bin/nri-kubernetes -pretty
```

### Tests

For running unit tests, use

```bash
make test
```

For running e2e tests locally, use:

```bash
CLUSTER_NAME=<your-cluster-name> NR_LICENSE_KEY=<your-license-key>  make e2e
```

This make target is executing `go run e2e/cmd/e2e.go`. You could execute that
command with `--help` flag to see all the available options.

## Running OpenShift Locally Using CodeReady Containers

For running and testing locally with OpenShift 4.x and above, [CodeReady Containers](https://developers.redhat.com/products/codeready-containers/overview) can be used. Instructions are provided below.  For running and testing locally with Openshift 3.x and prior, [minishift](https://github.com/minishift/minishift) can be used.

### Using CodeReady Containers

1. Login to [the RedHat Customer Portal](https://access.redhat.com/) with your RedHat account
1. Follow the instructions [here](https://access.redhat.com/documentation/en-us/red_hat_codeready_containers/1.0/html/getting_started_guide/getting-started-with-codeready-containers_gsg) to download and install CRC
1. When you get to the `crc start` command, if you encounter errors related to timeouts when attempting to check DNS resolution from within the guest VM, proceed to stop the VM (`crc stop`) and then restart it with `crc start -n 8.8.8.8`.
1. Make sure to follow the steps for accessing the `oc` command via the `CLI` including running the `crc oc-env` command and using the `oc login ...` command to login to the cluster.

### Accessing and Exposing the Internal Openshift Image Registry

The local CRC development flow depends on the Openshift image registry being exposed outside the cluster and being accessible to a valid Openshift user. To achieve this, perform the following steps.

1. Follow [these steps](https://docs.openshift.com/container-platform/4.1/registry/accessing-the-registry.html) to add the `registry-viewer` and `registry-editor` role to the `developer` user.
1. Follow [these steps](https://docs.openshift.com/container-platform/4.1/registry/securing-exposing-registry.html) to expose the registry outside the cluster _using the default route_.

### CRC Configuration

Configuration is generally the same as above with the following differences.

1. The local configuration file used by `skaffold` is `local-openshift.yaml`
1. In addition to setting the `CLUSTER_NAME` and `NRIA_LICENSE_KEY`, you will need to uncomment the `*_ENDPOINT_URL` variables. The defaults set at the time of this writing (2/19/2020) are properly set for the default CRC environment.
1. Etcd in Openshift requires mTLS. This means you have to follow our documentation [here](https://docs.newrelic.com/docs/integrations/kubernetes-integration/installation/configure-control-plane-monitoring#mtls-how-to) in order to setup client cert auth. The only difference is how you obtain the client cert/key and cacert. The default CRC setup does not provide the private key of the root CA and therefore you can't use your own cert/key pair since you can't sign the CSR. However, they do already provide a pre-generated cert/key pair that "peers" can use. Following is how you can get this info.
   1. Use `scp -i ~/.crc/machines/crc/id_rsa core@$(crc ip):PATH_TO_FILE` to copy the following files to your local machine
      * The peer/client cert: `/etc/kubernetes/static-pod-resources/etcd-member/system:etcd-metric:etcd-0.crc.testing.crt`
      * The peer/client private key: `/etc/kubernetes/static-pod-resources/etcd-member/system:etcd-metric:etcd-0.crc.testing.key`
      * The root CA cert: `/etc/kubernetes/static-pod-resources/etcd-member/metric-ca.crt`
   1. Rename `system:etcd-metric:etcd-0.crc.testing.crt` to `cert`
   1. Rename `system:etcd-metric:etcd-0.crc.testing.key` to `key`
   1. Rename `metric-ca.crt` to `cacert`
   1. Carry on with the steps in our documentation.

### Skaffold Deployment

To deploy the integration to CRC via `skaffold`, run `skaffold run -p openshift`.

### Manual Deployment

The `skaffold` deployment doesn't always seem to work reliably. In case you need to deploy manually, perform the following steps.

Perform the following steps once per terminal session.

```bash
oc login -u kubeadmin -p PASSWORD_HERE https://api.crc.testing:6443
OCHOST=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
oc login -u developer -p developer https://api.crc.testing:6443
docker login -u developer -p $(oc whoami -t) $OCHOST
oc login -u kubeadmin -p PASSWORD_HERE https://api.crc.testing:6443
```

Perform the following steps each time you want to deploy.

```bash
make compile-dev
docker build . -t infrastructure-k8s-dev
docker tag infrastructure-k8s-dev default-route-openshift-image-registry.apps-crc.testing/default/infrastructure-k8s-dev
docker push default-route-openshift-image-registry.apps-crc.testing/default/infrastructure-k8s-dev
oc apply -f deploy/local-openshift.yaml
```

### Tips

* If at any point you need to login to the guest VM, use the following command: `ssh -i ~/.crc/machines/crc/id_rsa core@$(crc ip)`
* During testing it seemed that occassionally the cluster would stop reporting data for no reason (especially after my machine wakes up from sleep mode). If this happens, use the Microsoft solution (just restart the cluster).

## Support

New Relic hosts and moderates an online forum where customers can interact with New Relic employees as well as other customers to get help and share best practices. Like all official New Relic open source projects, there's a related Community topic in the New Relic Explorers Hub. You can find this project's topic/threads here:

>Add the url for the support thread here

## Contributing
We encourage your contributions to improve the New Relic Integration for Kubernetes! Keep in mind when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. You only have to sign the CLA one time per project.
If you have any questions, or to execute our corporate CLA, required if your contribution is on behalf of a company,  please drop us an email at opensource@newrelic.com.

## License
New Relic Integration for Kubernetes is licensed under the [Apache 2.0](http://apache.org/licenses/LICENSE-2.0.txt) License.
