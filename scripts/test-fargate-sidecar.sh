#!/bin/bash
# test-fargate-sidecar.sh - Test Fargate sidecar mode on any Kubernetes cluster
#
# Usage:
#   ./scripts/test-fargate-sidecar.sh [options]
#
# Options:
#   --cluster-type TYPE    Cluster type (auto-detected if not set):
#                          Local:  kind, k3d, minikube, microk8s, k3s, k0s
#                          Remote: gke, eks, aks, generic
#   --registry REGISTRY    Container registry for pushing images
#                          Required for: remote clusters, or local clusters accessed remotely
#                          Optional for: microk8s (has built-in registry at localhost:32000)
#   --local-cluster NAME   Local cluster name for kind/k3d/minikube (auto-detected from context if not set)
#                          Examples: --local-cluster my-cluster (for k3d-my-cluster or kind-my-cluster)
#   --remote               Treat cluster as remote (use registry instead of local import)
#   --ssh-host HOST        SSH host for remote local clusters (e.g., user@192.168.1.100)
#   --image-tag TAG        Image tag to use (default: dev-test)
#   --node NODE            Existing node to use as Fargate simulation (auto-selects if not set)
#   --namespace NS         Namespace for test workload (default: fargate-test)
#   --cluster-name NAME    Cluster name for New Relic (default: fargate-sidecar-test)
#   --license-key KEY      New Relic license key (or set NEWRELIC_LICENSE_KEY env var)
#   --skip-build           Skip building the binary and image
#   --skip-helm            Skip helm install/upgrade
#   --cleanup              Remove all test resources and exit
#   --help                 Show this help message
#
# Examples:
#   # Local k3d cluster
#   ./scripts/test-fargate-sidecar.sh --license-key $NR_LICENSE_KEY
#
#   # Local microk8s with built-in registry
#   ./scripts/test-fargate-sidecar.sh --cluster-type microk8s --license-key $NR_LICENSE_KEY
#
#   # Remote k3s cluster via SSH
#   ./scripts/test-fargate-sidecar.sh --cluster-type k3s --ssh-host user@myserver --license-key $NR_LICENSE_KEY
#
#   # Remote cluster with registry
#   ./scripts/test-fargate-sidecar.sh --cluster-type generic --registry docker.io/myuser --license-key $NR_LICENSE_KEY
#
#   # EKS/GKE with ECR/GCR
#   ./scripts/test-fargate-sidecar.sh --cluster-type eks --registry 123456789.dkr.ecr.us-east-1.amazonaws.com --license-key $NR_LICENSE_KEY

set -e

# Default values
CLUSTER_TYPE=""
REGISTRY=""
IMAGE_TAG="dev-test"
TARGET_NODE=""
NAMESPACE="fargate-test"
CLUSTER_NAME="fargate-sidecar-test"
LICENSE_KEY="${NEWRELIC_LICENSE_KEY:-}"
SKIP_BUILD=false
SKIP_HELM=false
CLEANUP=false
REMOTE=false
SSH_HOST=""
LOCAL_CLUSTER=""  # Local cluster name for kind/k3d/minikube
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

usage() {
    head -25 "$0" | tail -20
    exit 0
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --cluster-type) CLUSTER_TYPE="$2"; shift 2 ;;
        --registry) REGISTRY="$2"; shift 2 ;;
        --image-tag) IMAGE_TAG="$2"; shift 2 ;;
        --node) TARGET_NODE="$2"; shift 2 ;;
        --namespace) NAMESPACE="$2"; shift 2 ;;
        --cluster-name) CLUSTER_NAME="$2"; shift 2 ;;
        --license-key) LICENSE_KEY="$2"; shift 2 ;;
        --skip-build) SKIP_BUILD=true; shift ;;
        --skip-helm) SKIP_HELM=true; shift ;;
        --cleanup) CLEANUP=true; shift ;;
        --remote) REMOTE=true; shift ;;
        --ssh-host) SSH_HOST="$2"; REMOTE=true; shift 2 ;;
        --local-cluster) LOCAL_CLUSTER="$2"; shift 2 ;;
        --help) usage ;;
        *) log_error "Unknown option: $1"; usage ;;
    esac
done

