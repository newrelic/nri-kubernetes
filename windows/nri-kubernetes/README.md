# nri-kubernetes - Windows Integration Build Automation

This folder contains files and scripts used for automating the build process of the Kubernetes integration for Windows. These scripts are run via GitHub Actions and result in Windows-compatible images for LTSC 2019 and LTSC 2022. 

The target Dockerhub repository is [newrelic/nri-kubernetes](https://hub.docker.com/r/newrelic/nri-kubernetes). We use a combined manifest to support both Windows and Linux images having the same image tag.

### Relevant GHA workflow files
- [release-integration](https://github.com/newrelic/nri-kubernetes/blob/main/.github/workflows/release-integration.yml)
- [reusable-release-integration](https://github.com/newrelic/k8s-agents-automation/blob/main/.github/workflows/reusable-release-integration.yml)