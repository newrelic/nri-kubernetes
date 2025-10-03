#!/usr/bin/env bash

set -eu

# Change to repo root directory
cd "$(dirname "$0")/.." || exit 1

# Detect OS
OS="$(uname -s)"
case "${OS}" in
    Linux*)     PLATFORM=linux;;
    Darwin*)    PLATFORM=mac;;
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
            ;;
        *)
            echo "Command skipped."
            return 1
            ;;
    esac
}

# Initialize env file
echo "# OpenShift Environment Variables - $(date)" > "$ENV_FILE"
echo "# Platform: ${PLATFORM}" >> "$ENV_FILE"
echo "" >> "$ENV_FILE"

# Function 1: Add registry roles
add_registry_role() {
    echo "Adding registry roles to users..."

    for user in kubeadmin developer; do
        echo "  Adding roles for user: $user"
        oc policy add-role-to-user registry-viewer "$user"
        oc policy add-role-to-user registry-editor "$user"
    done

    echo "Registry roles added successfully."
}

# Function 2: Expose default registry
expose_default_registry() {
    echo "Exposing default registry..."

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
    echo "Building and pushing image..."

    # Get registry host (in case this function is run standalone)
    HOST=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}' 2>/dev/null || echo "")

    if [ -z "$HOST" ]; then
        echo "Error: Registry host not found. Run expose_default_registry first."
        exit 1
    fi

    # Prompt for scenario tag
    read -rp "Enter scenario tag: " SCENARIO_TAG
    TARGET_NAMESPACE="nr-${SCENARIO_TAG}"

    log_env_var "SCENARIO_TAG" "$SCENARIO_TAG"
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

# Interactive menu
show_menu() {
    echo ""
    echo "OpenShift Setup Script"
    echo "======================"
    echo "Platform detected: ${PLATFORM}"
    echo ""
    echo "1) Add registry roles"
    echo "2) Expose default registry"
    echo "3) Build and push image"
    echo "4) Run all functions"
    echo "5) Exit"
    echo ""
}

# Main execution
main() {
    while true; do
        show_menu
        read -rp "Select an option (1-5): " choice

        case $choice in
            1)
                add_registry_role
                ;;
            2)
                expose_default_registry
                ;;
            3)
                build_push_image
                ;;
            4)
                echo "Running all functions..."
                add_registry_role
                expose_default_registry
                build_push_image
                echo ""
                echo "All functions completed!"
                ;;
            5)
                echo "Exiting..."
                exit 0
                ;;
            *)
                echo "Invalid option. Please select 1-5."
                ;;
        esac

        echo ""
        read -rp "Press Enter to continue..."
    done
}

# Run main function
main
