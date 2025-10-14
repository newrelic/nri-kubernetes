#!/usr/bin/env bash

set -eu

# Change to repo root directory
cd "$(dirname "$0")/.." || exit 1

# Detect OS
OS="$(uname -s)"
case "${OS}" in
    Linux*)     PLATFORM=linux; CONTAINER_RUNTIME=podman;;
    Darwin*)    PLATFORM=mac; CONTAINER_RUNTIME=docker;;
    *)          echo "Unsupported OS: ${OS}"; exit 1;;
esac

# Environment variables file
ENV_FILE="openshift/openshift-env-vars.txt"

# Function to log environment variables
log_env_var() {
    echo "$1=$2" >> "$ENV_FILE"
}

# Function to run sudo commands with user confirmation
run_sudo() {
    local cmd="$*"
    echo ""
    echo "The following command requires sudo privileges:"
    echo "  sudo $cmd"
    echo ""
    read -rp "Do you want to proceed? (y/n): " response

    case "$response" in
        [yY]|[yY][eE][sS])
            sudo $cmd
            return 0
            ;;
        *)
            echo "Command skipped."
            return 1
            ;;
    esac
}

# Initialize env file
initialize_env_file() {
    echo "# OpenShift Environment Variables - $(date)" > "$ENV_FILE"
    echo "# Platform: ${PLATFORM}" >> "$ENV_FILE"
    echo "" >> "$ENV_FILE"
}

# Function 1: Add registry roles
add_registry_role() {
    echo ""
    echo "=== Adding registry roles to users ==="

    for user in kubeadmin developer; do
        echo "  Adding roles for user: $user"
        oc policy add-role-to-user registry-viewer "$user"
        oc policy add-role-to-user registry-editor "$user"
    done

    echo "Registry roles added successfully."
}

# Function 2: Expose default registry
expose_default_registry() {
    echo ""
    echo "=== Exposing default registry ==="

    # Enable default route
    oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge

    # Get registry host
    HOST=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
    echo "Registry host: $HOST"
    log_env_var "REGISTRY_HOST" "$HOST"

    # Extract certificate
    CERT_NAME=$(oc get ingresscontroller -n openshift-ingress-operator default -o json | jq '.spec.defaultCertificate.name // "router-certs-default"' -r)
    oc extract "secret/$CERT_NAME" -n openshift-ingress --confirm

    # Handle certificate trust based on platform
    if [ "$PLATFORM" = "linux" ]; then
        echo "Setting up certificate trust (Linux)..."
        run_sudo mv tls.crt /etc/pki/ca-trust/source/anchors/
        run_sudo update-ca-trust enable

        # Login with podman
        echo "Logging into registry with podman..."
        TOKEN="$(oc whoami -t)"
        run_sudo podman login -u kubeadmin -p "$TOKEN" "$HOST"

    elif [ "$PLATFORM" = "mac" ]; then
        echo "Setting up certificate trust (macOS)..."
        run_sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain tls.crt
        rm tls.crt

        # Login with docker
        echo "Logging into registry with docker..."
        docker login -u kubeadmin -p "$(oc whoami -t)" "$HOST"
    fi

    echo "Registry exposed and configured successfully."
}

# Function 3: Build and push image
build_push_image() {
    local scenario_tag="$1"

    echo ""
    echo "=== Building and pushing image ==="

    # Get registry host (in case this function is run standalone)
    HOST=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}' 2>/dev/null || echo "")

    if [ -z "$HOST" ]; then
        echo "Error: Registry host not found. Run expose_default_registry first."
        return 1
    fi

    TARGET_NAMESPACE="nr-${scenario_tag}"

    log_env_var "SCENARIO_TAG" "$scenario_tag"
    log_env_var "TARGET_NAMESPACE" "$TARGET_NAMESPACE"
    log_env_var "IMAGE_NAME" "e2e-nri-kubernetes:e2e"
    log_env_var "FULL_IMAGE_PATH" "${HOST}/${TARGET_NAMESPACE}/e2e-nri-kubernetes:e2e"

    # Create namespace if it doesn't exist
    echo "Creating namespace: ${TARGET_NAMESPACE}"
    kubectl create namespace "${TARGET_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

    # Build
    echo "Compiling multiarch..."
    make compile-multiarch

    echo "Building Docker image..."
    docker build -t e2e-nri-kubernetes:e2e .

    echo "Tagging image for registry..."
    docker tag e2e-nri-kubernetes:e2e "${HOST}/${TARGET_NAMESPACE}/e2e-nri-kubernetes:e2e"

    echo "Pushing image to registry..."
    docker push "${HOST}/${TARGET_NAMESPACE}/e2e-nri-kubernetes:e2e"

    echo "Image built and pushed to ${TARGET_NAMESPACE} successfully."
}

