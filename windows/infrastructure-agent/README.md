# nri-kubernetes - Windows Agent Build Automation

This folder contains files and scripts used for automating the build process of the Kubernetes agent for Windows. These scripts are run via GitHub Actions and result in a Windows-compatible image for LTSC 2022.

The target Dockerhub repository is [newrelic/infrastrucutre-windows](https://hub.docker.com/r/newrelic/infrastructure-windows). This repository does not use a combined manifest, so the Windows image is additionally tagged with `ltsc2022`.

### Relevant GHA workflow files
- [build-win-infra-agent](https://github.com/newrelic/nri-kubernetes/blob/main/.github/workflows/build-win-infra-agent.yml)
- 