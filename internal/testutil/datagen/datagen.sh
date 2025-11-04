#!/usr/bin/env bash

set -e

# e2e-resources chart path
helm_e2e_path="../../../charts/internal/e2e-resources"

# scrapper_selector is the label with which the scraper deployment is deployed.
scrapper_selector="app=scraper"
scrapper_namespace="scraper"

# Default endpoints for minikube, extensible to some extent to other distros like kubeadm.
KSM_ENDPOINT=${KSM_ENDPOINT:-http://e2e-kube-state-metrics.${scrapper_namespace}.svc:8080/metrics}
KUBELET_ENDPOINT=${KUBELET_ENDPOINT:-https://localhost:10250}
# If control plane is not reachable (e.g. managed k8s), set DISABLE_CONTROLPLANE=1.
K8S_CONTROL_PLANE_NAMESPACE=${K8S_CONTROL_PLANE:-kube-system}
ETCD_ENDPOINT=${ETCD_ENDPOINT:-http://localhost:2381/metrics}
APISERVER_ENDPOINT=${APISERVER_ENDPOINT:-https://kubernetes.default:443/metrics}
CONTROLLERMANAGER_ENDPOINT=${CONTROLLERMANAGER_ENDPOINT:-https://localhost:10257/metrics}
SCHEDULER_ENDPOINT=${SCHEDULER_ENDPOINT:-https://localhost:10259/metrics}

# Script will apply minikube-specific actions if this variable is set to 1.
# If empty (default), minikube environment will be autodetected based on the name of the context.
IS_MINIKUBE=""

# Extra args that will be appended to the helm install e2e command.
# Values can be overridden using this and --set commands, or -f and point to a custom values file.
# Useful to tweak things specific to minikube, or toggle specific features.
# See  $helm_e2e_path/values.yaml for more details.
HELM_E2E_ARGS=""

# Time to wait for pods to settle.
# Might be useful to increase this for freshly spawned clusters on slow machines, e.g. Macbooks.
WAIT_TIMEOUT=${WAIT_TIMEOUT:-180s}

# Time to wait after bootstrap is finished (some metrics may take a while to show up)
WAIT_AFTER_BOOTSTRAP=${WAIT_AFTER_BOOTSTRAP:-30}

# kubectl command, set it up in case you need to use some kind of custom command.
# E.g.: `minikube kubectl -- `
KUBECTL_CMD=${KUBECTL_CMD:-kubectl}

# Profile name for Minikube cluster
MINIKUBE_PROFILE="${0##*/}"
MINIKUBE_PROFILE=${MINIKUBE_PROFILE%.*}
MINIKUBE_PROFILE="${MINIKUBE_PROFILE}-${1/./-}"

# Supported Kubernetes versions to test against in format "v1.31.1".  Specify one patch version per supported minor version.
# The patch version is used for the test, but the test is considered valid for all patches in the minor version.
K8S_PATCH_VERSIONS=(
  "v1.34.0"
  "v1.33.0"
  "v1.32.0"
  "v1.31.0"
  "v1.30.0"
)
# KSM version to use for the K8S_PATCH_VERSIONS, matched by index.
KSM_IMAGE_VERSIONS=(
  "v2.16.0"
  "v2.16.0"
  "v2.16.0"
  "v2.13.0"
  "v2.13.0"
)
# K8S_MINOR_VERSIONS are formatted like "1.28".
# An RC version like "v1.28.0-rc.1" translates to "1.28", using "data/1_28" and "1_28-exceptions.yml".
K8S_MINOR_VERSIONS=()
for index in "${!K8S_PATCH_VERSIONS[@]}"; do
  K8S_MINOR_VERSIONS[index]=$(echo ${K8S_PATCH_VERSIONS[index]} | sed -n 's/v\([0-9]\.[0-9][0-9]*\)\.[0-9].*/\1/p')
done


# main subcommand runs the whole flow of the script: Bootstrap, scrape, and cleanup
function main() {
    if [[ $# -eq 0 ]]; then
      echo "Please provide Kubernetes version as MAJOR.MINOR (e.g., ${0##*/} 1.28)"
      exit 1
    fi

    setup $1

    K8S_VERSION_JSON=$($KUBECTL_CMD version -o json)
    K8S_VERSION_MAJOR=$(echo $K8S_VERSION_JSON | jq '.serverVersion.major | tonumber')
    K8S_VERSION_MINOR=$(echo $K8S_VERSION_JSON | jq '.serverVersion.minor | tonumber')
    K8S_VERSION=$K8S_VERSION_MAJOR"."$K8S_VERSION_MINOR
    OUTPUT_FOLDER=$(echo $K8S_VERSION | sed 's/\./_/')

    # Install scraper pod and e2e resources.
    bootstrap "$K8S_VERSION"

    # Scraper pod is a deployment, so we need to locate it.
    pod=$(scraper_pod "$scrapper_selector")
    if [[ -z "$pod" ]]; then
        echo "Could not find scraper pod (-l $scrapper_selector)"
        cleanup
        exit 1
    fi

    # Collect cds to the specified dir so we invoke it on a subshell
    ( collect "$OUTPUT_FOLDER" )

    cleanup
}

function setup() {
    # Arg 1 is the K8s minor version like "1.28"
    for index in "${!K8S_MINOR_VERSIONS[@]}"; do
      if [[ ${K8S_MINOR_VERSIONS[index]} = "$1" ]]; then
        K8S_VERSION=${K8S_PATCH_VERSIONS[index]}
      fi
    done
    if [ -z "${K8S_VERSION}" ]; then
        echo "ERROR (${0##*/}:$LINENO): specific Kubernetes version needs to be defined for '$1'"
        exit 1
    fi
    echo "minikube start --profile=$MINIKUBE_PROFILE --container-runtime=containerd --driver=docker --kubernetes-version=$K8S_VERSION"
    minikube start \
      --profile=$MINIKUBE_PROFILE \
      --container-runtime=containerd \
      --driver=docker \
      --kubernetes-version=$K8S_VERSION
}

function collect() {
    # Dump test info
    mkdir -p "$1" && cd "$1"
    testinfo > README.md

    # Kubelet endpoints
    mkdir -p kubelet
    echo "Extracting kubelet /pods"
    $KUBECTL_CMD exec -n $scrapper_namespace $pod -- datagen.sh scrape kubelet pods > kubelet/pods

    echo "Extracting kubelet /metrics/cadvisor"
    mkdir -p kubelet/metrics
    $KUBECTL_CMD exec -n $scrapper_namespace $pod -- datagen.sh scrape kubelet metrics/cadvisor > kubelet/metrics/cadvisor

    echo "Extracting kubelet /stats/summary"
    mkdir -p kubelet/stats
    $KUBECTL_CMD exec -n $scrapper_namespace $pod -- datagen.sh scrape kubelet stats/summary > kubelet/stats/summary

    # KSM endpoint
    mkdir -p ksm
    echo "Extracting ksm /metrics"
    $KUBECTL_CMD exec -n $scrapper_namespace $pod -- datagen.sh scrape ksm > ksm/metrics

    if [[ "$DISABLE_CONTROLPLANE" != "1" ]]; then
        # Control plane components
        mkdir -p controlplane/api-server
        echo "Extracting api-server /metrics"
        $KUBECTL_CMD exec -n $scrapper_namespace $pod -- datagen.sh scrape controlplane apiserver > controlplane/api-server/metrics

        mkdir -p controlplane/etcd
        echo "Extracting etcd /metrics"
        $KUBECTL_CMD exec -n $scrapper_namespace $pod -- datagen.sh scrape controlplane etcd > controlplane/etcd/metrics

        mkdir -p controlplane/controller-manager
        echo "Extracting controller-manager /metrics"
        $KUBECTL_CMD exec -n $scrapper_namespace $pod -- datagen.sh scrape controlplane controllermanager > controlplane/controller-manager/metrics

        mkdir -p controlplane/scheduler
        echo "Extracting scheduler /metrics"
        $KUBECTL_CMD exec -n $scrapper_namespace $pod -- datagen.sh scrape controlplane scheduler > controlplane/scheduler/metrics
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
    echo "Waiting for Kubernetes control plane resources to settle"
    $KUBECTL_CMD --namespace $K8S_CONTROL_PLANE_NAMESPACE wait --timeout=$WAIT_TIMEOUT --for=condition=Ready pods --all > /dev/null
    echo "Waiting for DNS service to settle"  # It takes longer than the rest of the control plane to be spun
    wait_for_pod $K8S_CONTROL_PLANE_NAMESPACE "-l k8s-app=kube-dns" Ready

    ctx=$($KUBECTL_CMD config current-context)
    if [[ -z "$IS_MINIKUBE" && "$ctx" = "$MINIKUBE_PROFILE" ]]; then
        echo "Assuming minikube distribution since context is \"$ctx\""
        echo "Set IS_MINIKUBE to 0 or 1 to override autodetection"
        IS_MINIKUBE=1
    else
        echo
        echo -e "Using context $ctx, is this ok?\n^C now if it is not."
        read
    fi

    if [[ -z $SKIP_INSTALL ]]; then
        echo "Installing e2e-resources chart"
        if [[ "$IS_MINIKUBE" = "1" ]]; then
            # Enable PVC if we are in minikube
            minikube_args="--set persistentVolumeClaim.enabled=true --set loadBalancerService.fakeIP=127.1.2.3"
            minikube addons enable metrics-server --profile=$MINIKUBE_PROFILE 
            echo "Waiting for metrics-server to settle"
            wait_for_pod $K8S_CONTROL_PLANE_NAMESPACE "-l k8s-app=metrics-server" Ready
        fi

        echo "Updating helm dependencies"

        for index in "${!K8S_MINOR_VERSIONS[@]}"; do
          if [[ ${K8S_MINOR_VERSIONS[index]} = "$1" ]]; then
            KSM_IMAGE_VERSION=${KSM_IMAGE_VERSIONS[index]}
          fi
        done
        if [ -z "${KSM_IMAGE_VERSION}" ]; then
          echo "ERROR (${0##*/}:$LINENO): KSM image version needs to be defined for Kubernetes version $1"
          cleanup
          exit 1
        fi
        echo "Using KSM image $KSM_IMAGE_VERSION"
        
        helm dependency update $helm_e2e_path \
          --kube-context $MINIKUBE_PROFILE \
          > /dev/null
        helm upgrade --install e2e $helm_e2e_path \
          --kube-context $MINIKUBE_PROFILE \
          --namespace $scrapper_namespace --create-namespace \
          --set scraper.enabled=true \
          --set persistentVolume.enabled=true \
          --set kube-state-metrics.image.tag=${KSM_IMAGE_VERSION} \
          $minikube_args \
          $HELM_E2E_ARGS

        echo "Waiting for KSM to become ready"
        wait_for_pod $scrapper_namespace "-l app.kubernetes.io/name=kube-state-metrics" Ready

        echo "Waiting for E2E resources to settle"
        wait_for_pod $scrapper_namespace "-l app=hpa" Ready
        wait_for_pod $scrapper_namespace "-l app=daemonset" Ready
        wait_for_pod $scrapper_namespace "-l app=statefulset" Ready
        wait_for_pod $scrapper_namespace "-l app=deployment" Ready
        wait_for_pod $scrapper_namespace "-l app=cronjob" Initialized
        wait_for_pod $scrapper_namespace "-l app=failjob" Initialized
        wait_for_pod $scrapper_namespace "-l app=creating" Initialized
    fi

    echo "Waiting for scraper pod to be ready"
    wait_for_pod $scrapper_namespace "-l $scrapper_selector" Ready
    pod=$(scraper_pod "$scrapper_selector")
    if [[ -z "$pod" ]]; then
        echo "Could not find scraper pod (-l $scrapper_selector)"
        cleanup
        exit 1
    fi
    echo "Found scraper pod $pod"

    echo "Copying datagen.sh to scraper pods"
    $KUBECTL_CMD cp datagen.sh $scrapper_namespace/$pod:/bin/

    echo "Waiting $WAIT_AFTER_BOOTSTRAP seconds..."
    sleep $WAIT_AFTER_BOOTSTRAP
}

# wait_for_pod checks for a pod in a given namespace and label selector to show a specific condition
function wait_for_pod() {
  NAMESPACE=$1
  SELECTOR=$2
  CONDITION=$3

  while [[ -z $($KUBECTL_CMD --namespace $NAMESPACE  get pod $SELECTOR --output NAME) ]]; do
    # echo "Waiting for $SELECTOR to show up"
    sleep 5
  done
  # echo "$SELECTOR showed up!"

  $KUBECTL_CMD --namespace $NAMESPACE wait --timeout=$WAIT_TIMEOUT --for=condition=$CONDITION pod $SELECTOR > /dev/null
}

# cleanup uninstalls the dummy resources and the scraper pod.
function cleanup() {
    if [[ "$DISABLE_CLEANUP" != "1" ]]; then
      echo "Removing e2e-resources chart"
      helm uninstall e2e \
      --kube-context $MINIKUBE_PROFILE \
      --namespace $scrapper_namespace \
      --wait \
      2> /dev/null || true

      echo "minikube delete --profile $MINIKUBE_PROFILE"
      minikube delete --profile $MINIKUBE_PROFILE
    fi
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
    $KUBECTL_CMD get pods -l $1 -n $scrapper_namespace -o jsonpath='{.items[0].metadata.name}'
}

# kubedump is a helper function that dumps the specified Kubernetes resource to a yaml file of the same name.
function kubedump() {
    echo "Extracting kubernetes $1"
    $KUBECTL_CMD get "$1" -o yaml --all-namespaces > "$1".yaml
}

# Testinfo is a helper function that returns a markdown-formatted string with basic info of the environment where this
# script was run.
function testinfo() {
cat <<EOF
# Static test data generated by $(whoami) on $(date)

### \`nri-kubernetes\` commit
\`\`\`
$(git rev-parse HEAD)
\`\`\`

\`git status --short\`

\`\`\`
$(git status --short)
\`\`\`

### \`$ $KUBECTL_CMD version\`
\`\`\`
$($KUBECTL_CMD version 2> /dev/null)
\`\`\`

### Kubernetes nodes
\`\`\`
$($KUBECTL_CMD get nodes -o wide)
\`\`\`
EOF
}

# Generate static test data for all supported versions
function all_versions() {
  for version in "${K8S_MINOR_VERSIONS[@]}"; do
    ./datagen.sh $version || exit 1;
  done

  for version in "${K8S_MINOR_VERSIONS[@]}"; do
    OUTPUT_FOLDER=$(echo $version | sed 's/\./_/')
    rm -rf ../data/$OUTPUT_FOLDER;
    mv $OUTPUT_FOLDER ../data/;
  done
}

# Script entrypoint
command=$1

case $command in
scrape|bootstrap|cleanup|testinfo|all_versions)
    shift
    $command "$@"
    exit $?
  ;;
esac

main "$@"