# Detect cluster type if not specified
detect_cluster_type() {
    if [[ -n "$CLUSTER_TYPE" ]]; then
        return
    fi

    local context=$(kubectl config current-context 2>/dev/null || echo "")
    local os_image=$(kubectl get nodes -o jsonpath='{.items[0].status.nodeInfo.osImage}' 2>/dev/null || echo "")
    local kubelet_version=$(kubectl get nodes -o jsonpath='{.items[0].status.nodeInfo.kubeletVersion}' 2>/dev/null || echo "")

    if [[ "$context" == *"k3d"* ]]; then
        CLUSTER_TYPE="k3d"
    elif [[ "$context" == *"kind"* ]]; then
        CLUSTER_TYPE="kind"
    elif [[ "$context" == *"minikube"* ]]; then
        CLUSTER_TYPE="minikube"
    elif [[ "$context" == *"microk8s"* ]] || [[ "$kubelet_version" == *"microk8s"* ]]; then
        CLUSTER_TYPE="microk8s"
    elif [[ "$context" == *"gke"* ]]; then
        CLUSTER_TYPE="gke"
    elif [[ "$context" == *"eks"* ]] || [[ "$context" == *"aws"* ]]; then
        CLUSTER_TYPE="eks"
    elif [[ "$context" == *"aks"* ]] || [[ "$context" == *"azure"* ]]; then
        CLUSTER_TYPE="aks"
    elif [[ "$kubelet_version" == *"k3s"* ]] || [[ "$os_image" == *"k3s"* ]] || [[ "$os_image" == *"K3s"* ]]; then
        CLUSTER_TYPE="k3s"
    elif [[ "$kubelet_version" == *"k0s"* ]]; then
        CLUSTER_TYPE="k0s"
    else
        CLUSTER_TYPE="generic"
    fi

    log_info "Auto-detected cluster type: $CLUSTER_TYPE"
}

# Detect or validate local cluster name for kind/k3d/minikube
detect_local_cluster() {
    local context=$(kubectl config current-context 2>/dev/null || echo "")

    if [[ -n "$LOCAL_CLUSTER" ]]; then
        log_info "Using specified local cluster: $LOCAL_CLUSTER"
        return
    fi

    case $CLUSTER_TYPE in
        k3d)
            # k3d context format: k3d-<cluster-name>
            LOCAL_CLUSTER=$(echo "$context" | sed 's/^k3d-//')
            # Validate cluster exists
            if ! k3d cluster list 2>/dev/null | grep -q "^${LOCAL_CLUSTER}"; then
                log_warn "Could not verify k3d cluster '$LOCAL_CLUSTER'"
                # Try to get from k3d directly
                LOCAL_CLUSTER=$(k3d cluster list -o json 2>/dev/null | jq -r '.[0].name' 2>/dev/null || echo "$LOCAL_CLUSTER")
            fi
            ;;
        kind)
            # kind context format: kind-<cluster-name>
            LOCAL_CLUSTER=$(echo "$context" | sed 's/^kind-//')
            # Validate cluster exists
            if ! kind get clusters 2>/dev/null | grep -q "^${LOCAL_CLUSTER}$"; then
                log_warn "Could not verify kind cluster '$LOCAL_CLUSTER'"
                # Try to get from kind directly
                LOCAL_CLUSTER=$(kind get clusters 2>/dev/null | head -1 || echo "$LOCAL_CLUSTER")
            fi
            ;;
        minikube)
            # minikube uses profiles, default is 'minikube'
            LOCAL_CLUSTER=$(minikube profile list -o json 2>/dev/null | jq -r '.valid[0].Name' 2>/dev/null || echo "minikube")
            # Or try from context
            if [[ -z "$LOCAL_CLUSTER" ]] || [[ "$LOCAL_CLUSTER" == "null" ]]; then
                LOCAL_CLUSTER="minikube"
            fi
            ;;
        *)
            LOCAL_CLUSTER=""
            ;;
    esac

    if [[ -n "$LOCAL_CLUSTER" ]]; then
        log_info "Detected local cluster name: $LOCAL_CLUSTER"
    fi
}

