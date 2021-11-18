#!/usr/bin/env bash

set -e

scrapper_selector="app=scraper"

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

    # Kubelet endpoints
    mkdir -p kubelet
    echo "Extracting kubelet /pods"
    kubectl exec -n scraper $pod -- datagen.sh scrape kubelet pods > kubelet/pods

    echo "Extracting kubelet /metrics/cadvisor"
    mkdir -p kubelet/metrics
    kubectl exec -n scraper $pod -- datagen.sh scrape kubelet metrics/cadvisor > kubelet/metrics/cadvisor

    echo "Extracting kubelet /stats/summary"
    mkdir -p kubelet/stats
    kubectl exec -n scraper $pod -- datagen.sh scrape kubelet metrics/cadvisor > kubelet/stats/summary

    # KSM endpoint
    mkdir -p ksm
    echo "Extracting ksm /metrics"
    kubectl exec -n scraper $pod -- datagen.sh scrape ksm > ksm/metrics

    # K8s objects
    echo "Generating list of kubernetes resources"
    kubedump namespaces
    kubedump services
    kubedump pods

    cleanup
}

# bootstrap install the required components in the cluster to generate the testdata
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

function cleanup() {
    echo "Removing e2e-resources chart"
    helm uninstall e2e -n mock || true
    echo "Removing ksm"
    helm uninstall ksm -n ksm || true
    echo "Removing scraper pods"
    kubectl delete -f ./deployments/scraper.yaml --wait || true
}

function scraper_pod() {
    kubectl get pods -l $1 -n scraper -o jsonpath='{.items[0].metadata.name}'
}

function kubedump() {
    echo "Extracting kubernetes $1"
    kubectl get "$1" -o yaml --all-namespaces > "$1".yaml
}

# scrape will curl the specified component and output the response body to standard output
function scrape() {
    case $1 in
    ksm)
      endpoint=(http://ksm-kube-state-metrics.ksm.svc:8080/metrics)
    ;;
    kubelet)
      endpoint=(https://localhost:10250/pods -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)")
    ;;
    esac

    curl -ksSL "${endpoint[@]}"
}

command=$1

case $command in
scrape|bootstrap|cleanup)
    shift
    $command $@
    exit $?
  ;;
esac

main $@
