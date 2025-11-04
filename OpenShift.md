## Table Of Contents
- [Table Of Contents](#table-of-contents)
- [Running OpenShift locally using CodeReady Containers](#running-openshift-locally-using-codeready-containers)
- [Automated Steps for running and testing NRI-Kubernetes on OpenShift Local](#automated-steps-for-running-and-testing-nri-kubernetes-on-openshift-local)
- [Manual Steps for running and testing NRI-Kubernetes on OpenShift Local](#manual-steps-for-running-and-testing-nri-kubernetes-on-openshift-local)
  - [1. Install, Setup, Start OpenShift Local](#1-install-setup-start-openshift-local)
  - [2. Create the Namespace In the OpenShift Cluster](#2-create-the-namespace-in-the-openshift-cluster)
  - [3. Add the service accounts your privileged Security Context Constraints](#3-add-the-service-accounts-your-privileged-security-context-constraints)
  - [4. Setup mTLS for ETCD Metrics](#4-setup-mtls-for-etcd-metrics)
  - [5. Create the e2e-values-openshift.yml file](#5-create-the-e2e-values-openshiftyml-file)
  - [6. Setup and Run E2E Test Suite](#6-setup-and-run-e2e-test-suite)
- [Accessing and exposing the internal Openshift image registry](#accessing-and-exposing-the-internal-openshift-image-registry)
- [Deploying e2e-resources on OpenShift](#deploying-e2e-resources-on-openshift)
- [Misc Tips](#misc-tips)

## Running OpenShift locally using CodeReady Containers

For running and testing locally with OpenShift 4.18 and above, [OpenShift Local](https://developers.redhat.com/products/openshift-local/overview)
To push a local compiled image to OpenShift, you'll need to [expose and push it to the internal registry](#accessing-and-exposing-the-internal-openshift-image-registry)



## Automated Steps for running and testing NRI-Kubernetes on OpenShift Local
1. run `./openshift/crc_setup.sh` to `install, setup, and start OpenShift Local`
2. run `./openshift/run.sh` for everything else 
- see [./openshift/README.md](./openshift/README.md) for more details


## Manual Steps for running and testing NRI-Kubernetes on OpenShift Local
- [Table Of Contents](#table-of-contents)
- [Running OpenShift locally using CodeReady Containers](#running-openshift-locally-using-codeready-containers)
- [Automated Steps for running and testing NRI-Kubernetes on OpenShift Local](#automated-steps-for-running-and-testing-nri-kubernetes-on-openshift-local)
- [Manual Steps for running and testing NRI-Kubernetes on OpenShift Local](#manual-steps-for-running-and-testing-nri-kubernetes-on-openshift-local)
  - [1. Install, Setup, Start OpenShift Local](#1-install-setup-start-openshift-local)
  - [2. Create the Namespace In the OpenShift Cluster](#2-create-the-namespace-in-the-openshift-cluster)
  - [3. Add the service accounts your privileged Security Context Constraints](#3-add-the-service-accounts-your-privileged-security-context-constraints)
  - [4. Setup mTLS for ETCD Metrics](#4-setup-mtls-for-etcd-metrics)
  - [5. Create the e2e-values-openshift.yml file](#5-create-the-e2e-values-openshiftyml-file)
  - [6. Setup and Run E2E Test Suite](#6-setup-and-run-e2e-test-suite)
- [Accessing and exposing the internal Openshift image registry](#accessing-and-exposing-the-internal-openshift-image-registry)
- [Deploying e2e-resources on OpenShift](#deploying-e2e-resources-on-openshift)
- [Misc Tips](#misc-tips)

 

### 1. Install, Setup, Start OpenShift Local
1. Download [Openshift Local Installer](console.redhat.com/openshift/create/local) and the pull secret
2. Run the Installer
3. Setup openshift local with the recommended settings for running e2e-tests locally   
   - initial setup takes ~10 minutes and some settings can only be set after it’s been setup
```
crc setup
crc config set enable-cluster-monitoring true
crc start -p ./pull-secret.txt
crc stop 
crc config set cpus 8
crc config set memory 32768
crc config set disk-size 90
```
   - keep track of the username and pw that is created for you during setup 

4. start openshift local: `crc start -p ./pull-secret.txt`

### 2. Create the Namespace In the OpenShift Cluster
- Decide what you want your scenario tag to be
- this will be the name of the cluster on NR1
- The namespace will be “NR-${scenario_tag}”
  - e.g scenario_tag=orange, namespace=nr-orange
```
kubectl create namespace nr-orange
```

### 3. Add the service accounts your privileged Security Context Constraints
   - from [Install the Kubernetes integration](https://docs.newrelic.com/install/kubernetes/#install-openshift-container-platform)

```
oc adm policy add-scc-to-user privileged system:serviceaccount:<namespace>:<release_name>-newrelic-infrastructure
oc adm policy add-scc-to-user privileged system:serviceaccount:<namespace>:<release_name>-nrk8s-controlplane
oc adm policy add-scc-to-user privileged system:serviceaccount:<namespace>:<release_name>-kube-state-metrics
oc adm policy add-scc-to-user privileged system:serviceaccount:<namespace>:<release_name>-newrelic-logging
oc adm policy add-scc-to-user privileged system:serviceaccount:<namespace>:<release_name>-nri-kube-events
oc adm policy add-scc-to-user privileged system:serviceaccount:<namespace>:<release_name>-nri-metadata-injection-admission
oc adm policy add-scc-to-user privileged system:serviceaccount:<namespace>:<release_name>-nrk8s-controlplane
oc adm policy add-scc-to-user privileged system:serviceaccount:<namespace>:default
for e2e-tests, the scenario_tag is the release_name and the namespace is nr-${scenario_tag}
```
 

### 4. Setup mTLS for ETCD Metrics
- To setup mTLS for ETCD in OpenShift. Follow [these](https://docs.newrelic.com/docs/kubernetes-pixie/kubernetes-integration/advanced-configuration/configure-control-plane-monitoring/#mtls-how-to-openshift) instruction.

 

### 5. Create the e2e-values-openshift.yml file
- Create a `./e2e/e2e-values-openshift.yml` file with the following values. 
  - This is to set up mTLS for etcd in OpenShift.
  - added a namespace for the ksm scraper to search in because OpenShift has it's own kube-state-metrics


```
provider: OPEN_SHIFT
ksm:
  config:
    timeout: 60s
    retries: 3
    selector: "app.kubernetes.io/name=kube-state-metrics"
    scheme: "http"
    namespace: "${namespace}"
controlPlane:
  config:
    etcd:
      enabled: true
      autodiscover:
      - selector: "app=etcd,etcd=true,k8s-app=etcd"
        namespace: openshift-etcd
        matchNode: true
        endpoints:
          - url: https://localhost:2379
            insecureSkipVerify: true
            auth:
              type: mTLS
              mtls:
                secretName: my-etcd-secret
                secretNamespace: ${namespace}
```

### 6. Setup and Run E2E Test Suite
- Run the following commands to execute e2e tests:

Include helm needed repositories.
```
helm repo add newrelic https://helm-charts.newrelic.com
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
```
Install e2e integration runner
```
go install github.com/newrelic/newrelic-integration-e2e-action@latest
```
Execute e2e test with the specific openshift files 

```
export EXCEPTION_SOURCE_FILE="1_32-exceptions-openshift.yml"
export LICENSE_KEY=xxx
export API_KEY=xxx
export ACCOUNT_ID=xxx
LICENSE_KEY=${LICENSE_KEY} EXCEPTIONS_SOURCE_FILE=${EXCEPTION_FILE}  go run github.com/newrelic/newrelic-integration-e2e-action@latest \
     --commit_sha=test-string --retry_attempts=5 --retry_seconds=60 \
	 --account_id=${ACCOUNT_ID} --api_key=${API_KEY} --license_key=${LICENSE_KEY} \
	 --spec_path=./e2e/test-specs-openshift.yml --verbose_mode=true --agent_enabled="false" --region="Staging"
```
- note these have OpenShift specific exceptions and test-spec files 
- see the [/e2e/README.md](./e2e/README.md) for more specifics 
---
## Accessing and exposing the internal Openshift image registry

The local CRC development flow depends on the Openshift image registry being exposed outside the cluster and being
accessible to a valid Openshift user. To achieve this, perform the following steps.
1. Follow [these steps](https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/registry/accessing-the-registry) to add
   the `registry-viewer` and `registry-editor` role to the `developer` user.
2. Follow [these steps](https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/registry/securing-exposing-registry#registry-exposing-default-registry-manually_securing-exposing-registry) to
   expose the registry outside the cluster _using the default route_.

---
## Deploying e2e-resources on OpenShift
The namespace we'll be using as an example is `e2e-openshift-running` 

1. Create a new service account to be assigned to the hpa and statefulset deployment pods 
```
oc create serviceaccount nri-bundle-sa
```

2. Add the `privileged` scc to your new user
```
oc adm policy add-scc-to-user privileged system:serviceaccounts:e2e-openshift-running:nri-bundle-sa
```

3. Enable OpenShift in the `charts/internal/e2e-resources/values.yaml` 
```
openShift: 
  enabled: true
```

4. Enable multiNode if using OpenShift platform 
```
persistentVolume:
  enabled: true
  multiNode: true
```

5. Must run in `demo` mode 
```
helm upgrade --install e2e-resources --set demo.enabled=true charts/internal/e2e-resources -n e2e-openshift-running
```

---
## Misc Tips

- If at any point you need to login to the guest VM, use the following command: `ssh -i ~/.crc/machines/crc/id_rsa core@$(crc ip)`
- When you get to the `crc start` command, if you encounter errors related to timeouts when attempting to check DNS
   resolution from within the guest VM, proceed to stop the VM (`crc stop`) and then restart it with `crc start -n 8.8.8.8`.
- Make sure to follow the steps for accessing the `oc` command via the `CLI` including running the `crc oc-env` command and using the `oc login ...` command to login to the cluster.
