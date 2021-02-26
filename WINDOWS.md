# Windows

## Steps to build Windows image.

### Requirements:
* Windows host with build version equal o grater than the target os version of the image. (eg: To build a an image for 20H2 version the machine needs to be 20H2) 
* Docker for Windows installed

### Build base image (Infrastructure Agent)
* Clone the Infra Agent repo from github:
  ```
  git clone git@github.com:newrelic/infrastructure-agent.git
  ```
* Get the latest Agent binaries from [newrelic](http://download.newrelic.com/infrastructure_agent/binaries/windows/amd64/) and and place them in `infrastructure-agent/target/bin/windows_amd64/`
* Compile base docker image:
  ```
  ./build/win_build_container.ps1 -baseImageTag <WINDOWS VERSION TARGET> -agentVersion <AGENT VERSION>
  ```
### Build nri-kubernetes image
* Clone this repo and checkout latest version.
  ```
  git@github.com:newrelic/nri-kubernetes.git
  cd nri-kubernetes
  git checkout <LATEST RELEASE TAG>
  ```
* Build the container
  ```
  IMAGE_TAG=latest docker build -f Dockerfile.windows
  ```
  
## Missing metrics

This are the metrics that are not being exposed by the Windows kubelet.

### Node

/stats/summary

- memoryRssBytes: zero
- memoryPageFaults: zero
- memoryMajorPageFaultsPerSecond: zero
- net.rxBytesPerSecond
- net.txBytesPerSecond
- net.errorsPerSecond
- fsInodesFree
- fsInodes
- fsInodesUsed
- runtimeInodesFree
- runtimeInodes
- runtimeInodesUsed

### Pod

/stats/summary

- net.errorsPerSecond
- fsInodesFree
- fsInodes
- fsInodesUsed
- We still need to make a test for the pvc metrics

### Container

/stats/summary

- memoryUsedBytes
- fsUsedBytes: zero, so fsUsedPercent is zero
- fsInodesFree
- fsInodes
- fsInodesUsed

/metrics/cadvisor

- containerID
- containerImageID

### Pod

/stats/summary

- net.errorsPerSecond
