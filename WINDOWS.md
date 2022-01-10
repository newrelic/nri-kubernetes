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

### Host metrics

The following metrics will be missing from most samples as they are host metrics:
```
coreCount
processorCount
kernelVersion
```

### `K8sContainerSample`
```
containerID
containerImageID

containerMemoryMappedFileBytes

fsInodes
fsInodesFree
fsInodesUsed
fsUsedBytes*
fsUsedPercent*

memoryUsedBytes
memoryUtilization
```
`memoryWorkingSetBytes` and `memoryWorkingSetUtilization` can be used as alternatives to `memoryUsedBytes` and `memoryUtilization`.

### `K8sPodSample`:
```
net.errorsPerSecond
```

### `K8sNodeSample`

```
memoryRssBytes*
memoryPageFaults*
memoryMajorPageFaultsPerSecond*
allocatableHugepages1Gi
allocatableHugepages2Mi
capacityHugepages1Gi
capacityHugepages2Mi
condition.CorruptDockerOverlay2
condition.FrequentContainerdRestart
condition.FrequentDockerRestart
condition.FrequentKubeletRestart
condition.FrequentUnregisterNetDevice
condition.KernelDeadlock
condition.ReadonlyFilesystem
fsInodes
fsInodesFree
fsInodesUsed
linuxDistribution
net.errorsPerSecond
net.rxBytesPerSecond
net.txBytesPerSecond
runtimeInodes
runtimeInodesFree
runtimeInodesUsed
```

### `K8sVolumeSample`
```
fsInodes*
fsInodesFree*
fsInodesUsed*
```

> *: These metrics are reported, but always have the value `0`.

Metrics for other samples should be the same, as they are gathered from cluster-wide state broker `kube-state-metrics`.
