#!/usr/bin/env bash

set -e

# scrapper_selector is the label with which the scraper deployment is deployed.
scrapper_selector="app=scraper"
scrapper_namespace="scraper"

# Default endpoints for minikube, extensible to some extent to other distros like kubeadm.
KSM_ENDPOINT=${KSM_ENDPOINT:-http://e2e-kube-state-metrics.${scrapper_namespace}.svc:8080/metrics}
KUBELET_ENDPOINT=${KUBELET_ENDPOINT:-https://localhost:10250}
# If control plane is not reachable (e.g. managed k8s), set DISABLE_CONTROLPLANE=1.
ETCD_ENDPOINT=${ETCD_ENDPOINT:-http://localhost:2381/metrics}
APISERVER_ENDPOINT=${APISERVER_ENDPOINT:-https://localhost:8443/metrics}
CONTROLLERMANAGER_ENDPOINT=${CONTROLLERMANAGER_ENDPOINT:-https://localhost:10257/metrics}
SCHEDULER_ENDPOINT=${SCHEDULER_ENDPOINT:-https://localhost:10259/metrics}

# Script will apply minikube-specific actions if this variable is set to 1.
# If empty (default), minikube environment will be autodetected based on the name of the context.
IS_MINIKUBE=""

# Extra args that will be appended to the helm install e2e command.
# Values can be overridden using this and --set commands, or -f and point to a custom values file.
# Useful to tweak things specific to minikube, or toggle specific features.
# See  ../../../e2e/charts/e2e-resources/values.yaml for more details.
HELM_E2E_ARGS=""

# main subcommand runs the whole flow of the script: Bootstrap, scrape, and cleanup
function main() {
    if [[ -z "$1" ]]; then
        echo "Usage: $0 <output_folder>"
        exit 1
    fi

    # Install scraper pod and e2e resources.
    bootstrap

    # Scraper pod is a deployment, so we need to locate it.
    pod=$(scraper_pod "$scrapper_selector")
    if [[ -z "$pod" ]]; then
        echo "Could not find scraper pod (-l $scrapper_selector)"
        exit 1
    fi

    # Collect cds to the specified dir so we invoke it on a subshell
    ( collect "$1" )

    if [[ "$DISABLE_CLEANUP" != "1" ]]; then
      cleanup
    fi
}

function collect() {
    # Dump test info
    mkdir -p "$1" && cd "$1"
    testinfo > README.md

    # Kubelet endpoints
    mkdir -p kubelet
    echo "Extracting kubelet /pods"
    kubectl exec -n $scrapper_namespace $pod -- datagen.sh scrape kubelet pods > kubelet/pods

    echo "Extracting kubelet /metrics/cadvisor"
    mkdir -p kubelet/metrics
    kubectl exec -n $scrapper_namespace $pod -- datagen.sh scrape kubelet metrics/cadvisor > kubelet/metrics/cadvisor

    echo "Extracting kubelet /stats/summary"
    mkdir -p kubelet/stats
    kubectl exec -n $scrapper_namespace $pod -- datagen.sh scrape kubelet stats/summary > kubelet/stats/summary

    # KSM endpoint
    mkdir -p ksm
    echo "Extracting ksm /metrics"
    kubectl exec -n $scrapper_namespace $pod -- datagen.sh scrape ksm > ksm/metrics

    if [[ "$DISABLE_CONTROLPLANE" != "1" ]]; then
        # Control plane components
        mkdir -p controlplane/api-server
        echo "Extracting api-server /metrics"
        kubectl exec -n $scrapper_namespace $pod -- datagen.sh scrape controlplane apiserver > controlplane/api-server/metrics

        mkdir -p controlplane/etcd
        echo "Extracting etcd /metrics"
        kubectl exec -n $scrapper_namespace $pod -- datagen.sh scrape controlplane etcd > controlplane/etcd/metrics

        mkdir -p controlplane/controller-manager
        echo "Extracting controller-manager /metrics"
        kubectl exec -n $scrapper_namespace $pod -- datagen.sh scrape controlplane controllermanager > controlplane/controller-manager/metrics

        mkdir -p controlplane/scheduler
        echo "Extracting scheduler /metrics"
        kubectl exec -n $scrapper_namespace $pod -- datagen.sh scrape controlplane scheduler > controlplane/scheduler/metrics
    else
        echo "Skipping control plane metrics"
    fi

    # K8s objects
    echo "Generating list of kubernetes resources"
    kubedump nodes
    kubedump namespaces
    kubedump endpoints
    kubedump services
    kubedump pods
}

# bootstrap suvcommand installs the required components in the cluster to generate the testdata.
# If $SKIP_INSTALL is non-empty, it skips deploying KSM, the dummy resources, and the scrapper pod and just copies
# script inside an scraper pod that is assumed to exist already.
function bootstrap() {
    ctx=$(kubectl config current-context)
    if [[ -z "$IS_MINIKUBE" && "$ctx" = "minikube" ]]; then
        echo "Assuming minikube distribution since context is \"$ctx\""
        echo "Set IS_MINIKUBE to 0 or 1 to override autodetection"
        IS_MINIKUBE=1
    fi

    echo
    echo -e "Using context $ctx, is this ok?\n^C now if it is not."
    read

    if [[ -z $SKIP_INSTALL ]]; then
        echo "Installing e2e-resources chart"
        if [[ "$IS_MINIKUBE" = "1" ]]; then
            # Enable PVC if we are in minikube
            minikube_args="--set persistentVolumeClaim.enabled=true --set loadBalancerService.fakeIP=127.1.2.3"
            minikube addons enable metrics-server
        fi

        echo "Updating helm dependencies"
        helm dependency update ../../../e2e/charts/e2e-resources > /dev/null
        helm upgrade --install e2e ../../../e2e/charts/e2e-resources -n $scrapper_namespace --create-namespace \
          --set scraper.enabled=true \
          --set persistentVolume.enabled=true \
          $minikube_args \
          $HELM_E2E_ARGS

        echo "Waiting for KSM to become ready"
        kubectl -n $scrapper_namespace wait --for=condition=Ready pod -l app.kubernetes.io/name=kube-state-metrics

        echo "Waiting for E2E resources to settle"
        kubectl -n $scrapper_namespace wait --for=condition=Ready pod -l app=hpa
        kubectl -n $scrapper_namespace wait --for=condition=Ready pod -l app=daemonset
        kubectl -n $scrapper_namespace wait --for=condition=Ready pod -l app=statefulset
    fi

    echo "Waiting for scraper pod to be ready"
    kubectl -n $scrapper_namespace wait --for=condition=Ready pod -l "$scrapper_selector"
    pod=$(scraper_pod "$scrapper_selector")
    if [[ -z "$pod" ]]; then
        echo "Could not find scraper pod (-l $scrapper_selector)"
        exit 1
    fi
    echo "Found scraper pod $pod"

    echo "Copying datagen.sh to scraper pods"
    kubectl cp datagen.sh $scrapper_namespace/$pod:/bin/
}

# cleanup uninstalls the dummy resources and the scraper pod.
function cleanup() {
    echo "Removing e2e-resources chart"
    helm uninstall e2e -n $scrapper_namespace || true
}

# scrape will curl the specified component and output the response body to standard output.
function scrape() {
    bearer="Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)"
    endpoint=unsupported://

    case $1 in
    ksm)
      endpoint=${KSM_ENDPOINT}
    ;;
    kubelet)
      # Callers put the subpath in $2, e.g. `scrape kubelet stats/summary`
      endpoint=${KUBELET_ENDPOINT}/${2}
    ;;
    controlplane)
      case $2 in
        etcd)
          endpoint=${ETCD_ENDPOINT}
          ;;
        apiserver)
          endpoint=${APISERVER_ENDPOINT}
          ;;
        controllermanager)
          endpoint=${CONTROLLERMANAGER_ENDPOINT}
          ;;
        scheduler)
          endpoint=${SCHEDULER_ENDPOINT}
        ;;
          *)
          echo "Unsupported controlplane component $2" >&2
          return 1
          ;;
      esac
      ;;
    *)
      echo "Unknown scrapable $1" >&2
      return 1
      ;;
    esac

    curl -ksSL "$endpoint" -H "$bearer"
    return $?
}

# scraper_pod is a helper function that returns the name of a pod matching the scraper label.
function scraper_pod() {
    kubectl get pods -l $1 -n $scrapper_namespace -o jsonpath='{.items[0].metadata.name}'
}

# kubedump is a helper function that dumps the specified Kubernetes resource to a yaml file of the same name.
function kubedump() {
    echo "Extracting kubernetes $1"
    kubectl get "$1" -o yaml --all-namespaces > "$1".yaml
}

# Testinfo is a helper function that returns a markdown-formatted string with basic info of the environment where this
# script was run.
function testinfo() {
cat <<EOF
# Static test data generated by $(whoami) on $(date -I)

### \`nri-kubernetes\` commit
\`\`\`
$(git rev-parse HEAD)
\`\`\`

\`git status --short\`

\`\`\`
$(git status --short)
\`\`\`

### \`$ kubectl version\`
\`\`\`
$(kubectl version)
\`\`\`

### Kubernetes nodes
\`\`\`
$(kubectl get nodes -o wide)
\`\`\`
EOF
}

# Script entrypoint
command=$1

case $command in
scrape|bootstrap|cleanup|testinfo|etcd_certs)
    shift
    $command "$@"
    exit $?
  ;;
esac

main "$@"
