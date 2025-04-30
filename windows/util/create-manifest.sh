#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

DOCKER_IMAGE_NAME=${DOCKER_IMAGE_NAME:-""}
DOCKER_IMAGE_TAG=${DOCKER_IMAGE_TAG:-""}
IS_PRERELEASE=${IS_PRERELEASE:-"true"}

export GGCR_DISABGLE_CACHE=1

# Argument parsing
while [[ $# -gt 0 ]]; do
  case "$1" in
    --docker-image-name) DOCKER_IMAGE_NAME="$2"; shift 2 ;;
    --docker-image-tag) DOCKER_IMAGE_TAG="$2"; shift 2 ;;
    --is-prerelease) IS_PRERELEASE="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# Validation
for var in DOCKER_IMAGE_NAME DOCKER_IMAGE_TAG IS_PRERELEASE; do
  [[ -z "${!var}" ]] && echo "Error: $var is required." && exit 1
done

# Image tag configuration
IMAGE_TAG="${DOCKER_IMAGE_NAME}:${DOCKER_IMAGE_TAG}$([[ "$IS_PRERELEASE" != "false" ]] && echo "-pre")"
echo "IMAGE_TAG=${IMAGE_TAG}"

windows2019_image="${IMAGE_TAG}-windows-ltsc2019"
windows2022_image="${IMAGE_TAG}-windows-ltsc2022"
# windows2019_image="newrelic/nri-kubernetes-internal:0.1.0-pre-windows-ltsc2019"
# windows2022_image="newrelic/nri-kubernetes-internal:0.1.0-pre-windows-ltsc2022"

linux_manifest=$(regctl manifest get "${IMAGE_TAG}" --format '{{jsonPretty .}}')
echo "Extracted Linux manifest: %s" "$linux_manifest"

arm64_digest=$(jq -r '.manifests[] | select(.platform.architecture=="arm64" and .platform.os=="linux") | .digest' <<< "$linux_manifest")
armv7_digest=$(jq -r '.manifests[] | select(.platform.architecture=="arm" and .platform.variant=="v7" and .platform.os=="linux") | .digest' <<< "$linux_manifest")
amd64_digest=$(jq -r '.manifests[] | select(.platform.architecture=="amd64" and .platform.os=="linux") | .digest' <<< "$linux_manifest")

all_attestations=$(jq -r '.manifests[] | select(.platform.architecture=="unknown" and .platform.os=="unknown" and .annotations["vnd.docker.reference.type"]=="attestation-manifest")' <<< "$linux_manifest")
echo "All attestation objects: ${all_attestations}"

attestations_array=$(echo ${all_attestations} | jq -s '.')
echo "Attestation objects array: ${attestations_array}"

declare -A images=(
  [arm64]="${DOCKER_IMAGE_NAME}@${arm64_digest}"
  [armv7]="${DOCKER_IMAGE_NAME}@${armv7_digest}"
  [amd64]="${DOCKER_IMAGE_NAME}@${amd64_digest}"
  [win2019]="${windows2019_image}"
  [win2022]="${windows2022_image}"
)

for arch in "${!images[@]}"; do
  [[ -n "${images[$arch]}" ]] && echo "${arch^^} image: ${images[$arch]}"
done

echo "Creating new manifest..."

IMAGE_TAG="${IMAGE_TAG}-combined2"
manifest_cmd="docker manifest create ${IMAGE_TAG}"
for img in "${images[@]}"; do
  [[ -n "$img" ]] && manifest_cmd+=" --amend $img"
  printf "\nAdding image to manifest: $img\n"
done

echo "Executing: ${manifest_cmd}"
eval $manifest_cmd
echo "Manifest created successfully."
docker manifest inspect "${IMAGE_TAG}" > new_manifest.json

jq --argjson atts "${attestations_array}" '.manifests += $atts' new_manifest.json > updated_manifest.json

regctl manifest put ${IMAGE_TAG} < updated_manifest.json