# Function 4: Create e2e values file
create_e2e_values() {
    local scenario_tag="$1"
    local image_repository="image-registry.openshift-image-registry.svc:5000/nr-${scenario_tag}/e2e-nri-kubernetes"

    cat > e2e/e2e-values-dynamic.yml <<EOF
images:
  integration:
    pullPolicy: Always
    tag: e2e
    repository: ${image_repository}
EOF

    echo "Created e2e-values-dynamic.yml with repository: ${image_repository}"
}

# Function 5: Add Security Context Constraints
add_sccs() {
    local scenario_tag="$1"

    echo ""
    echo "=== Adding Security Context Constraints ==="

    local service_accounts=(
        "${scenario_tag}-newrelic-infrastructure"
        "${scenario_tag}-newrelic-infrastructure-controlplane"
        "${scenario_tag}-kube-state-metrics"
        "${scenario_tag}-newrelic-logging"
        "${scenario_tag}-nri-kube-events"
        "${scenario_tag}-nri-metadata-injection-admission"
        "default"
        "${scenario_tag}-nrk8s-controlplane"
        "newrelic-bundle-newrelic-logging"
    )

    for sa in "${service_accounts[@]}"; do
        echo "  Adding privileged SCC to: $sa"
        oc adm policy add-scc-to-user privileged "system:serviceaccount:nr-${scenario_tag}:${sa}"
    done

    echo "SCCs added successfully."
}

