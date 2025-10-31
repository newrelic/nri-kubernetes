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

# Function to run sudo commands with user confirmation, only needed for exposing default registry and handling certs
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

# Function 3: Build image
build_image() {
    echo ""
    echo "=== Building image ==="

    # Build
    echo "Compiling multiarch..."
    make compile-multiarch

    echo "Building Docker image..."
    docker build -t e2e-nri-kubernetes:e2e .

    echo "Image built successfully."
}

# Function 4: Push image
push_image_to_open_shift() {
    local target_namespace="$1"
    echo ""
    echo "=== Pushing image ==="

    # Get registry host (in case this function is run standalone)
    HOST=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}' 2>/dev/null || echo "")

    if [ -z "$HOST" ]; then
        echo "Error: Registry host not found. Run expose_default_registry first."
        return 1
    fi

    log_env_var "TARGET_NAMESPACE" "$target_namespace"
    log_env_var "IMAGE_NAME" "e2e-nri-kubernetes:e2e"
    log_env_var "FULL_IMAGE_PATH" "${HOST}/${target_namespace}/e2e-nri-kubernetes:e2e"

    # Create namespace if it doesn't exist
    echo "Creating namespace: ${target_namespace}"
    kubectl create namespace "${target_namespace}" --dry-run=client -o yaml | kubectl apply -f -
    echo "Tagging image for registry..."
    docker tag e2e-nri-kubernetes:e2e "${HOST}/${target_namespace}/e2e-nri-kubernetes:e2e"

    echo "Pushing image to registry..."
    docker push "${HOST}/${target_namespace}/e2e-nri-kubernetes:e2e"

    echo "Image pushed to ${target_namespace} successfully."
}

# Function 5: Create e2e values file
create_e2e_test_values_image_registry() {
    local namespace="$1"
    local image_repository="image-registry.openshift-image-registry.svc:5000/${namespace}/e2e-nri-kubernetes"
    cat > e2e/e2e-values-openshift.yml <<EOF
provider: OPEN_SHIFT
ksm:
  config:
    timeout: 60s
    retries: 3
    selector: "app.kubernetes.io/name=kube-state-metrics"
    scheme: "http"
    namespace: ${namespace}
images:
  integration:
    pullPolicy: Always
    tag: e2e
    repository: ${image_repository}
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
EOF

    echo "Created e2e-values-openshift.yml with repository: ${image_repository}"
}

# Function 6: Add Security Context Constraints
add_sccs() {
    local scenario_tag="$1"
    local namespace="$2"

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
        "nri-bundle-sa"
    )

    for sa in "${service_accounts[@]}"; do
        echo "  Adding privileged SCC to: $sa"
        oc adm policy add-scc-to-user privileged "system:serviceaccount:${namespace}:${sa}"
    done

    echo "SCCs added successfully."
}

# Function 7: Setup mTLS for etcd
setup_mtls_for_etcd() {
    local target_namespace="$1"
    local temp_file="etcd-secret.yaml"

    echo ""
    echo "=== Setting up mTLS for etcd for namespace ${target_namespace} ==="

    # Get the etcd-client secret
    echo "Fetching etcd-client secret from openshift-etcd namespace..."
    kubectl get secret etcd-client -n openshift-etcd -o yaml > "$temp_file"

    # Remove creationTimestamp, resourceVersion, and uid
    echo "Cleaning up secret metadata..."
    sed -i.bak '/creationTimestamp:/d; /resourceVersion:/d; /uid:/d' "$temp_file"

    # Change the name from etcd-client to my-etcd-secret
    echo "Updating secret name to my-etcd-secret..."
    sed -i.bak 's/name: etcd-client/name: my-etcd-secret/' "$temp_file"

    # Change the namespace
    echo "Updating namespace to ${target_namespace}..."
    sed -i.bak "s/namespace: openshift-etcd/namespace: ${target_namespace}/" "$temp_file"

    # Apply the secret
    echo "Applying secret to ${target_namespace}..."
    kubectl apply -n "${target_namespace}" -f "$temp_file"

    # Clean up backup files
    # rm -f "${temp_file}.bak"

    echo ""
    echo "mTLS secret configured successfully!"
    echo "Namespace: ${target_namespace}"
    echo "Secret name: my-etcd-secret"
}

