#!/usr/bin/env bash
#
# Manual GKE Autopilot allowlist-confirmation e2e for nri-kubernetes.
#
# Runs test-specs-gke-autopilot.yml against an EXISTING GKE Autopilot cluster (never minikube). Each
# scenario applies its WorkloadAllowlist shape, deploys the privileged kubelet DaemonSet, and asserts
# the metrics that shape should produce.
#
# IMAGE_SOURCE selects what images the workload runs:
#   released : the published newrelic/nri-kubernetes image (chart default). Zero setup, no registry.
#   local    : build a MULTI-ARCH (amd64+arm64) image from local source and push it to $REGISTRY,
#              then run that. Multi-arch matters: GKE Autopilot nodes are usually amd64, and a
#              single-arch local build will fail to pull ("no match for platform in manifest").
#
# REGION selects the New Relic backend (US | EU | Staging | Local). When Staging, the agent is also
# deployed with global.nrStaging=true so its data lands where the API key queries.
#
# Self-guiding: config is resolved from environment -> .env -> interactive prompt. On any auth gate it
# prints the exact command for YOU to run. It NEVER switches kube-context, gcloud account, or docker
# identity (hard rule).
#
# Usage:  bash e2e/run-gke-autopilot-e2e.sh
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$SCRIPT_DIR/gke-autopilot-allowlists/.env"

# Tee all output (terminal + a timestamped log) so the latest run can be read from disk. results/ is
# gitignored. results/latest.log always points at the most recent run.
RESULTS_DIR="$SCRIPT_DIR/gke-autopilot-allowlists/results"
mkdir -p "$RESULTS_DIR"
RUN_LOG="$RESULTS_DIR/run-$(date +%Y%m%d-%H%M%S).log"
exec > >(tee "$RUN_LOG") 2>&1
ln -sf "$(basename "$RUN_LOG")" "$RESULTS_DIR/latest.log"
echo "Logging this run to: $RUN_LOG"
echo "Latest run always at: $RESULTS_DIR/latest.log"
echo

DEFAULT_KUBE_CONTEXT="$(kubectl config current-context 2>/dev/null || true)"   # your active context; the runner only verifies it, never switches
DEFAULT_REGION="US"               # US | EU | Staging | Local (GKE Autopilot testing uses a prod account)
DEFAULT_IMAGE_SOURCE="released"   # released | local
# local mode only. host/project; the image is pushed as $REGISTRY/newrelic/nri-kubernetes:$TAG so the
# CR image regex ^(.*/)?newrelic/nri-kubernetes$ still matches.
DEFAULT_REGISTRY=""               # no default; set to a registry your cluster can pull from (local mode only)
DEFAULT_TAG="e2e-gke-ap"

PROMPTED=()

if [[ -f "$ENV_FILE" ]]; then
  echo "Loading saved config from $ENV_FILE"
  set -a; # shellcheck disable=SC1090
  source "$ENV_FILE"; set +a
fi

prompt_var() { # name prompt secret default
  local name="$1" prompt="$2" secret="$3" default="$4"
  if [[ -n "${!name:-}" ]]; then return; fi
  local input=""
  if [[ "$secret" == "secret" ]]; then
    read -rsp "  $prompt: " input; echo
  elif [[ -n "$default" ]]; then
    read -rp "  $prompt [$default]: " input; input="${input:-$default}"
  else
    read -rp "  $prompt: " input
  fi
  printf -v "$name" '%s' "$input"
  PROMPTED+=("$name")
}

echo "Enter any missing config (press enter to accept a [default]):"
prompt_var KUBE_CONTEXT  "Kube context"                              "" "$DEFAULT_KUBE_CONTEXT"
prompt_var REGION        "New Relic region (US|EU|Staging|Local)"    "" "$DEFAULT_REGION"
prompt_var IMAGE_SOURCE  "Image source: 'released' or 'local'"       "" "$DEFAULT_IMAGE_SOURCE"
case "$IMAGE_SOURCE" in
  released|local) ;;
  *) echo "IMAGE_SOURCE must be 'released' or 'local' (got '$IMAGE_SOURCE')."; exit 1 ;;
esac
if [[ "$IMAGE_SOURCE" == "local" ]]; then
  prompt_var REGISTRY "Image registry (host/project)" "" "$DEFAULT_REGISTRY"
  prompt_var TAG      "Dev image tag"                 "" "$DEFAULT_TAG"
fi
prompt_var ACCOUNT_ID  "New Relic account ID (must match REGION + keys)" ""     ""
prompt_var API_KEY     "New Relic USER API key"                         secret ""
prompt_var LICENSE_KEY "New Relic INGEST license key"                    secret ""