# Get image name based on registry
get_image_name() {
    local mode="${1:-normal}"
    local tag_suffix=""
    [[ "$mode" == "unprivileged" ]] && tag_suffix="-unprivileged"

    if [[ -n "$REGISTRY" ]]; then
        echo "${REGISTRY}/nri-kubernetes-sidecar:${IMAGE_TAG}${tag_suffix}"
    else
        echo "newrelic/nri-kubernetes-sidecar:${IMAGE_TAG}${tag_suffix}"
    fi
}

# Build binary and Docker image
build_image() {
    if [[ "$SKIP_BUILD" == "true" ]]; then
        log_info "Skipping build (--skip-build)"
        return
    fi

    log_info "Building nri-kubernetes binary..."
    cd "$REPO_ROOT"

    # Determine target architecture
    local arch=$(uname -m)
    case $arch in
        x86_64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
    esac

    # Build binary
    GOOS=linux GOARCH=$arch go build -o "bin/nri-kubernetes-linux-${arch}" ./cmd/nri-kubernetes

    log_info "Building sidecar Docker image..."

    # Build both normal and unprivileged variants
    docker build \
        --build-arg MODE=normal \
        --platform "linux/${arch}" \
        -t "$(get_image_name normal)" \
        -f Dockerfile.sidecar .

    docker build \
        --build-arg MODE=unprivileged \
        --platform "linux/${arch}" \
        -t "$(get_image_name unprivileged)" \
        -f Dockerfile.sidecar .

    log_success "Images built successfully"
}

# Load image via SSH to remote local cluster
load_image_ssh() {
    local image=$1
    local remote_cmd=""

    log_info "Loading image via SSH to $SSH_HOST..."

    case $CLUSTER_TYPE in
        k3s)
            remote_cmd="sudo k3s ctr images import -"
            ;;
        k0s)
            remote_cmd="sudo k0s ctr images import -"
            ;;
        microk8s)
            remote_cmd="microk8s ctr image import -"
            ;;
        kind)
            docker save "$image" | ssh "$SSH_HOST" "docker load && kind load docker-image $image --name $LOCAL_CLUSTER"
            return
            ;;
        k3d)
            docker save "$image" | ssh "$SSH_HOST" "docker load && k3d image import $image -c $LOCAL_CLUSTER"
            return
            ;;
        minikube)
            docker save "$image" | ssh "$SSH_HOST" "docker load && minikube image load $image -p $LOCAL_CLUSTER"
            return
            ;;
        *)
            log_error "SSH loading not supported for cluster type: $CLUSTER_TYPE"
            exit 1
            ;;
    esac

    # Stream image directly to containerd
    docker save "$image" | ssh "$SSH_HOST" "$remote_cmd"
}

