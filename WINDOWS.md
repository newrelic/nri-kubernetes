# Windows

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
