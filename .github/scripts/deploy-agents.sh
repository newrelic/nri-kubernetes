#!/bin/bash
# Usage: ./deploy-agents.sh [--cluster <name>] [--license <key>] [--ksm-image-version <version>] [--release_candidate true] [--staging true] [--gke-autopilot true]
# Configuration can be provided via .env file or command line parameters (CLI parameters take precedence)
set -euo pipefail

# Load config from .env file if it exists
CONFIG_FILE="${CONFIG_FILE:-.env}"
if [[ -f "$CONFIG_FILE" ]]; then
  source "$CONFIG_FILE"
fi

# Set defaults (can be overridden by .env or CLI)
KSM_IMAGE_VERSION="${KSM_IMAGE_VERSION:-v2.15.0}"
USE_RC="${USE_RC:-false}"
USE_STAGING="${USE_STAGING:-false}"
USE_GKE_AUTOPILOT="${USE_GKE_AUTOPILOT:-false}"

# Parse command line arguments (these override config file)
while [[ $# -gt 0 ]]; do
  case $1 in
    --cluster)
      CLUSTER_NAME="$2"
      shift 2
      ;;
    --license)
      NEW_RELIC_LICENSE_KEY="$2"
      shift 2
      ;;
    --ksm-image-version)
      KSM_IMAGE_VERSION="$2"
      shift 2
      ;;
    --release_candidate)
      USE_RC="$2"
      shift 2
      ;;
    --staging)
      USE_STAGING="$2"
      shift 2
      ;;
    --gke-autopilot)
      USE_GKE_AUTOPILOT="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: $0 --cluster <name> --license <key> [--ksm-image-version <version>] [--release_candidate true] [--staging true] [--gke-autopilot true]"
      exit 1
      ;;
  esac
done

if [[ -z "${CLUSTER_NAME:-}" ]]; then
  echo "Error: --cluster is required (set via CLI or CLUSTER_NAME in $CONFIG_FILE)"
  exit 1
fi

if [[ -z "${NEW_RELIC_LICENSE_KEY:-}" ]]; then
  echo "Error: --license is required (set via CLI or NEW_RELIC_LICENSE_KEY in $CONFIG_FILE)"
  exit 1
fi

if ! kubectl cluster-info &>/dev/null; then
  echo "Error: Cannot connect to cluster"
  exit 1
fi

echo "Deploying to cluster: ${CLUSTER_NAME}"

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm upgrade --install kube-state-metrics prometheus-community/kube-state-metrics \
  --set image.tag="${KSM_IMAGE_VERSION}" \
  --namespace newrelic \
  --create-namespace \
  --set metricLabelsAllowlist[0]='pods=[*]' \
  --set metricLabelsAllowlist[1]='namespaces=[*]' \
  --set metricLabelsAllowlist[2]='deployments=[*]' \
  --set metricAnnotationsAllowList[0]='pods=[*]' \
  --set metricAnnotationsAllowList[1]='namespaces=[*]' \
  --set metricAnnotationsAllowList[2]='deployments=[*]'

helm repo add nri-kubernetes https://newrelic.github.io/nri-kubernetes
helm repo update

NRI_ARGS=(
  --namespace newrelic
  --create-namespace
  --set licenseKey="${NEW_RELIC_LICENSE_KEY}"
  --set cluster="${CLUSTER_NAME}"
  --set nrStaging="${USE_STAGING}"
)

if [[ "${USE_RC}" == "true" ]]; then
  NRI_ARGS+=(--set images.integration.tag=nightly)
fi

if [[ "${USE_GKE_AUTOPILOT}" == "true" ]]; then
  NRI_ARGS+=(--set global.provider=GKE_AUTOPILOT)
fi

helm upgrade --install newrelic-infrastructure nri-kubernetes/newrelic-infrastructure "${NRI_ARGS[@]}"

kubectl get pods -n newrelic
echo "Deployment complete."