# Function 9: Run E2E tests
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
        --spec_path=./e2e/test-specs-openshift.yml --verbose_mode=true --agent_enabled="false" --scenario_tag="$scenario_tag"

    echo "E2E tests completed."
}
create_e2e_test_values_online_registry() {
    local namespace="$1"

    # Online charts - exclude images section
    echo "Configuring for online charts (without image configuration)..."
    cat > e2e/e2e-values-openshift.yml <<EOF
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
  extraVolumeMounts:
    - name: etcd-tls-secret
      mountPath: /etc/etcd-secrets
      readOnly: true
  extraVolumes:
    - name: etcd-tls-secret
      secret:
        secretName: my-etcd-secret
EOF

    echo "Created e2e-values-openshift.yml for namespace: ${namespace}"
}

# Function: Run E2E tests (dev version)
dev_run_e2e_tests() {
    local release_name="$1"

    echo ""
    echo "=== Running E2E tests (Development) ==="

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
        --spec_path=./e2e/test-specs-openshift.yml --verbose_mode=true --agent_enabled="false" --scenario_tag="$release_name"

    echo "E2E tests completed."
}

# Function: Deploy nri-kubernetes (dev version)
dev_deploy_nri_kubernetes() {
    local namespace="$1"
    local release_name="$2"
    local values_file="$3"

    echo ""
    echo "=== Deploying nri-kubernetes (Development) ==="
    echo "Namespace: ${namespace}"
    echo "Release name: ${release_name}"

    # Check if LICENSE_KEY is set
    if [ -z "${LICENSE_KEY:-}" ]; then
        echo "Error: LICENSE_KEY environment variable is not set"
        echo "Please export LICENSE_KEY before deploying"
        return 1
    fi

    local cmd="helm upgrade --install \"${release_name}\" \
        --namespace \"${namespace}\" \
        --create-namespace \
        --set global.licenseKey=${LICENSE_KEY} \
        --set global.cluster=${release_name} \
        charts/newrelic-infrastructure"

    if [ -n "$values_file" ]; then
        echo "Using values file: ${values_file}"
        cmd="${cmd} -f ${values_file}"
    fi

    eval "$cmd"

    echo ""
    echo "nri-kubernetes deployed successfully!"
}

