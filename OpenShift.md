## Running OpenShift locally using CodeReady Containers

For running and testing locally with OpenShift 4.x and above,
[CodeReady Containers](https://developers.redhat.com/products/codeready-containers/overview) can be used.
Instructions are provided below.

For running and testing locally with Openshift 3.x and prior,
[minishift](https://github.com/minishift/minishift) can be used.

### Using CodeReady Containers

1. Login to [the RedHat Customer Portal](https://access.redhat.com/) with your RedHat account
2. Follow the instructions
   [here](https://access.redhat.com/documentation/en-us/red_hat_codeready_containers/1.0/html/getting_started_guide/getting-started-with-codeready-containers_gsg)
   to download and install CRC
3. When you get to the `crc start` command, if you encounter errors related to timeouts when attempting to check DNS
   resolution from within the guest VM, proceed to stop the VM (`crc stop`) and then restart it with `crc start -n 8.8.8.8`.
4. Make sure to follow the steps for accessing the `oc` command via the `CLI` including running the `crc oc-env`
5. command and using the `oc login ...` command to login to the cluster.

### Accessing and exposing the internal Openshift image registry

The local CRC development flow depends on the Openshift image registry being exposed outside the cluster and being
accessible to a valid Openshift user. To achieve this, perform the following steps.

1. Follow [these steps](https://docs.openshift.com/container-platform/4.1/registry/accessing-the-registry.html) to add
   the `registry-viewer` and `registry-editor` role to the `developer` user.
2. Follow [these steps](https://docs.openshift.com/container-platform/4.1/registry/securing-exposing-registry.html) to
   expose the registry outside the cluster _using the default route_.

### CRC configuration

Configuration is generally the same as above with the following differences.

Etcd in Openshift requires mTLS. This means you have to follow our documentation
[here](https://docs.newrelic.com/docs/integrations/kubernetes-integration/installation/configure-control-plane-monitoring#mtls-how-to)
in order to setup client cert auth. The only difference is how you obtain the client cert/key and cacert.
The default CRC setup does not provide the private key of the root CA and therefore you can't use your own cert/key pair
since you can't sign the CSR. However, they do already provide a pre-generated cert/key pair that "peers" can use.
Following is how you can get this info.
 1. Use `scp -i ~/.crc/machines/crc/id_rsa core@$(crc ip):PATH_TO_FILE` to copy the following files to your local machine
     * The peer/client cert: `/etc/kubernetes/static-pod-resources/etcd-member/system:etcd-metric:etcd-0.crc.testing.crt`
     * The peer/client private key: `/etc/kubernetes/static-pod-resources/etcd-member/system:etcd-metric:etcd-0.crc.testing.key`
     * The root CA cert: `/etc/kubernetes/static-pod-resources/etcd-member/metric-ca.crt`
 2. Rename `system:etcd-metric:etcd-0.crc.testing.crt` to `cert`
 3. Rename `system:etcd-metric:etcd-0.crc.testing.key` to `key`
 4. Rename `metric-ca.crt` to `cacert`


### Manual deployment

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

### Deploying e2e-resources on OpenShift
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
