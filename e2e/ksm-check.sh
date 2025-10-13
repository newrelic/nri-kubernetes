#!/bin/bash

# Function to extract Kubernetes server version
ver() {
    kubectl version 2>&1 | grep 'Server Version' |
    awk -F' v' '{ print $2; }' |
    awk -F. '{ print $1"."$2; }'
}

# Get the Kubernetes server version
K8S_VERSION=$(ver)

# Initialize ksm-version variable
KSM_VERSION=""

# Use a case statement to determine the correct ksm-version
case $K8S_VERSION in
    1.2[1-9])
        KSM_VERSION="v2.10.0"
        ;;
    "1.30")
        KSM_VERSION="v2.13.0"
        ;;
    "1.31")
        KSM_VERSION="v2.14.0"
        ;;
    "1.32")
        KSM_VERSION="v2.16.0"
        ;;
    "1.33"|1.3[4-9]|1.[4-9][0-9]|[2-9].[0-9]*)
        KSM_VERSION="v2.17.0"
        ;;
    *)
        echo "Unsupported Kubernetes version: $K8S_VERSION"
        exit 1
        ;;
esac

# Output the determined ksm version
echo "$KSM_VERSION"