# Function 6: Deploy E2E resources
deploy_e2e_resources() {
    local scenario_tag="$1"
    local values_file="${2:-$HOME/ConfigFiles/e2e-resources/9-26-oshift-values.yaml}"

    echo ""
    echo "=== Deploying E2E Resources ==="

    # Function to convert version string to comparable number
    ver() {
        printf $((10#$(printf "%03d%03d" $(echo "$1" | tr '.' ' '))))
    }

    # Get Kubernetes server version
    K8S_VERSION=$(kubectl version 2>&1 | grep 'Server Version' | awk -F' v' '{ print $2; }' | awk -F. '{ print $1"."$2; }')
    echo "Detected Kubernetes version: ${K8S_VERSION}"

    # Select appropriate kube-state-metrics version
    if [[ $(ver "$K8S_VERSION") -gt $(ver "1.22") ]]; then
        KSM_IMAGE_VERSION="v2.15.0"
    else
        KSM_IMAGE_VERSION="v2.10.0"
    fi

    echo "Will use KSM image version ${KSM_IMAGE_VERSION}"

    # Deploy with Helm
    helm upgrade --install "${scenario_tag}-resources" \
        --namespace "nr-${scenario_tag}" \
        --create-namespace \
        ../charts/internal/e2e-resources \
        --set persistentVolume.enabled=true \
        --set kube-state-metrics.image.tag="${KSM_IMAGE_VERSION}" \
        -f "${values_file}"

    echo "E2E resources deployed successfully."
}

# Function 7: Run E2E tests
run_e2e_tests() {
    local scenario_tag="$1"

    echo ""
    echo "=== Running E2E tests ==="

    # Check required environment variables
    if [ -z "${LICENSE_KEY:-}" ] || [ -z "${ACCOUNT_ID:-}" ] || [ -z "${API_KEY:-}" ]; then
        echo "Error: Missing required environment variables"
        echo ""
        echo "Please set the following environment variables:"
        echo "  export LICENSE_KEY=your-license-key"
        echo "  export ACCOUNT_ID=your-account-id"
        echo "  export API_KEY=your-api-key"
        echo "  export EXCEPTIONS_SOURCE_FILE=e2e/1_34-exceptions.yml  # optional"
        return 1
    fi

    # Run the e2e test
    LICENSE_KEY=${LICENSE_KEY} EXCEPTIONS_SOURCE_FILE=${EXCEPTIONS_SOURCE_FILE:-} go run github.com/newrelic/newrelic-integration-e2e-action@latest \
        --commit_sha=test-string --retry_attempts=5 --retry_seconds=60 \
        --account_id=${ACCOUNT_ID} --api_key=${API_KEY} --license_key=${LICENSE_KEY} \
        --spec_path=./e2e/test-specs.yml --verbose_mode=true --agent_enabled="false" --scenario_tag="$scenario_tag"

    echo "E2E tests completed."
}

# Workflow: Setup only (no scenario tag needed)
run_setup() {
    echo ""
    echo "=== Running Setup Workflow ==="
    initialize_env_file
    add_registry_role
    expose_default_registry
    echo ""
    echo "=== Setup workflow completed! ==="
}

# Workflow: Scenario workflow (build, configure, test)
run_scenario_workflow() {
    local scenario_tag="$1"

    echo ""
    echo "=== Running Scenario Workflow ==="

    build_push_image "$scenario_tag"
    create_e2e_values "$scenario_tag"
    add_sccs "$scenario_tag"
    run_e2e_tests "$scenario_tag"

    echo ""
    echo "=== Scenario workflow completed! ==="
}

# Workflow: Run all
run_all() {
    echo ""
    echo "=== Running All Functions ==="

    initialize_env_file
    add_registry_role
    expose_default_registry

    read -rp "Enter scenario tag for build/test workflow: " scenario_tag

    run_scenario_workflow "$scenario_tag"

    echo ""
    echo "=== All functions completed! ==="
}

# Interactive menu
show_menu() {
    echo ""
    echo "=================================================="
    echo "OpenShift E2E Test Runner"
    echo "=================================================="
    echo "Platform: ${PLATFORM}"
    echo "Container Runtime: ${CONTAINER_RUNTIME}"
    echo ""
    echo "Setup Functions (no scenario tag needed):"
    echo "  1) Add registry roles"
    echo "  2) Expose default registry"
    echo "  3) Run setup workflow (1-2)"
    echo ""
    echo "Scenario Functions (scenario tag required):"
    echo "  4) Build and push image"
    echo "  5) Deploy E2E resources"
    echo "  6) Configure and run E2E tests"
    echo "  7) Run scenario workflow (4+6)"
    echo ""
    echo "Combined:"
    echo "  8) Run all functions"
    echo ""
    echo "  9) Exit"
    echo ""
}

# Main execution
main() {
    # Non-interactive mode with command-line arguments
    if [ $# -gt 0 ]; then
        local command="$1"
        local scenario_tag="${2:-}"

        case "$command" in
            all)
                run_all
                ;;
            setup)
                run_setup
                ;;
            scenario)
                if [ -z "$scenario_tag" ]; then
                    read -rp "Enter scenario tag: " scenario_tag
                fi
                run_scenario_workflow "$scenario_tag"
                ;;
            resources)
                if [ -z "$scenario_tag" ]; then
                    read -rp "Enter scenario tag: " scenario_tag
                fi
                local values_file="${3:-$HOME/ConfigFiles/e2e-resources/9-26-oshift-values.yaml}"
                deploy_e2e_resources "$scenario_tag" "$values_file"
                ;;
            *)
                echo "Unknown command: $command"
                echo "Usage: $0 [all|setup|scenario|resources] [scenario_tag] [values_file]"
                exit 1
                ;;
        esac
        exit 0
    fi

    # Interactive mode
    while true; do
        show_menu
        read -rp "Select an option (1-9): " choice

        case $choice in
            1)
                add_registry_role
                ;;
            2)
                expose_default_registry
                ;;
            3)
                run_setup
                ;;
            4)
                read -rp "Enter scenario tag: " scenario_tag
                build_push_image "$scenario_tag"
                ;;
            5)
                read -rp "Enter scenario tag: " scenario_tag
                read -rp "Enter values file path (default: ~/ConfigFiles/e2e-resources/9-26-oshift-values.yaml): " values_file
                deploy_e2e_resources "$scenario_tag" "${values_file}"
                ;;
            6)
                read -rp "Enter scenario tag: " scenario_tag
                create_e2e_values "$scenario_tag"
                add_sccs "$scenario_tag"
                run_e2e_tests "$scenario_tag"
                ;;
            7)
                read -rp "Enter scenario tag: " scenario_tag
                run_scenario_workflow "$scenario_tag"
                ;;
            8)
                run_all
                ;;
            9)
                echo "Exiting..."
                exit 0
                ;;
            *)
                echo "Invalid option. Please select 1-9."
                ;;
        esac

        echo ""
        read -rp "Press Enter to continue..."
    done
}

# Run main function
main "$@"
