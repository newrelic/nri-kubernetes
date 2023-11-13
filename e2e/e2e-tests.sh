#!/usr/bin/env bash

# Test cluster
CLUSTER_NAME=""
K8S_VERSION=""

# Metric exceptions
EXCEPTIONS_SOURCE_FILE=""

# New Relic account (production) details
ACCOUNT_ID=""
API_KEY=""
LICENSE_KEY=""

# Unset if you only want to setup a test cluster with E2E specifications
# Set to true if you additionally want to run tests
RUN_TESTS=""

function main() {
    parse_args "$@"
    create_cluster
    if [[ "$RUN_TESTS" == "true" ]]; then
        run_tests
        teardown
    fi
}

function parse_args() {
    totalArgs=$#

    # Arguments are passed by value, so other functions
    # are not affected by destructive processing
    while [[ $# -gt 0 ]]; do
        case $1 in
            --account_id)
            shift
            ACCOUNT_ID="$1"
            ;;
            --api_key)
            shift
            API_KEY="$1"
            ;;
            --exceptions)
            shift
            EXCEPTIONS_SOURCE_FILE="$1"
            ;;
            --help)
            help
            exit 0
            ;;
            --k8s_version)
            shift
            K8S_VERSION="$1"
            ;;
            --license_key)
            shift
            LICENSE_KEY="$1"
            ;;
            --run_tests)
            RUN_TESTS="true"
            ;;
            -*|--*|*)
            echo "Unknown field: $1"
            exit 1
            ;;
        esac
        shift
    done

    if [[ totalArgs -lt 10 ]]; then
        help
    fi
}

function help() {
    cat <<END
 Usage:
 ${0##*/}    --k8s_version <cluster_version> --exceptions <exceptions_file>
             --account_id <new_relic_prod> --api_key <api_key>
             --license_key <license_key> [--run_tests]

 --k8s_version:  valid Kubernetes cluster version. It is highly recommended to use same versions as E2E tests
 --exceptions:   choose one '*-exceptions.yml' file
 --account_id:   New Relic account in production
 --api_key:      key type 'USER'
 --license_key:  key type 'INGEST - LICENSE'
 --run_tests:    if unset, create a cluster with specifications matching E2E tests
                 otherwise run tests in addition to setting up cluster
END
}

function create_cluster() {
    cd ..

    echo "🔄 Setup"
    minikube delete --all > /dev/null
    rm -rf bin/ > /dev/null
    now=$( date "+%Y-%m-%d-%H-%M-%S" )
    CLUSTER_NAME=${now}-e2e-tests

    echo "🔄 Creating cluster ${CLUSTER_NAME}"
    minikube start --container-runtime=containerd --kubernetes-version=v${K8S_VERSION} --profile ${CLUSTER_NAME} > /dev/null

    echo "🔄 Enabling metrics-server"
    minikube addons enable metrics-server --profile ${CLUSTER_NAME} > /dev/null

    echo "🔄 Building binary"
    make compile-multiarch > /dev/null

    echo "🔄 Building Docker image"
    export DOCKER_BUILDKIT=1
    docker build --tag e2e/nri-kubernetes:e2e  . --quiet > /dev/null

    echo "🔄 Loading image into cluster"
    minikube image load e2e/nri-kubernetes:e2e --profile ${CLUSTER_NAME} > /dev/null

    echo "🔄 Adding Helm repositories"
    helm repo add newrelic https://helm-charts.newrelic.com > /dev/null
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts > /dev/null
    helm repo update > /dev/null

    cd e2e/
}

function run_tests() {
    echo "🔄 Installing E2E action"
    go install github.com/newrelic/newrelic-integration-e2e-action@latest > /dev/null

    echo "🔄 Starting E2E tests"
    export EXCEPTIONS_SOURCE_FILE=${EXCEPTIONS_SOURCE_FILE}
    export ACCOUNT_ID=${ACCOUNT_ID}
    export API_KEY=${API_KEY}
    export LICENSE_KEY=${LICENSE_KEY}
    EXCEPTIONS_SOURCE_FILE=${EXCEPTIONS_SOURCE_FILE} LICENSE_KEY=${LICENSE_KEY} go run github.com/newrelic/newrelic-integration-e2e-action@latest \
        --commit_sha=test-string --retry_attempts=5 --retry_seconds=60 \
            --account_id=${ACCOUNT_ID} --api_key=${API_KEY} --license_key=${LICENSE_KEY} \
            --spec_path=test-specs.yml --verbose_mode=true --agent_enabled="false"
}

function teardown() {
    echo "🔄 Teardown"
    rm -rf ../bin/ > /dev/null
    minikube delete --all > /dev/null
}

main "$@"