# Function: Deploy E2E resources (dev version)
dev_deploy_e2e_resources() {
    local namespace="$1"
    local release_name="$2"

    echo ""
    echo "=== Deploying E2E Resources (Development) ==="

    # Function to convert version string to comparable number
    ver() {
        printf $((10#$(printf "%03d%03d" $(echo "$1" | tr '.' ' '))))
    }

    # Get Kubernetes server version
    K8S_VERSION=$(kubectl version 2>&1 | grep 'Server Version' | awk -F' v' '{ print $2; }' | awk -F. '{ print $1"."$2; }')
    echo "Detected Kubernetes version: ${K8S_VERSION}"

    # Select appropriate kube-state-metrics version
    if [[ $(ver "$K8S_VERSION") -gt $(ver "1.29") ]]; then
        KSM_IMAGE_VERSION="v2.13.0"
    else
        KSM_IMAGE_VERSION="v2.10.0"
    fi

    echo "Will use KSM image version ${KSM_IMAGE_VERSION}"

    echo ""
    echo "Adding SCCs for OpenShift (before deployment)..."
    add_sccs "${release_name}" "${namespace}"

    echo ""
    # Deploy with Helm using default values.yaml from chart with OpenShift enabled
    helm upgrade --install "${release_name}-resources" \
        --namespace "${namespace}" \
        --create-namespace \
        ./charts/internal/e2e-resources \
        --set persistentVolume.enabled=true \
        --set persistentVolume.hostPath="/var/tmp/e2e-storage" \
        --set kube-state-metrics.image.tag="${KSM_IMAGE_VERSION}" \
        --set openShift.enabled=true

    echo "E2E resources deployed successfully."
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

# Workflow: Scenario workflow (build, push, configure, test)
run_scenario_workflow() {
    local scenario_tag="$1"
    local namespace="nr-$scenario_tag"

    echo ""
    echo "=== Running Scenario Workflow ==="

    # Option 4: Build image
    build_image

    # Option 5: Push image
    push_image_to_open_shift "$namespace"

    # Option 6: Setup mTLS for etcd
    setup_mtls_for_etcd "$namespace"

    # Option 8: Configure and run E2E tests
    create_e2e_test_values_image_registry "$namespace"
    add_sccs "$scenario_tag" "$namespace"
    run_e2e_tests "$scenario_tag" "$namespace"

    echo ""
    echo "=== Scenario workflow completed! ==="
}

# Workflow: Rerun scenario workflow (no build, just push, configure, test)
rerun_scenario_workflow() {
    local scenario_tag="$1"
    local namespace="nr-$scenario_tag"

    echo ""
    echo "=== Rerunning Scenario Workflow (No Build) ==="

    # Option 5: Push image
    push_image_to_open_shift "$namespace"

    # Option 6: Setup mTLS for etcd
    setup_mtls_for_etcd "$namespace"

    # Option 8: Configure and run E2E tests
    create_e2e_test_values_image_registry "$namespace"
    add_sccs "$scenario_tag" "$namespace"
    run_e2e_tests "$scenario_tag" "$namespace"

    echo ""
    echo "=== Scenario workflow completed! ==="
}

# this runs e2e tests with online images, not testing code changes
run_online_based_scenario_workflow() {
    local scenario_tag="$1"
    local namespace="nr-$scenario_tag"

    echo ""
    echo "=== Running Scenario Workflow (Online) ==="

    # Create namespace if it doesn't exist
    echo "Creating namespace: ${namespace}"
    if ! kubectl create namespace "${namespace}" --dry-run=client -o yaml | kubectl apply -f -; then
        echo "Error: Failed to create namespace ${namespace}"
        return 1
    fi

    # Verify namespace was created
    if ! kubectl get namespace "${namespace}" &>/dev/null; then
        echo "Error: Namespace ${namespace} was not created successfully"
        return 1
    fi

    echo "Namespace ${namespace} is ready"

    setup_mtls_for_etcd "$namespace"
    create_e2e_test_values_online_registry "$namespace"
    add_sccs "$scenario_tag" "$namespace"
    run_e2e_tests "$scenario_tag" "$namespace"

    echo ""
    echo "=== Scenario workflow (Online) completed! ==="
}

# Interactive menu
show_menu() {
    echo ""
    echo "=========================================================="
    echo "OpenShift E2E Test Runner"
    echo "=========================================================="
    echo "Platform: ${PLATFORM}"
    echo "Container Runtime: ${CONTAINER_RUNTIME}"
    echo ""
    echo "Quick Test (Online Images):"
    echo "  1) Run online-based scenario workflow (tests with online images only)"
    echo ""
    echo "Setup Functions (no scenario tag needed, only need to run once per openshift local cluster):"
    echo "  2) Add registry roles"
    echo "  3) Expose default registry"
    echo "  4) Run setup workflow (2-3)"
    echo ""
    echo "Scenario Functions (scenario tag required):"
    echo "  5) Build image"
    echo "  6) Push image"
    echo "  7) Setup mTLS for etcd"
    echo "  8) Configure and run E2E tests"
    echo "  9) Run scenario workflow (5+6+7+8)"
    echo " 10) Rerun scenario workflow (no build, 6+7+8)"
    echo ""
    echo "Development Functions (namespace/release_name required):"
    echo " 11) Setup mTLS for etcd (dev)"
    echo " 12) Create e2e-values file (dev)"
    echo " 13) Deploy E2E resources (dev)"
    echo " 14) Uninstall E2E resources (dev)"
    echo " 15) Build image (dev)"
    echo " 16) Push image (dev)"
    echo " 17) Deploy nri-kubernetes (dev)"
    echo " 18) Run E2E tests (dev)"
    echo ""
    echo "  0) Exit"
    echo ""
}

# Main execution
main() {
    # Store namespace and release_name for dev functions
    local dev_namespace=""
    local dev_release_name=""

    while true; do
        show_menu
        read -rp "Select an option (0-18): " choice

        case $choice in
            0)
                echo "Exiting..."
                exit 0
                ;;
            1)
                local scenario_tag=""
                read -rp "Enter scenario tag: " scenario_tag
                echo "Using scenario tag: ${scenario_tag}"
                run_online_based_scenario_workflow "$scenario_tag"
                ;;
            2)
                add_registry_role
                ;;
            3)
                expose_default_registry
                ;;
            4)
                run_setup
                ;;
            5)
                build_image
                ;;
            6)
                read -rp "Enter scenario tag: " scenario_tag
                push_image_to_open_shift "nr-$scenario_tag"
                ;;
            7)
                read -rp "Enter scenario tag: " scenario_tag
                setup_mtls_for_etcd "nr-$scenario_tag"
                ;;
            8)
                read -rp "Enter scenario tag: " scenario_tag
                create_e2e_test_values_image_registry "nr-$scenario_tag"
                add_sccs "$scenario_tag" "nr-$scenario_tag"
                run_e2e_tests "$scenario_tag"
                ;;
            9)
                read -rp "Enter scenario tag: " scenario_tag
                run_scenario_workflow "$scenario_tag"
                ;;
            10)
                read -rp "Enter scenario tag: " scenario_tag
                rerun_scenario_workflow "$scenario_tag"
                ;;
            11)
                # Setup namespace if not already set
                if [ -z "$dev_namespace" ]; then
                    read -rp "Enter namespace: " dev_namespace
                fi
                setup_mtls_for_etcd "$dev_namespace"
                ;;
            12)
                # Setup namespace and release name if not already set
                if [ -z "$dev_namespace" ]; then
                    read -rp "Enter namespace: " dev_namespace
                fi
                if [ -z "$dev_release_name" ]; then
                    read -rp "Enter release name: " dev_release_name
                fi
                create_e2e_test_values_image_registry "$dev_namespace"
                ;;
            12)
                # Setup namespace and release name if not already set
                if [ -z "$dev_namespace" ]; then
                    read -rp "Enter namespace: " dev_namespace
                fi
                if [ -z "$dev_release_name" ]; then
                    read -rp "Enter release name: " dev_release_name
                fi
                create_e2e_test_values_image_registry "$dev_namespace"
                ;;
            13)
                # Setup namespace and release name if not already set
                if [ -z "$dev_namespace" ]; then
                    read -rp "Enter namespace: " dev_namespace
                fi
                if [ -z "$dev_release_name" ]; then
                    read -rp "Enter release name: " dev_release_name
                fi
                dev_deploy_e2e_resources "$dev_namespace" "$dev_release_name"
                ;;
            14)
                # Uninstall E2E resources
                if [ -z "$dev_namespace" ]; then
                    read -rp "Enter namespace: " dev_namespace
                fi
                if [ -z "$dev_release_name" ]; then
                    read -rp "Enter release name: " dev_release_name
                fi
                echo ""
                echo "=== Uninstalling E2E resources ==="
                helm uninstall "${dev_release_name}-resources" -n "${dev_namespace}"
                echo "E2E resources uninstalled."

                sleep 5
                kubectl delete ns "${dev_namespace}"
                echo "Namespace ${dev_namespace} and its resources deleted."
                ;;
            15)
                # Build image (dev)
                build_image
                ;;
            16)
                # Push image (dev)
                if [ -z "$dev_namespace" ]; then
                    read -rp "Enter namespace: " dev_namespace
                fi

                push_image_to_open_shift "$dev_namespace"
                ;;
            17)
                # Setup namespace and release name if not already set
                if [ -z "$dev_namespace" ]; then
                    read -rp "Enter namespace: " dev_namespace
                fi
                if [ -z "$dev_release_name" ]; then
                    read -rp "Enter release name: " dev_release_name
                fi
                read -rp "Enter values file path relative to repo root (e.g., e2e/e2e-values-openshift.yml) or press Enter to skip: " values_file
                dev_deploy_nri_kubernetes "$dev_namespace" "$dev_release_name" "$values_file"
                ;;
            18)
                # Setup release name if not already set
                if [ -z "$dev_release_name" ]; then
                    read -rp "Enter release name: " dev_release_name
                fi
                dev_run_e2e_tests "$dev_release_name"
                ;;
            *)
                echo "Invalid option. Please select 0-18."
                ;;
        esac

        echo ""
        read -rp "Press Enter to continue..."
    done
}

# Run main function
main "$@"
