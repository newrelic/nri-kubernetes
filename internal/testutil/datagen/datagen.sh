#!/usr/bin/env bash

set -e

# scrapper_selector is the label with which the scraper deployment is deployed.
scrapper_selector="app=scraper"

# main subcommand runs the whole flow of the script: Bootstrap, scrape, and cleanup
function main() {
    if [[ -z "$1" ]]; then
        echo "Usage: $0 <output_folder>"
        exit 1
    fi

    bootstrap

    pod=$(scraper_pod "$scrapper_selector")
    if [[ -z "pod" ]]; then
        echo "Could not find scraper pod (-l $scrapper_selector)"
        exit 1
    fi

    mkdir -p "$1" && cd "$1"
    testinfo > README.md

    # Kubelet endpoints
    mkdir -p kubelet
    echo "Extracting kubelet /pods"
    kubectl exec -n scraper $pod -- datagen.sh scrape kubelet pods > kubelet/pods

    echo "Extracting kubelet /metrics/cadvisor"
    mkdir -p kubelet/metrics
    kubectl exec -n scraper $pod -- datagen.sh scrape kubelet metrics/cadvisor > kubelet/metrics/cadvisor

    echo "Extracting kubelet /stats/summary"
    mkdir -p kubelet/stats
    kubectl exec -n scraper $pod -- datagen.sh scrape kubelet stats/summary > kubelet/stats/summary

    # KSM endpoint
    mkdir -p ksm
    echo "Extracting ksm /metrics"
    kubectl exec -n scraper $pod -- datagen.sh scrape ksm > ksm/metrics

    # Control plane components
    mkdir -p controlplane/api-server
    echo "Extracting api-server /metrics"
    kubectl exec -n scraper $pod -- datagen.sh scrape controlplane apiserver > controlplane/api-server/metrics

    mkdir -p controlplane/etcd
    echo "Extracting etcd /metrics"
    #kubectl exec -n scraper $pod -- datagen.sh scrape controlplane etcd > controlplane/etcd/metrics

    mkdir -p controlplane/controller-manager
    echo "Extracting controller-manager /metrics"
    kubectl exec -n scraper $pod -- datagen.sh scrape controlplane controllermanager > controlplane/controller-manager/metrics

    mkdir -p controlplane/scheduler
    echo "Extracting scheduler /metrics"
    kubectl exec -n scraper $pod -- datagen.sh scrape controlplane scheduler > controlplane/scheduler/metrics

    # K8s objects
    echo "Generating list of kubernetes resources"
    kubedump namespaces
    kubedump services
    kubedump pods

    cd -
    cleanup
}

# bootstrap suvcommand installs the required components in the cluster to generate the testdata.
# If $SKIP_INSTALL is non-empty, it skips deploying KSM, the dummy resources, and the scrapper pod and just copies
# script inside an scraper pod that is assumed to exist already.
function bootstrap() {
    echo -e "Using context $(kubectl config current-context), is this ok?\n^C now if it is not."
    read

    if [[ -z $SKIP_INSTALL ]]; then
        echo "Installing e2e-resources chart"
        helm dependency update ../../../e2e/charts/e2e-resources > /dev/null
        helm upgrade --install e2e ../../../e2e/charts/e2e-resources -n mock --create-namespace

        echo "Installing KSM"
        helm dependency update ./deployments/ksm > /dev/null
        helm upgrade --install ksm ./deployments/ksm -n ksm --create-namespace --wait

        echo "Deploying scraper pods"
        kubectl apply -f ./deployments/scraper.yaml
    fi

    echo "Waiting for scraper pod to be ready"
    kubectl -n scraper wait --for=condition=Ready pod -l "$scrapper_selector"
    pod=$(scraper_pod "$scrapper_selector")
    if [[ -z "pod" ]]; then
        echo "Could not find scraper pod (-l $scrapper_selector)"
        exit 1
    fi
    echo "Found scraper pod $pod"

    echo "Copying datagen.sh to scraper pods"
    kubectl cp datagen.sh scraper/$pod:/bin/
}

# cleanup uninstalls the dummy resources and the scraper pod.
function cleanup() {
    echo "Removing e2e-resources chart"
    helm uninstall e2e -n mock || true
    echo "Removing ksm"
    helm uninstall ksm -n ksm || true
    echo "Removing scraper pods"
    kubectl delete -f ./deployments/scraper.yaml --wait || true
}

# scrape will curl the specified component and output the response body to standard output.
function scrape() {
    bearer="Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)"
    endpoint=unsupported://

    case $1 in
    ksm)
      endpoint=http://ksm-kube-state-metrics.ksm.svc:8080/metrics
    ;;
    kubelet)
      endpoint=https://localhost:10250/pods
    ;;
    controlplane)
      case $2 in
        etcd)
          # TODO: Support etcd
          echo "ETCD scrapping is not supported" >&2
          return 1
          ;;
        apiserver)
          endpoint=https://localhost:8443/metrics
          ;;
        controllermanager)
          endpoint=https://localhost:10257/metrics
          ;;
        scheduler)
          endpoint=https://localhost:10259/metrics
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
    kubectl get pods -l $1 -n scraper -o jsonpath='{.items[0].metadata.name}'
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
$(kubectl get nodes)
\`\`\`
EOF
}

# Script entrypoint:

command=$1

case $command in
scrape|bootstrap|cleanup|testinfo)
    shift
    $command $@
    exit $?
  ;;
esac

main $@