# Load/push image to cluster
load_image() {
    local image_normal=$(get_image_name normal)
    local image_unprivileged=$(get_image_name unprivileged)

    log_info "Loading images to cluster (type: $CLUSTER_TYPE, remote: $REMOTE)..."

    # If using registry, tag and push
    if [[ -n "$REGISTRY" ]]; then
        log_info "Pushing images to registry: $REGISTRY"

        # If images were built without registry prefix, tag them
        local base_image_normal="newrelic/nri-kubernetes-sidecar:${IMAGE_TAG}"
        local base_image_unprivileged="newrelic/nri-kubernetes-sidecar:${IMAGE_TAG}-unprivileged"

        if [[ "$image_normal" != "$base_image_normal" ]]; then
            # Tag with registry prefix if not already tagged
            if ! docker image inspect "$image_normal" &>/dev/null; then
                log_info "Tagging $base_image_normal -> $image_normal"
                docker tag "$base_image_normal" "$image_normal"
            fi
            if ! docker image inspect "$image_unprivileged" &>/dev/null; then
                log_info "Tagging $base_image_unprivileged -> $image_unprivileged"
                docker tag "$base_image_unprivileged" "$image_unprivileged"
            fi
        fi

        docker push "$image_normal"
        docker push "$image_unprivileged"
        log_success "Images pushed to registry"
        return
    fi

    # If remote flag set but no registry, try SSH
    if [[ "$REMOTE" == "true" ]]; then
        if [[ -z "$SSH_HOST" ]]; then
            log_error "Remote clusters require --registry or --ssh-host"
            exit 1
        fi
        load_image_ssh "$image_normal"
        load_image_ssh "$image_unprivileged"
        log_success "Images loaded via SSH"
        return
    fi

    # Local cluster - use native tools
    case $CLUSTER_TYPE in
        k3d)
            log_info "Loading images to k3d cluster: $LOCAL_CLUSTER"
            k3d image import "$image_normal" "$image_unprivileged" -c "$LOCAL_CLUSTER"
            ;;
        kind)
            log_info "Loading images to kind cluster: $LOCAL_CLUSTER"
            kind load docker-image "$image_normal" "$image_unprivileged" --name "$LOCAL_CLUSTER"
            ;;
        minikube)
            log_info "Loading images to minikube profile: $LOCAL_CLUSTER"
            # Check if using docker driver (can use docker env) or other
            if minikube docker-env -p "$LOCAL_CLUSTER" &>/dev/null; then
                log_info "Using minikube docker environment..."
                eval $(minikube docker-env -p "$LOCAL_CLUSTER")
                # Rebuild in minikube's docker
                cd "$REPO_ROOT"
                docker build --build-arg MODE=normal -t "$image_normal" -f Dockerfile.sidecar .
                docker build --build-arg MODE=unprivileged -t "$image_unprivileged" -f Dockerfile.sidecar .
            else
                minikube image load "$image_normal" -p "$LOCAL_CLUSTER"
                minikube image load "$image_unprivileged" -p "$LOCAL_CLUSTER"
            fi
            ;;
        microk8s)
            # microk8s has a built-in registry at localhost:32000
            if microk8s status | grep -q "registry: enabled" 2>/dev/null; then
                log_info "Using microk8s built-in registry..."
                local mk8s_image_normal="localhost:32000/nri-kubernetes-sidecar:${IMAGE_TAG}"
                local mk8s_image_unprivileged="localhost:32000/nri-kubernetes-sidecar:${IMAGE_TAG}-unprivileged"
                docker tag "$image_normal" "$mk8s_image_normal"
                docker tag "$image_unprivileged" "$mk8s_image_unprivileged"
                docker push "$mk8s_image_normal"
                docker push "$mk8s_image_unprivileged"
                # Update image names for helm values
                REGISTRY="localhost:32000"
            else
                # Import directly via microk8s ctr
                log_info "Importing images to microk8s..."
                docker save "$image_normal" | microk8s ctr image import -
                docker save "$image_unprivileged" | microk8s ctr image import -
            fi
            ;;
        k3s)
            # Local k3s - import via ctr
            if command -v k3s &>/dev/null; then
                log_info "Importing images to k3s..."
                docker save "$image_normal" | sudo k3s ctr images import -
                docker save "$image_unprivileged" | sudo k3s ctr images import -
            else
                log_error "k3s CLI not found. Use --ssh-host for remote k3s or --registry"
                exit 1
            fi
            ;;
        k0s)
            # Local k0s - import via ctr
            if command -v k0s &>/dev/null; then
                log_info "Importing images to k0s..."
                docker save "$image_normal" | sudo k0s ctr images import -
                docker save "$image_unprivileged" | sudo k0s ctr images import -
            else
                log_error "k0s CLI not found. Use --ssh-host for remote k0s or --registry"
                exit 1
            fi
            ;;
        gke|eks|aks|generic)
            log_error "Cloud/remote clusters require --registry to push images"
            log_error "Example: --registry gcr.io/my-project or --registry docker.io/myuser"
            exit 1
            ;;
        *)
            log_error "Unknown cluster type: $CLUSTER_TYPE"
            exit 1
            ;;
    esac

    log_success "Images loaded to cluster"
}

