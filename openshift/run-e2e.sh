#!/usr/bin/env bash

# Change to repo root directory
cd "$(dirname "$0")/.." || exit 1

# Parse command line arguments
SCENARIO_TAG=""

for arg in "$@"; do
  case $arg in
    --scenario_tag=*)
      SCENARIO_TAG="${arg#*=}"
      shift
      ;;
  esac
done

# If no tag provided via flag, prompt for it
if [ -z "$SCENARIO_TAG" ]; then
  read -p "Enter scenario tag: " SCENARIO_TAG
fi

# Set image repository for the scenario namespace
export IMAGE_REPOSITORY="image-registry.openshift-image-registry.svc:5000/nr-${SCENARIO_TAG}/e2e-nri-kubernetes"

# Create dynamic e2e values file with correct repository
cat > e2e/e2e-values-dynamic.yml <<EOF
images:
  integration:
    pullPolicy: Always
    tag: e2e
    repository: ${IMAGE_REPOSITORY}
EOF

echo "Created e2e-values-dynamic.yml with repository: ${IMAGE_REPOSITORY}"

oc adm policy add-scc-to-user privileged system:serviceaccount:nr-"$SCENARIO_TAG":"$SCENARIO_TAG"-newrelic-infrastructure
oc adm policy add-scc-to-user privileged system:serviceaccount:nr-"$SCENARIO_TAG":"$SCENARIO_TAG"-newrelic-infrastructure-controlplane 
oc adm policy add-scc-to-user privileged system:serviceaccount:nr-"$SCENARIO_TAG":"$SCENARIO_TAG"-kube-state-metrics 
oc adm policy add-scc-to-user privileged system:serviceaccount:nr-"$SCENARIO_TAG":"$SCENARIO_TAG"-newrelic-logging 
oc adm policy add-scc-to-user privileged system:serviceaccount:nr-"$SCENARIO_TAG":"$SCENARIO_TAG"-nri-kube-events 
oc adm policy add-scc-to-user privileged system:serviceaccount:nr-"$SCENARIO_TAG":"$SCENARIO_TAG"-nri-metadata-injection-admission 
oc adm policy add-scc-to-user privileged system:serviceaccount:nr-"$SCENARIO_TAG":default 
oc adm policy add-scc-to-user privileged system:serviceaccount:nr-"$SCENARIO_TAG":"$SCENARIO_TAG"-nrk8s-controlplane
oc adm policy add-scc-to-user privileged system:serviceaccount:nr-"$SCENARIO_TAG":newrelic-bundle-newrelic-logging

# Run the e2e test with the provided tag
LICENSE_KEY=${LICENSE_KEY} EXCEPTIONS_SOURCE_FILE=${EXCEPTIONS_SOURCE_FILE} go run github.com/newrelic/newrelic-integration-e2e-action@latest \
  --commit_sha=test-string --retry_attempts=5 --retry_seconds=60 \
  --account_id=${ACCOUNT_ID} --api_key=${API_KEY} --license_key=${LICENSE_KEY} \
  --spec_path=./e2e/test-specs.yml --verbose_mode=true --agent_enabled="false" --scenario_tag="$SCENARIO_TAG"
