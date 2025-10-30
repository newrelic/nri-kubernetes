# e2e-resources

![Version: 1.14.0-devel](https://img.shields.io/badge/Version-1.14.0--devel-informational?style=flat-square)

This chart creates e2e resources for nri-kubernetes.

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| dbudziwojskiNR |  | <https://github.com/dbudziwojskiNR> |
| tmnguyen12 |  | <https://github.com/tmnguyen12> |
| kondracek-nr |  | <https://github.com/kondracek-nr> |

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| https://helm-charts.newrelic.com | common-library | 1.3.3 |
| https://prometheus-community.github.io/helm-charts | kube-state-metrics | 5.30.1 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| cronjob.enabled | bool | `true` | Deploy a dummy cronjob |
| daemonSet.enabled | bool | `true` | Deploy a dummy daemonSet |
| demo.enabled | bool | `false` | Deploy in demo mode. Make entities consume non-negligible resources so metrics can be easily observed in the dashboards. This setting only applies to resources that would negatively impact testing times if enabled by default |
| deployment.enabled | bool | `true` | Deploy a dummy deployment |
| failingJob.enabled | bool | `true` | Deploy a failing job |
| fileSystemTest | object | `{"fileName":"pi.txt"}` | Variables for filesystem testing |
| hpa.enabled | bool | `true` | Enable hpa resources |
| kube-state-metrics.metricAnnotationsAllowList[0] | string | `"resourcequotas=[owner,description]"` |  |
| kube-state-metrics.metricAnnotationsAllowList[1] | string | `"namespaces=[owner,description]"` |  |
| kube-state-metrics.metricAnnotationsAllowList[2] | string | `"deployments=[owner,description]"` |  |
| kube-state-metrics.metricAnnotationsAllowList[3] | string | `"pods=[owner,description]"` |  |
| kube-state-metrics.metricLabelsAllowlist[0] | string | `"resourcequotas=[environment,team]"` |  |
| kube-state-metrics.metricLabelsAllowlist[1] | string | `"namespaces=[environment,team]"` |  |
| kube-state-metrics.metricLabelsAllowlist[2] | string | `"deployments=[environment,team]"` |  |
| kube-state-metrics.metricLabelsAllowlist[3] | string | `"pods=[environment,team]"` |  |
| loadBalancerService.annotations | object | `{}` |  |
| loadBalancerService.enabled | bool | `true` | Deploy a loadBalancer service |
| loadBalancerService.fakeIP | string | `""` | If set, will deploy service with a loadBalancerIP set to this value |
| openShift.enabled | bool | `false` |  |
| pending.enabled | bool | `true` | Enable crashing and pending pods |
| persistentVolume.enabled | bool | `true` | Create PVs |
| persistentVolume.multiNode | bool | `false` | Changes PV type to run on multi-node clusters (e.g. GKE, OpenShift on GCP) |
| persistentVolumeClaim.enabled | bool | `true` | Create PVCs |
| scraper.enabled | bool | `false` | Deploy the scraper pod |
| statefulSet.enabled | bool | `true` | Deploy a dummy statefulSet |
| windows.is2019 | bool | `false` | Deploy resources on Windows Server 2019 nodes |
| windows.is2022 | bool | `false` | Deploy resources on Windows Server 2022 nodes |