# Setup Fargate simulation node
setup_fargate_node() {
    if [[ -n "$TARGET_NODE" ]]; then
        log_info "Using specified node: $TARGET_NODE"
    else
        # Try to create a new node for clusters that support it
        case $CLUSTER_TYPE in
            k3d)
                if kubectl get node k3d-fargate-sim-0 &>/dev/null; then
                    log_info "Fargate simulation node already exists"
                    TARGET_NODE="k3d-fargate-sim-0"
                else
                    log_info "Creating new k3d agent node for Fargate simulation in cluster: $LOCAL_CLUSTER"
                    k3d node create fargate-sim --cluster "$LOCAL_CLUSTER" --role agent --wait
                    TARGET_NODE="k3d-fargate-sim-0"
                    sleep 5
                fi
                ;;
            kind)
                # kind doesn't support dynamic node creation, use existing worker
                TARGET_NODE=$(kubectl get nodes --no-headers -o custom-columns=":metadata.name" | grep -E "worker|node" | head -1)
                if [[ -z "$TARGET_NODE" ]]; then
                    TARGET_NODE=$(kubectl get nodes --no-headers -o custom-columns=":metadata.name" | grep -v control-plane | head -1)
                fi
                ;;
            minikube)
                # minikube can add nodes
                if ! kubectl get node minikube-fargate-sim &>/dev/null; then
                    if minikube node list 2>/dev/null | grep -q "fargate-sim"; then
                        TARGET_NODE="minikube-fargate-sim"
                    else
                        log_info "Adding minikube node for Fargate simulation..."
                        minikube node add --worker 2>/dev/null || true
                        TARGET_NODE=$(kubectl get nodes --no-headers -o custom-columns=":metadata.name" | grep -v "^minikube$" | tail -1)
                    fi
                else
                    TARGET_NODE="minikube-fargate-sim"
                fi
                # Fallback to main node if no workers
                [[ -z "$TARGET_NODE" ]] && TARGET_NODE="minikube"
                ;;
            microk8s|k3s|k0s)
                # Single-node clusters typically - use the main node or first worker
                TARGET_NODE=$(kubectl get nodes --no-headers -o custom-columns=":metadata.name" | grep -v master | grep -v control-plane | head -1)
                [[ -z "$TARGET_NODE" ]] && TARGET_NODE=$(kubectl get nodes --no-headers -o custom-columns=":metadata.name" | head -1)
                ;;
            *)
                # Cloud/generic - use first worker node
                TARGET_NODE=$(kubectl get nodes --no-headers -o custom-columns=":metadata.name" | grep -v master | grep -v control-plane | head -1)
                [[ -z "$TARGET_NODE" ]] && TARGET_NODE=$(kubectl get nodes --no-headers -o custom-columns=":metadata.name" | head -1)
                ;;
        esac

        if [[ -z "$TARGET_NODE" ]]; then
            log_error "Could not find a suitable node for Fargate simulation"
            exit 1
        fi
        log_info "Selected node: $TARGET_NODE"
    fi

    # Add Fargate labels and taint
    log_info "Adding Fargate labels and taint to node: $TARGET_NODE"
    kubectl label node "$TARGET_NODE" eks.amazonaws.com/compute-type=fargate --overwrite
    kubectl taint node "$TARGET_NODE" eks.amazonaws.com/compute-type=fargate:NoSchedule --overwrite 2>/dev/null || true

    log_success "Fargate simulation node configured: $TARGET_NODE"
}