if [[ ${#PROMPTED[@]} -gt 0 ]]; then
  read -rp "Save these to $ENV_FILE for next time? [y/N]: " save || true
  if [[ "${save:-}" =~ ^[Yy]$ ]]; then
    mkdir -p "$(dirname "$ENV_FILE")"
    {
      echo "# gke-autopilot e2e config — gitignored, DO NOT commit."
      echo "KUBE_CONTEXT=$KUBE_CONTEXT"
      echo "REGION=$REGION"
      echo "IMAGE_SOURCE=$IMAGE_SOURCE"
      echo "REGISTRY=${REGISTRY:-}"
      echo "TAG=${TAG:-}"
      echo "ACCOUNT_ID=$ACCOUNT_ID"
      echo "API_KEY=$API_KEY"
      echo "LICENSE_KEY=$LICENSE_KEY"
    } > "$ENV_FILE"
    chmod 600 "$ENV_FILE"
    echo "Saved to $ENV_FILE (chmod 600)."
  fi
fi

echo
echo "──────── config ────────"
echo "  KUBE_CONTEXT : $KUBE_CONTEXT"
echo "  REGION       : $REGION"
echo "  IMAGE_SOURCE : $IMAGE_SOURCE"
if [[ "$IMAGE_SOURCE" == "local" ]]; then
  echo "  REGISTRY     : $REGISTRY"
  echo "  TAG          : $TAG"
fi
echo "  ACCOUNT_ID   : $ACCOUNT_ID"
echo "  API_KEY      : ${API_KEY:0:4}… (${#API_KEY} chars)"
echo "  LICENSE_KEY  : ${LICENSE_KEY:0:4}… (${#LICENSE_KEY} chars)"
echo "────────────────────────"
echo

# Image override flags consumed by the spec's before-block. Empty for released mode (chart defaults).
if [[ "$IMAGE_SOURCE" == "local" ]]; then
  IMAGE_SET_ARGS="--set images.integration.registry=$REGISTRY --set images.integration.tag=$TAG --set images.integration.pullPolicy=Always"
else
  IMAGE_SET_ARGS=""
fi
# Route the agent to staging ingest when testing against the Staging backend.
if [[ "$REGION" == "Staging" ]]; then
  STAGING_SET_ARGS="--set global.nrStaging=true"
else
  STAGING_SET_ARGS=""
fi

# --- preflight: kube-context (NEVER switch it) ---
CURRENT_CTX="$(kubectl config current-context 2>/dev/null || true)"
if [[ "$CURRENT_CTX" != "$KUBE_CONTEXT" ]]; then
  cat <<EOF
Active kube-context is '$CURRENT_CTX', expected '$KUBE_CONTEXT'.
This script does NOT switch contexts. Switch it yourself, then re-run:
    kubectl config use-context $KUBE_CONTEXT
EOF
  exit 1
fi
echo "kube-context OK: $KUBE_CONTEXT"

# --- build + push multi-arch (local mode only) ---
if [[ "$IMAGE_SOURCE" == "local" ]]; then
  IMAGE_REF="$REGISTRY/newrelic/nri-kubernetes:$TAG"
  REGISTRY_HOST="${REGISTRY%%/*}"
  echo "Building integration binaries for all arches (make compile-multiarch)…"
  ( cd "$REPO_ROOT" && make compile-multiarch )
  echo "Building + pushing MULTI-ARCH image $IMAGE_REF (linux/amd64,linux/arm64)…"
  export DOCKER_BUILDKIT=1
  # buildx --push publishes a multi-arch manifest directly. amd64 is required for GKE Autopilot nodes;
  # arm64 covers arm node pools. The Dockerfile's COPY step pulls in
  # bin/nri-kubernetes-${TARGETOS}-${TARGETARCH}, so each platform gets its matching prebuilt binary.
  if ! ( cd "$REPO_ROOT" && docker buildx build --platform linux/amd64,linux/arm64 --tag "$IMAGE_REF" --push . ); then
    cat <<EOF

Multi-arch build/push failed for $IMAGE_REF.
This script does NOT change docker/gcloud auth or the buildx builder. Common fixes to run yourself:
  Configure docker auth for the registry:
      gcloud auth configure-docker $REGISTRY_HOST
  If the repository does not exist yet (adjust repo name / --location / --project to your layout):
      gcloud artifacts repositories create newrelic --repository-format=docker --location=<LOCATION> --project=<PROJECT>
  If buildx cannot emit multi-platform, create a container-driver builder once:
      docker buildx create --use --name multiarch
Then re-run this script.
EOF
    exit 1
  fi
  echo "Pushed multi-arch image."
else
  echo "Using released images (chart defaults: newrelic/nri-kubernetes). Skipping build/push."
fi

# --- run the e2e action (before-blocks in the spec consume these env vars) ---
echo "Running GKE Autopilot e2e (spec: test-specs-gke-autopilot.yml, both shapes, region=$REGION)…"
export IMAGE_SET_ARGS STAGING_SET_ARGS LICENSE_KEY ACCOUNT_ID API_KEY
cd "$SCRIPT_DIR"
go run github.com/newrelic/newrelic-integration-e2e-action@latest \
  --commit_sha=gke-autopilot-manual --retry_attempts=8 --retry_seconds=60 --region="$REGION" \
  --account_id="$ACCOUNT_ID" --api_key="$API_KEY" --license_key="$LICENSE_KEY" \
  --spec_path=test-specs-gke-autopilot.yml --verbose_mode=true --agent_enabled=false

echo "Done. Review the pass/fail summary above."