# Create namespace and RBAC
setup_rbac() {
    log_info "Setting up namespace and RBAC..."

    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: fargate-sidecar-test
  namespace: $NAMESPACE
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: fargate-sidecar-test
rules:
- apiGroups: [""]
  resources: ["nodes", "nodes/metrics", "nodes/stats", "nodes/proxy"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: fargate-sidecar-test
subjects:
- kind: ServiceAccount
  name: fargate-sidecar-test
  namespace: $NAMESPACE
roleRef:
  kind: ClusterRole
  name: fargate-sidecar-test
  apiGroup: rbac.authorization.k8s.io
EOF

    log_success "RBAC configured"
}

# Deploy test workload
deploy_workload() {
    log_info "Deploying test workload..."

    cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fargate-test-workload
  namespace: $NAMESPACE
  labels:
    app: fargate-test-workload
spec:
  replicas: 1
  selector:
    matchLabels:
      app: fargate-test-workload
  template:
    metadata:
      labels:
        app: fargate-test-workload
        eks.amazonaws.com/fargate-profile: test
    spec:
      serviceAccountName: fargate-sidecar-test
      tolerations:
      - key: "eks.amazonaws.com/compute-type"
        operator: "Equal"
        value: "fargate"
        effect: "NoSchedule"
      nodeSelector:
        eks.amazonaws.com/compute-type: fargate
      containers:
      - name: test-app
        image: nginx:alpine
        ports:
        - containerPort: 80
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"
EOF

    log_info "Waiting for test workload to be ready..."
    kubectl rollout status deployment/fargate-test-workload -n "$NAMESPACE" --timeout=120s

    log_success "Test workload deployed"
}

# Generate values file for helm
generate_values() {
    local values_file="${REPO_ROOT}/scripts/values-fargate-test-generated.yaml"
    local image=$(get_image_name unprivileged)
    local pull_policy="IfNotPresent"

    # Use Never for local clusters
    case $CLUSTER_TYPE in
        k3d|kind|minikube) pull_policy="Never" ;;
    esac

    log_info "Generating Helm values file: $values_file"

    cat > "$values_file" <<EOF
# Auto-generated values for Fargate sidecar testing
# Generated by: test-fargate-sidecar.sh
# Cluster type: $CLUSTER_TYPE

newrelic-infrastructure:
  enabled: true

kube-state-metrics:
  enabled: true

nri-kube-events:
  enabled: false

newrelic-logging:
  enabled: false

nri-prometheus:
  enabled: false

newrelic-prometheus-agent:
  enabled: false

newrelic-pixie:
  enabled: false

pixie-chart:
  enabled: false

nr-ebpf-agent:
  enabled: false

k8s-agents-operator:
  enabled: false

nri-metadata-injection:
  enabled: true

newrelic-k8s-metrics-adapter:
  enabled: false

newrelic-infra-operator:
  enabled: true
  config:
    infraAgentInjection:
      agentConfig:
        image:
          repository: ${image%:*}
          tag: ${image##*:}
          pullPolicy: $pull_policy
        configSelectors:
          - extraEnvVars:
              DISABLE_KUBE_STATE_METRICS: "true"
              NRI_KUBERNETES_VERBOSE: "false"
              NRI_KUBERNETES_SINK_HTTP_TIMEOUT: "30s"
              NRI_KUBERNETES_SINK_HTTP_RETRIES: "0"

global:
  cluster: $CLUSTER_NAME
  licenseKey: "$LICENSE_KEY"
  fargate: true
  lowDataMode: true
EOF

    echo "$values_file"
}

# Install or upgrade Helm release
install_helm() {
    if [[ "$SKIP_HELM" == "true" ]]; then
        log_info "Skipping Helm install (--skip-helm)"
        return
    fi

    if [[ -z "$LICENSE_KEY" ]]; then
        log_warn "No license key provided. Skipping Helm install."
        log_warn "Set --license-key or NEWRELIC_LICENSE_KEY to enable."
        return
    fi

    local values_file=$(generate_values)

    log_info "Installing/upgrading New Relic bundle..."

    # Add helm repo if not exists
    helm repo add newrelic https://newrelic.github.io/helm-charts/ 2>/dev/null || true
    helm repo update newrelic

    # Install or upgrade
    helm upgrade --install newrelic-bundle newrelic/nri-bundle \
        -n newrelic --create-namespace \
        -f "$values_file" \
        --wait --timeout 5m

    log_success "Helm release installed/upgraded"
}

# Cleanup all test resources
cleanup() {
    log_info "Cleaning up test resources..."

    # Delete test namespace
    kubectl delete namespace "$NAMESPACE" --ignore-not-found --timeout=60s

    # Find nodes with Fargate label
    local fargate_nodes=$(kubectl get nodes -l eks.amazonaws.com/compute-type=fargate -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)

    # Remove Fargate labels/taints from nodes
    for node in $fargate_nodes; do
        log_info "Removing Fargate labels/taint from node: $node"
        kubectl label node "$node" eks.amazonaws.com/compute-type- 2>/dev/null || true
        kubectl taint node "$node" eks.amazonaws.com/compute-type=fargate:NoSchedule- 2>/dev/null || true
    done

    # Delete RBAC
    kubectl delete clusterrolebinding fargate-sidecar-test --ignore-not-found
    kubectl delete clusterrole fargate-sidecar-test --ignore-not-found

    # Cluster-specific cleanup
    case $CLUSTER_TYPE in
        k3d)
            if kubectl get node k3d-fargate-sim-0 &>/dev/null; then
                read -p "Delete k3d fargate-sim node? [y/N] " -n 1 -r
                echo
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    k3d node delete k3d-fargate-sim-0 -c "$LOCAL_CLUSTER" 2>/dev/null || true
                fi
            fi
            ;;
        minikube)
            # Check for extra nodes added for testing
            local extra_nodes=$(minikube node list 2>/dev/null | grep -v "^minikube" | awk '{print $1}')
            if [[ -n "$extra_nodes" ]]; then
                read -p "Delete extra minikube nodes? [y/N] " -n 1 -r
                echo
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    for node in $extra_nodes; do
                        minikube node delete "$node" 2>/dev/null || true
                    done
                fi
            fi
            ;;
    esac

    # Remove generated values file
    rm -f "${REPO_ROOT}/scripts/values-fargate-test-generated.yaml"

    log_success "Cleanup complete"
}

# Show status and next steps
show_status() {
    local pod_status=$(kubectl get pods -n "$NAMESPACE" -l app=fargate-test-workload -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "Unknown")
    local container_count=$(kubectl get pods -n "$NAMESPACE" -l app=fargate-test-workload -o jsonpath='{.items[0].spec.containers[*].name}' 2>/dev/null | wc -w | tr -d ' ')

    echo ""
    log_success "========================================="
    log_success "Fargate Sidecar Test Setup Complete!"
    log_success "========================================="
    echo ""
    echo "Configuration:"
    echo "  Cluster type:    $CLUSTER_TYPE"
    [[ -n "$LOCAL_CLUSTER" ]] && echo "  Local cluster:   $LOCAL_CLUSTER"
    echo "  Fargate node:    $TARGET_NODE"
    echo "  Test namespace:  $NAMESPACE"
    echo "  NR cluster name: $CLUSTER_NAME"
    echo "  Image:           $(get_image_name unprivileged)"
    [[ -n "$REGISTRY" ]] && echo "  Registry:        $REGISTRY"
    [[ -n "$SSH_HOST" ]] && echo "  SSH Host:        $SSH_HOST"
    echo ""
    echo "Status:"
    echo "  Pod status:      $pod_status"
    echo "  Containers:      $container_count (should be 2 if sidecar injected)"
    echo ""
    echo "Useful commands:"
    echo ""
    echo "  # Check test workload"
    echo "  kubectl get pods -n $NAMESPACE -o wide"
    echo ""
    echo "  # Check sidecar logs"
    echo "  kubectl logs -n $NAMESPACE -l app=fargate-test-workload -c newrelic-infrastructure-sidecar -f"
    echo ""
    echo "  # Check if sidecar is scraping"
    echo "  kubectl logs -n $NAMESPACE -l app=fargate-test-workload -c newrelic-infrastructure-sidecar | grep -E 'publishing|scrape'"
    echo ""
    echo "  # Query in New Relic"
    echo "  FROM K8sNodeSample SELECT * WHERE clusterName = '$CLUSTER_NAME' SINCE 5 minutes ago"
    echo ""
    echo "  # Cleanup"
    echo "  $0 --cleanup --cluster-type $CLUSTER_TYPE"
    echo ""

    # Warn if sidecar wasn't injected
    if [[ "$container_count" -lt 2 ]]; then
        log_warn "Sidecar may not have been injected. Check operator logs:"
        echo "  kubectl logs -n newrelic -l app.kubernetes.io/name=newrelic-infra-operator"
    fi
}

# Main
main() {
    log_info "Starting Fargate sidecar test setup..."

    detect_cluster_type
    detect_local_cluster

    if [[ "$CLEANUP" == "true" ]]; then
        cleanup
        exit 0
    fi

    build_image
    load_image
    setup_fargate_node
    setup_rbac
    install_helm

    # Wait for operator to be ready
    if [[ "$SKIP_HELM" != "true" ]] && [[ -n "$LICENSE_KEY" ]]; then
        log_info "Waiting for operator to be ready..."
        kubectl rollout status deployment -n newrelic -l app.kubernetes.io/name=newrelic-infra-operator --timeout=120s 2>/dev/null || true
        sleep 5
    fi

    deploy_workload
    show_status
}

main
