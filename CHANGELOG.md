# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## Unreleased

### enhancement
- Update `e2e-resources` chart by @juanjjaramillo [#1018](https://github.com/newrelic/nri-kubernetes/pull/1018)

## v3.27.1 - 2024-03-18

### ⛓️ Dependencies
- Updated github.com/prometheus/common to v0.50.0 - [Changelog 🔗](https://github.com/prometheus/common/releases/tag/v0.50.0)

## v3.27.0 - 2024-03-11

### 🚀 Enhancements
- Automatically release E2E resources chart by @juanjjaramillo [#1013](https://github.com/newrelic/nri-kubernetes/pull/1013)

### 🐞 Bug fixes
- Give GitHub token permission to release chart by @juanjjaramillo [#1014](https://github.com/newrelic/nri-kubernetes/pull/1014)

### ⛓️ Dependencies
- Updated golang.org/x/crypto to v0.21.0
- Updated google.golang.org/protobuf to v1.33.0

## v3.26.1 - 2024-03-04

### ⛓️ Dependencies
- Updated kubernetes packages to v0.29.2
- Updated github.com/prometheus/client_model to v0.6.0 - [Changelog 🔗](https://github.com/prometheus/client_model/releases/tag/v0.6.0)
- Updated github.com/prometheus/common to v0.48.0 - [Changelog 🔗](https://github.com/prometheus/common/releases/tag/v0.48.0)

## v3.26.0 - 2024-02-26

### 🚀 Enhancements
- Add linux node selector @dbudziwojskiNR [#1000](https://github.com/newrelic/nri-kubernetes/pull/1000)

## v3.25.2 - 2024-02-19

### ⛓️ Dependencies
- Updated golang.org/x/crypto to v0.19.0
- Updated github.com/newrelic/infra-integrations-sdk to v3.8.2+incompatible

## v3.25.1 - 2024-02-12

### ⛓️ Dependencies
- Updated github.com/newrelic/infra-integrations-sdk to v3.8.0+incompatible

## v3.25.0 - 2024-02-05

### 🚀 Enhancements
- Add Codecov @dbudziwojskiNR [#980](https://github.com/newrelic/nri-kubernetes/pull/980)

## v3.24.2 - 2024-01-29

### 🐞 Bug fixes
- Update clusterrole.yaml by @akshaychopra5207 [#933](https://github.com/newrelic/nri-kubernetes/pull/933) and [#978](https://github.com/newrelic/nri-kubernetes/pull/978)

### ⛓️ Dependencies
- Updated kubernetes packages to v0.29.1
- Updated alpine to v3.19.1

## v3.24.1 - 2024-01-22

### ⛓️ Dependencies
- Updated github.com/prometheus/common to v0.46.0 - [Changelog 🔗](https://github.com/prometheus/common/releases/tag/v0.46.0)
- Updated go to v1.21.6

## v3.24.0 - 2024-01-08

### 🚀 Enhancements
- Add pod startup metrics by @w21froster [#964](https://github.com/newrelic/nri-kubernetes/pull/964)

## v3.23.3 - 2024-01-08

### ⛓️ Dependencies
- Updated golang.org/x/crypto to v0.18.0

## v3.23.2 - 2024-01-08

### ⛓️ Dependencies
- Updated kubernetes packages to v0.29.0


## v3.23.1 - 2023-12-25

### ⛓️ Dependencies
- Updated github.com/spf13/viper to v1.18.2 - [Changelog 🔗](https://github.com/spf13/viper/releases/tag/v1.18.2)
- Updated google.golang.org/protobuf to v1.32.0

## v3.23.0 - 2023-12-09

### 🚀 Enhancements
- Trigger release creation by @juanjjaramillo [#958](https://github.com/newrelic/nri-kubernetes/pull/958)

### ⛓️ Dependencies
- Updated alpine to v3.19.0
- Updated github.com/spf13/viper to v1.18.1 - [Changelog 🔗](https://github.com/spf13/viper/releases/tag/v1.18.1)
- Updated golang.org/x/crypto to v0.16.0
- Updated go to v1.21.5

## v3.22.0 - 2023-12-06

### 🚀 Enhancements
- Update reusable workflow dependency by @juanjjaramillo [#951](https://github.com/newrelic/nri-kubernetes/pull/951)

## v3.21.0 - 2023-11-20

### 🚀 Enhancements
- Improve E2E resources chart by @juanjjaramillo in [#946](https://github.com/newrelic/nri-kubernetes/pull/946)
- Update k8s.yaml by @juanjjaramillo in [#947](https://github.com/newrelic/nri-kubernetes/pull/947)
- Automate local E2E test runs by @juanjjaramillo in [#938](https://github.com/newrelic/nri-kubernetes/pull/938)
- Add PV, PVC dashboards tests by @dbudziwojskiNR in [#829](https://github.com/newrelic/nri-kubernetes/pull/829)
- Add statefulset dashboard tests by @dbudziwojskiNR in [#830](https://github.com/newrelic/nri-kubernetes/pull/830)
- Add deployment dashboard tests by @dbudziwojskiNR in [#832](https://github.com/newrelic/nri-kubernetes/pull/832)
- Add failed job dashboard tests by @dbudziwojskiNR in [#855](https://github.com/newrelic/nri-kubernetes/pull/855)

### ⛓️ Dependencies
- Updated kubernetes packages to v0.28.4

## v3.20.0 - 2023-11-13

### 🚀 Enhancements
- Update E2E resources by @juanjjaramillo in [#926](https://github.com/newrelic/nri-kubernetes/pull/926)
- Replace k8s v1.28.0-rc.1 with k8s 1.28.3 by @svetlanabrennan in [#936](https://github.com/newrelic/nri-kubernetes/pull/936)
- Add failed pod container pending e2e tests by @dbudziwojskiNR in [#849](https://github.com/newrelic/nri-kubernetes/pull/849)
- Add failed pod container creating e2e tests by @dbudziwojskiNR in [#848](https://github.com/newrelic/nri-kubernetes/pull/848)
- Add cronjob dashboard tests by @dbudziwojskiNR in [#827](https://github.com/newrelic/nri-kubernetes/pull/827)
- Add daemonset dashboard tests by @dbudziwojskiNR in [#828](https://github.com/newrelic/nri-kubernetes/pull/828)

### ⛓️ Dependencies
- Updated golang.org/x/crypto to v0.15.0

## v3.19.0 - 2023-11-06

### 🚀 Enhancements
- Add k8s v1.28.0-rc.1 support by @svetlanabrennan in [#919](https://github.com/newrelic/nri-kubernetes/pull/919)

## v3.18.4 - 2023-10-30

### 🐞 Bug fixes
- Fix `renovate` configuration by juanjjaramillo in [PR #921](https://github.com/newrelic/nri-kubernetes/pull/921)

## v3.18.3 - 2023-10-23

### ⛓️ Dependencies
- Updated kubernetes packages to v0.28.3
- Updated github.com/prometheus/common to v0.45.0 - [Changelog 🔗](https://github.com/prometheus/common/releases/tag/v0.45.0)

## v3.18.2 - 2023-10-16

### 🐞 Bug fixes
- Address CVE-2023-44487 and CVE-2023-39325 by juanjjaramillo in [PR #910](https://github.com/newrelic/nri-kubernetes/pull/910)

## v3.18.1 - 2023-10-12

### ⛓️ Dependencies
- Updated github.com/google/go-cmp to v0.6.0 - [Changelog 🔗](https://github.com/google/go-cmp/releases/tag/v0.6.0)
- Updated github.com/spf13/viper to v1.17.0 - [Changelog 🔗](https://github.com/spf13/viper/releases/tag/v1.17.0)

## v3.18.0 - 2023-10-06

### 🚀 Enhancements
- Enable automatic release [#900](https://github.com/newrelic/nri-kubernetes/pull/900)
- Bump appVersion to 3.17.0 and chart to 3.22.0 [#886](https://github.com/newrelic/nri-kubernetes/pull/886)

### ⛓️ Dependencies
- Upgraded alpine from 3.18.3 to 3.18.4
- Updated go to 1.21
- Updated golang.org/x/crypto to v0.14.0
- Updated github.com/prometheus/client_model to v0.5.0 - [Changelog 🔗](https://github.com/prometheus/client_model/releases/tag/v0.5.0)

## 3.17.0
### dependencies
* chore(deps): bump docker/setup-qemu-action from 2 to 3 by @dependabot in https://github.com/newrelic/nri-kubernetes/pull/881
* chore(deps): bump docker/login-action from 2 to 3 by @dependabot in https://github.com/newrelic/nri-kubernetes/pull/880
* chore(deps): bump manusa/actions-setup-minikube from 2.7.2 to 2.9.0 by @dependabot in https://github.com/newrelic/nri-kubernetes/pull/879
* chore(deps): bump docker/setup-buildx-action from 2 to 3 by @dependabot in https://github.com/newrelic/nri-kubernetes/pull/878
* chore(deps): update actions/checkout action to v4 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/864
* fix(deps): update kubernetes packages to v0.28.2 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/861
* chore(deps): update newrelic/infrastructure-bundle docker tag to v3.2.16 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/859
* chore(deps): update newrelic/k8s-events-forwarder docker tag to v1.47.0 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/860
* chore(deps): update module golang.org/x/crypto to v0.13.0 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/866
* chore(deps): update aquasecurity/trivy-action action to v0.12.0 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/862

### enhancement
- Update KSM version and chart, deprecate incompatible Kubernetes versions by @juanjjaramillo in [#867](https://github.com/newrelic/nri-kubernetes/pull/867)
- Update unit testing data by @juanjjaramillo in [#868](https://github.com/newrelic/nri-kubernetes/pull/868)
- Update Helm chart lint job by @juanjjaramillo in [#869](https://github.com/newrelic/nri-kubernetes/pull/869)
- Update cpuLimitCores metric not available log to debug level in [#870](https://github.com/newrelic/nri-kubernetes/pull/870)
- Updated KSM unit tests by @svetlanabrennan in [#876](https://github.com/newrelic/nri-kubernetes/pull/876)

### bugfix
- Use a KSM stable metric instead of an experimental one by @juanjjaramillo in [#872](https://github.com/newrelic/nri-kubernetes/pull/872)

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.16.0...v3.17.0

## 3.16.0
## What's Changed
- Add changelog workflow by @svetlanabrennan in [#837](https://github.com/newrelic/nri-kubernetes/pull/837)
- Update changelog workflow @svetlanabrennan in [#843](https://github.com/newrelic/nri-kubernetes/pull/843)
- Add k8s 1.27 support by @csongnr in [#845](https://github.com/newrelic/nri-kubernetes/pull/845)

## New Contributors
* @davidgit made their first contribution in https://github.com/newrelic/nri-kubernetes/pull/826
* @nr-security-github made their first contribution in https://github.com/newrelic/nri-kubernetes/pull/846

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.15.3...v3.15.4

## 3.15.3
## What's Changed
* bump app and chart version by @csongnr in https://github.com/newrelic/nri-kubernetes/pull/819
* NR-139168: Fix k8s.container.cpuCoresUtilization metric calculation by @sachin-shankar in https://github.com/newrelic/nri-kubernetes/pull/817

## New Contributors
* @sachin-shankar made their first contribution in https://github.com/newrelic/nri-kubernetes/pull/817

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.15.2...v3.15.3

## 3.15.2
## What's Changed
* Bump Chart Versions by @xqi-nr in #810
* Update Changelog by @xqi-nr in #809
* chore(deps): Directly Use Prometheus Parser - remove prom2json dep by @isaacadeleke-nr in #799
* fix(parsing): Log an error instead of fully failing on partial parsing failure by @isaacadeleke-nr in #802
* fix(deps): update module google.golang.org/protobuf to v1.31.0 by @renovate in #811
* chore(deps): update newrelic/k8s-events-forwarder docker tag to v1.45.0 by @renovate in #814
* chore(deps): update newrelic/infrastructure-bundle docker tag to v3.2.11 by @renovate in #816
* fix(deps): update kubernetes packages to v0.27.4 by @renovate in #815
* chore(deps): update module golang.org/x/crypto to v0.11.0 by @renovate in #813
* chore(deps): update newrelic/infrastructure-bundle docker tag to v3.2.12 by @renovate in #818

## New Contributors
@isaacadeleke-nr made their first contribution in #799

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.15.1...v3.15.2

## 3.15.1

## What's Changed
* update changelog by @csongnr in https://github.com/newrelic/nri-kubernetes/pull/804
* update chart version and reference latest docker image by @csongnr in https://github.com/newrelic/nri-kubernetes/pull/805
* chore(deps): update newrelic/k8s-events-forwarder docker tag to v1.43.1 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/806
* chore(deps): update newrelic/infrastructure-bundle docker tag to v3.2.9 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/807
* Upgrade Go Version to 1.20 by @xqi-nr in https://github.com/newrelic/nri-kubernetes/pull/808


**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.15.0...v3.15.1

## 3.15.0 

## What's Changed
* Update CHANGELOG.md by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/783
* Bump versions by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/784
* fix(deps): update module github.com/sirupsen/logrus to v1.9.3 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/785
* chore(deps): update aquasecurity/trivy-action action to v0.11.0 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/787
* chore(deps): update newrelic/k8s-events-forwarder docker tag to v1.42.5 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/789
* chore(deps): update newrelic/infrastructure-bundle docker tag to v3.2.7 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/788
* Automate generating static test data for all supported Kubernetes versions by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/792
* Update E2E readme file by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/801
* chore(deps): update aquasecurity/trivy-action action to v0.11.2 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/794
* chore(deps): update module golang.org/x/crypto to v0.10.0 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/795
* fix(deps): update kubernetes packages to v0.27.3 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/797
* bump alpine from 3.18.0 to 3.18.2 by @csongnr in https://github.com/newrelic/nri-kubernetes/pull/803
* chore(deps): update newrelic/k8s-events-forwarder docker tag to v1.43.0 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/800


**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.14.0...v3.15.0

## 3.14.0

## What's Changed

* Update chart and image versions by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/769
* Update static test data by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/776
* Update Prometheus dependencies by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/775
* Increase the number of parallel E2E tests by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/780
* Add demo mode for testing resources by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/779
* Update `datagen.sh` documentation by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/777
* chore(deps): update newrelic/k8s-events-forwarder docker tag to v1.42.3 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/781
* chore(deps): update newrelic/infrastructure-bundle docker tag to v3.2.4 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/782
* fix(deps): update module github.com/spf13/viper to v1.16.0 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/773
* fix(deps): update module github.com/stretchr/testify to v1.8.4 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/772

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.13.0...v3.14.0

## 3.13.0

## What's Changed

* Bump chart by @htroisi in https://github.com/newrelic/nri-kubernetes/pull/750
* Update kubelet static testing exclusions. by @htroisi in https://github.com/newrelic/nri-kubernetes/pull/759
* Update `datagen.sh` by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/762
* Silence log messages that mask testing errors by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/763
* fix(deps): update module github.com/stretchr/testify to v1.8.3 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/756
* chore(deps): update newrelic/k8s-events-forwarder docker tag to v1.42.1 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/755
* chore(deps): update newrelic/infrastructure-bundle docker tag to v3.2.2 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/751
* chore(deps): bump github.com/sirupsen/logrus from 1.9.0 to 1.9.2 by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/765
* fix(deps): update kubernetes packages to v0.27.2 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/740
* NEWRELIC-5968 Mount containerd socket by @DavSanchez in https://github.com/newrelic/nri-kubernetes/pull/734
* Integration tests for k8s 1.27 by @csongnr in https://github.com/newrelic/nri-kubernetes/pull/719
* chore(deps): update module golang.org/x/crypto to v0.9.0 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/739

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.12.0...v3.13.0

## 3.12.0

## What's Changed

* Bump app and chart version by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/736
* Update renovatebot by @htroisi in https://github.com/newrelic/nri-kubernetes/pull/732
* Add more workload name fields to K8sContainerSample, K8sPodSample by @htroisi in https://github.com/newrelic/nri-kubernetes/pull/733
* chore(deps): update newrelic/k8s-events-forwarder docker tag to v1.41.0 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/748
* chore(deps): update alpine docker tag to v3.18.0 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/746
* Add PersistentVolume and PersistentVolumeClaim KSM metrics by @htroisi in https://github.com/newrelic/nri-kubernetes/pull/729

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.11.0...v3.12.0

## 3.11.0

### Changed

* chore(deps): update newrelic/k8s-events-forwarder docker tag to v1.40.0 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/712
* Bump app and chart version by @htroisi in https://github.com/newrelic/nri-kubernetes/pull/710
* chore(deps): update newrelic/infrastructure-bundle docker tag to v3.1.6 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/716
* Improve Cronjob chart for testing by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/715
* CronJob chart improvement by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/718
* chore(deps): update newrelic/infrastructure-bundle docker tag to v3.1.7 by @renovate in https://github.com/newrelic/nri-kubernetes/pull/717
* Remove manual go cache since setup-go/v4 automatically caches by @htroisi in https://github.com/newrelic/nri-kubernetes/pull/713
* chore(deps): bump actions/github-script from 6.4.0 to 6.4.1 by @dependabot in https://github.com/newrelic/nri-kubernetes/pull/714
* Add spec for backoffLimit to help with testing cronjobs by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/720
* Add KSM metrics promoted to stable category by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/724
* Fix push_pr workflow by @htroisi in https://github.com/newrelic/nri-kubernetes/pull/726
* Improve testing for deployment object by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/730
* Adding new attribute `ReplicaFailure` to deployment workload by @juanjjaramillo in https://github.com/newrelic/nri-kubernetes/pull/725
* chore(deps): bump aquasecurity/trivy-action from 0.9.2 to 0.10.0 by @htroisi in https://github.com/newrelic/nri-kubernetes/pull/727
* Fix Helm unittests by @htroisi in https://github.com/newrelic/nri-kubernetes/pull/728
* Update infrastructure-bundle to v3.1.8 and k8s-events-forwarder to v1.40.1 by @htroisi in https://github.com/newrelic/nri-kubernetes/pull/731

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.10.0...v3.11.0

## 3.10.0

### Changed

* Add CronJob and Job kube-state-metrics collection
* Make ProbeTimeout and ProbeBackoff Configurable
* Updated dependencies

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.9.0...v3.10.0

## 3.9.0

### Changed

* Updated dependencies
* Update Kubernetes image registry

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.7.0...v3.9.0

## 3.7.0

### Changed

* Fix an issue where when the Kubelet becomes temporarily unavailable the agent fails: https://github.com/newrelic/nri-kubernetes/pull/633
* Update e2e testing chart templates
* Updated dependencies

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.6.0...v3.7.0

## 3.6.0

### Added

* Add support for kube-state-metrics v2

### Changed

* Update static test data to use KSM v2
* Update kube-state-metrics version in e2e testing chart

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.5.0...v3.6.0

## 3.5.0

### Changed

Updated go version and several dependencies

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.4.1...v3.5.0

## 3.4.1

### Fix

In version above 1.21 having the apiServer flag `service-account-extend-token-expiration` set to false was causing the kubelet scraper pod to be restarted each time the token expired.
In AWS having environments due to its implementation caused a pod restart each 90days

### Changed

Updated several dependencies

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.4.0...v3.4.1

## 3.4.0

### Added

* Add k8s v1.23 & v1.24 new metrics [[#485](https://github.com/newrelic/nri-kubernetes/pull/485), [#507](https://github.com/newrelic/nri-kubernetes/pull/507)]:
  * `apiserverCurrentInflightRequestsMutating`
  * `apiserverCurrentInflightRequestsReadOnly`
  * `containerOOMEventsDelta`
  * `nodeCollectorEvictionsDelta`
  * `schedulerPendingPodsActive`
  * `schedulerPendingPodsBackoff`
  * `schedulerPendingPodsUnschedulable`

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.3.1...v3.4.0

## 3.3.1

### Added

* Add nrFiltered attribute to K8sNamespaceSamples when using namespace filtering https://github.com/newrelic/nri-kubernetes/pull/496

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.3.0...v3.3.1

## 3.3.0

### Added

* Allow filter to only scrape selected namespaces in ksm and kubelet by @alvarocabanas and @marcsanmi in  https://github.com/newrelic/nri-kubernetes/pull/457, https://github.com/newrelic/nri-kubernetes/pull/476 and https://github.com/newrelic/nri-kubernetes/pull/487

### Changed

* Use Go version 1.18 in the pipelines @roobre https://github.com/newrelic/nri-kubernetes/pull/472

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.2.1...v3.3.0

## 3.2.1

### Changed

* fix: round up CPU allocatable and capacity metrics by @gsanchezgavier in https://github.com/newrelic/nri-kubernetes/pull/412
* Dockerfile: use COPY instead of ADD by @roobre in https://github.com/newrelic/nri-kubernetes/pull/433
* chore(deps): bump alpine from 3.15.4 to 3.16.0 by @dependabot in https://github.com/newrelic/nri-kubernetes/pull/458

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.2.0...v3.2.1

## 3.2.0

### Added

* Initial, internal-only implementation of mTLS-enabled sink by @roobre in https://github.com/newrelic/nri-kubernetes/pull/338
* `restartCount` metric for pods is now also available as `restartCountDelta` by @sigilioso in https://github.com/newrelic/nri-kubernetes/pull/382

### Fixed

* `isReady` metric is now correctly reported as `false` (rather than `NULL`) for pending pods by @paologallinaharbur in https://github.com/newrelic/nri-kubernetes/pull/404

### Dependencies

* chore(deps): bump github.com/google/go-cmp from 0.5.7 to 0.5.8 by @dependabot in https://github.com/newrelic/nri-kubernetes/pull/406
* chore(deps): bump alpine from 3.15.0 to 3.15.4 by @dependabot in https://github.com/newrelic/nri-kubernetes/pull/391
* chore(deps): bump github.com/newrelic/infra-integrations-sdk from 3.7.1+incompatible to 3.7.2+incompatible by @dependabot in https://github.com/newrelic/nri-kubernetes/pull/372
* chore(deps): bump github.com/sethgrid/pester from 1.1.0 to 1.2.0 by @dependabot in https://github.com/newrelic/nri-kubernetes/pull/358
* Update dependencies to solve security issues pointed by trivy by @kang-makes in https://github.com/newrelic/nri-kubernetes/pull/403

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.1.0...v3.1.1

## 3.1.0

* chore(deps): bump aquasecurity/trivy-action from 0.2.1 to 0.2.2 by @dependabot in https://github.com/newrelic/nri-kubernetes/pull/355
* controlplane/authenticator: allow to use `kubernetes.io/tls` secrets by @roobre in https://github.com/newrelic/nri-kubernetes/pull/344
* config: document options by @roobre in https://github.com/newrelic/nri-kubernetes/pull/363

**Full Changelog**: https://github.com/newrelic/nri-kubernetes/compare/v3.0.0...v3.1.0

## 3.0.0

This new version makes significant changes to the number of components that are deployed to the cluster, and introduces many new configuration options to tune the behavior to your environment. We encourage you to take a look at what's changed in full detail [here](/docs/kubernetes-pixie/kubernetes-integration/get-started/changes-since-v3/).

### Breaking changes

<Callout variant="tip">
  The number and format of the metrics reported by version 3 of the integration have not changed with respect to earlier versions.
</Callout>

* The format of the `values.yml` file has changed to accommodate the newly added configuration options. Please take a look at our [migration guide](/docs/kubernetes-pixie/kubernetes-integration/get-started/changes-since-v3/#migration-guide) to see how to change your configuration.

### Changed

* Our solution is now deployed in three components:
  * A `DaemonSet` to monitor the Kubelet, deployed in all nodes of the cluster.
  * A second `DaemonSet` to monitor the control plane, deployed in master nodes only.
  * A `Deployment` to collect metrics from kube-state-metrics, deployed in the same node as the latter.
* We now offer better control for CPU and memory limits and requests, which can be now configured for the three components individually.
* Impact of discovery and collection operations on the API server has been greatly reduced, thanks to the use of kubernetes [informers](https://pkg.go.dev/k8s.io/client-go/informers).
* Logs messages have been greatly revamped to surface problems more clearly.

### Added

* Comprehensive configuration options have been added to provide fine-grain control to how the integration discovers and connects to metric providers. Remarkably:
  * Discovery options for control plane components have been improved. You can check the details on how discovery is configured [here](/docs/kubernetes-pixie/kubernetes-integration/advanced-configuration/configure-control-plane-monitoring).
  * It is now possible to collect metrics from control plane components running outside of the cluster.
  * Discovery options for KSM and the kubelet have also been added.
* The interval at which metrics are collected is now [configurable](/docs/kubernetes-pixie/kubernetes-integration/installation/install-kubernetes-integration-using-helm#scrape-interval).

## 2.9.0

### Added

* Moved default config.sample to [V4](https://docs.newrelic.com/docs/create-integrations/infrastructure-integrations-sdk/specifications/host-integrations-newer-configuration-format/), added a dependency for infra-agent version 1.20.0

Please notice that old [V3](https://docs.newrelic.com/docs/create-integrations/infrastructure-integrations-sdk/specifications/host-integrations-standard-configuration-format/) configuration format is deprecated, but still supported.

## 2.8.3

### Changed

* Updated agent and integrations to their latest version.

## 2.8.2

### Changed

* Updated agent and integrations to their latest version.

## 2.8.1

### Changed

* Node status and conditions are now fetched from the API Server rather than KSM, which fixes some inconsistencies in the samples. This does not change which data is reported, and should be an invisible change. (https://github.com/newrelic/nri-kubernetes/pull/194).
* Add a series of parameters which allow to configure a jitter to be applied to API Server response caching, which might help to spread the load on large clusters. (https://github.com/newrelic/nri-kubernetes/pull/185).

## 2.7.1

> Note: This is an out-of-order release which brings some hotfixes to the 2.7.x branch

### Changed

* Node status and conditions are now fetched from the API Server rather than KSM, which fixes some inconsistencies in the samples. This does not change which data is reported, and should be an invisible change.

## 2.8.0

### Added

* Kubernetes v1.22.0 Support

### Changed

* Upgrade infrastructure-bundle to 2.6.4 (#123)
  * See https://github.com/newrelic/infrastructure-bundle/releases/tag/2.6.4 for more details about the upgraded integrations in this release of the infrastructure-bundle

## 2.7.0

### Added

* Integration now reports node status and conditions, as `condition.{Name}` (e.g. `condition.Ready`, `condition.PIDPressure`).
* Added new KubeStateMetricsNamespace parameter to restrict discovery of KSM pod to a particular namespace.
  * This should help reduce load in the control plane for clusters with many pods and/or nodes.

## 2.6.1

### Fixed

* Integration version shown in the samples.

## 2.6.0

### Changed

* Upgrade infrastructure-bundle to 2.6.0 (#123)
  * See https://github.com/newrelic/infrastructure-bundle/releases/tag/2.6.0 for more details about the upgraded integrations in this release of the infrastructure-bundle

## 2.5.0

### Changed

* Bumped all dependencies and moved to /v2 in go.mod https://github.com/newrelic/nri-kubernetes/pull/111
* Improved e2e tests with more coverage and support for Helm3 and k8s 1.20-1.21 https://github.com/newrelic/nri-kubernetes/pull/110 https://github.com/newrelic/nri-kubernetes/pull/108
* Improved KSM discovery logic https://github.com/newrelic/nri-kubernetes/pull/104

## 2.4.0

### Added

* Support for multiarch docker images

## 2.3.1

### Fixed

* Correctly identifying k8s server version with characters (#81)

## 2.3.0

### Changed

* The base image of `newrelic/infrastructure-k8s` has been updated to `2.2.3`.
  More info regarding all the integrations upgraded can be found in the [release notes of the base image](https://github.com/newrelic/infrastructure-bundle/releases/tag/2.2.3).
* Changed scale of node `cpuRequestedCores` to cores from millis

### Added

* Added metrics pertaining to Horizontal Pod Autoscaler. More information about the collected metrics can be found in the [official documentation](https://docs.newrelic.com/docs/integrations/kubernetes-integration/understand-use-data/find-use-your-kubernetes-data)

### Fixed

* LoadBalancerIP was not being collected properly. It is now fetched from KSM metric `kube_service_status_load_balancer_ingress`

## 2.2.0

### Changed

* The base image of `newrelic/infrastructure-k8s` has been updated to `2.2.1`.
  This base image has fixed an issue where `nrjmx` was not properly running due to the bundled java version.
  More info regarding all the integration upgraded can be found in the [release notes of the base image](https://github.com/newrelic/infrastructure-bundle/releases/tag/2.2.1).

## 2.1.0

### Changed

* Added aggregate cpu and memory requests for nodes

## 2.0.0

### Changed

* The base image of `newrelic/infrastructure-k8s` has been updated to `2.0.0`.
  That base image is bundling the integration `nri-nginx` `3.0.2` that contains a breaking change.
  More info regarding all the integration upgraded can be found in the [release notes of the base image](https://github.com/newrelic/infrastructure-bundle/releases/tag/2.0.0).

## 1.26.9

### Changed

* Added release pipeline to Github Actions.

## 1.26.8

### Changed

* Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.6.0.
  For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.6.0).

## 1.26.7

### Fixed

* When configuring API_SERVER_ENDPOINT_URL with https endpoint, the
  api-server component will use that that instead of the default.
  More info at issue #41

## 1.26.6

### Fixed

* When discovering kube-state-metrics behind a headless service, the
  DNS discovery will return an error. Before it would be considered
  successful and return "None" as endpoint.

## 1.26.5

### Fixed

* Container id's are correctly parsed when using system driver

## 1.26.4

### Added

* Added `restartCount` to containers in the `wanting` state
  * In case the container is in a crash loop the restart count would not be shown

## 1.26.3

### Added

* Added `restartCount` to containers in the `terminated` state
  * In case the container is in a crash loop the restart count would not be shown

## 1.26.2

### Changed

* Upgrade Docker image to use Tini entrypoint solving:
  * reaps orphaned zombie process attached to PID 1
  * correctly forwards signals to CMD process
  Read for more details: https://blog.phusion.nl/2015/01/20/docker-and-the-pid-1-zombie-reaping-problem/
* Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.5.1.
  For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.5.1).

## 1.26.1

### Changed

* Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.5.0.
  For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.5.0).

## 1.26.0

### Changed

* When querying the summary endpoint from Kubelet to get the Node or Pod
  network metrics, if the default network interface is not eth0 then summary
  endpoint for Kubelet doesn't return the metrics as we expect them. We rely on
  them being a direct member of the "network" object. See rxBytes, txBytes and
  rxErrors in the following example metrics:

```
"network": {
 "time": "2020-06-04T10:01:15Z",
 "name": "eth0",
 "rxBytes": 207909096,
 "rxErrors": 0,
 "txBytes": 8970981,
 "txErrors": 0,
 "interfaces": [
  {
   "name": "eth0",
   "rxBytes": 207909096,
   "rxErrors": 0,
   "txBytes": 8970981,
   "txErrors": 0
  },
  {
   "name": "ip6tnl0",
   "rxBytes": 0,
   "rxErrors": 0,
   "txBytes": 0,
   "txErrors": 0
  },
  {
   "name": "tunl0",
   "rxBytes": 0,
   "rxErrors": 0,
   "txBytes": 0,
   "txErrors": 0
  }
 ]
}
```

  This scenario only happens when the default interface is eth0. Kubernetes
  source code has it hardcoded that eth0 is the default. In the following
  example you can see that we only have network metrics inside the interfaces
  list, in this case there is no eth0 on the and the default interface is ens5:

```
"network": {
 "time": "2020-06-04T10:01:15Z",
 "name": "",
 "interfaces": [
  {
   "name": "ens5",
   "rxBytes": 207909096,
   "rxErrors": 42,
   "txBytes": 8970981,
   "txErrors": 24
  },
  {
   "name": "ip6tnl0",
   "rxBytes": 0,
   "rxErrors": 0,
   "txBytes": 0,
   "txErrors": 0
  },
  {
   "name": "tunl0",
   "rxBytes": 0,
   "rxErrors": 0,
   "txBytes": 0,
   "txErrors": 0
  }
 ]
```

  In cases like this, the integration will look for the default interface
  inside the interfaces list and use those values. The default interface name
  is retrieved from the network route file (default /proc/net/route).

  When running the unprivileged version of the integration we don't have access
  to the route file, the integration won't be able to get the default interface
  name and won't send network metrics for the unless there's a network
  interface called eth0.

  For Pods, this issue is mainly present when using hostNetwok since they
  shared the same network interfaces with the Node.

* Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.4.2.
  For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.4.2).

## 1.25.0

### Added

* Support for Kubernetes versions 1.17.X

### Changed

* Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.4.1.
  For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.4.1).

* The API server is queried by default on the Secure Port using the service account's bearer authentication.
  If the query on the Secure Port fails, it will fallback automatically to the non-secure one. This should preserve
  the same behavior as previous versions.

## 1.24.0

### Changed

* Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.4.0.
   For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.4.0).

## 1.23.1

### Fixed

* Bug that swapped values of node allocatable resources with node capacity
  resources.

## 1.23.0

### Added

* Kubernetes 1.16 is now officially supported.
  * The minimum supported version of kube-state-metrics for this release is 1.9.5, according to the [KSM compatibility matrix](https://github.com/kubernetes/kube-state-metrics#compatibility-matrix)
* Added container throttling metrics to the K8sContainerSample:
  * `containerCpuCfsPeriodsDelta`: Delta change of elapsed enforcement period intervals.
  * `containerCpuCfsThrottledPeriodsDelta`: Delta change of throttled period intervals.
  * `containerCpuCfsThrottledSecondsDelta`:  Delta change of duration the container has been throttled.
  * `containerCpuCfsPeriodsTotal`: Number of elapsed enforcement period intervals.
  * `containerCpuCfsThrottledPeriodsTotal`: Number of throttled period intervals.
  * `containerCpuCfsThrottledSecondsTotal`: Total time duration the container has been throttled.
* Added container mmap byte usage metrics to the K8sContainerSample:
  * `containerMemoryMappedFileBytes`: Size of memory mapped files in bytes.

## 1.22.0

### Changed

* Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.3.9.
   For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.3.9).

## 1.21.0

### Added

* Resources allocatable and capacity are retrieved from the apiserver and
   added to the `K8sNodeSample` as `capacity<ResourceName>` and
   `allocatable<ResourceName>`.
* The Kubernetes server version is retrieved from the apiserver and cached
   with the `APIServerCacheK8SVersionTTL` config option. The Kubernetes version
   is added to the `K8sClusterSample` as `clusterK8sVersion` and to inventory.
* Add support for static pods status for Kubernetes server versions 1.15 or
   newer.

### Changed

* Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.3.8.
   For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.3.8).

## 1.20.0

### Changed

* Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.3.5.
   For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.3.5).

## 1.19.0

### Added

* New label combination to discover the Kubernetes controller manager:
  * `app=controller-manager`
  * `controller-manager=true`

### Changed

* Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.3.4.
   For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.3.4).

## 1.18.0

### Changed

* Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.3.2.
   For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.3.2).

## 1.17.0

### Added

* Added the necessary files for building a windows image of the integration.
   The windows image needs to be manually created and it's still not in our
   CI/CD pipeline. We have the files for building it but we are not publishing
   it. The latest supported image for Windows, at the time of writing, is
   1.16.0.
* Upgraded Docker base image `newrelic/infrastructure-bundle` to v1.3.0.
   For more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.3.0).

## 1.16.0

### Added

* Support for completely avoid querying Kube State Metrics. It's behind the `DISABLE_KUBE_STATE_METRICS` environment variable
   and its default value is `false`. Note that disabling this will imply in missing metrics that are collected from KSM
   and possibly missing features in the Kubernetes Cluster Explorer. Please, refer to our [official documentation on this
   configuration option](https://docs.newrelic.com/docs/integrations/kubernetes-integration/installation/kubernetes-installation-configuration#disable-kube-state-metrics) for more information.

## 1.15.0

### Added

* Support for querying Kube State Metrics instances behind [kube-rbac-proxy](https://github.com/brancz/kube-rbac-proxy).
   It **ONLY works when the label-based KSM discovery is enabled through the `KUBE_STATE_METRICS_POD_LABEL` environment variable**.
   2 new configuration environment variables are added:
  * KUBE_STATE_METRICS_SCHEME: defaults to `http`. Valid values are `http` and `https`.
  * KUBE_STATE_METRICS_PORT: defaults to `8080`. On a standard setup of **kube-rbac-proxy** this should be set to `8443`.
* OpenShift Control Plane components are now automatically discovered.
* Added 4 new environment variables to explicitly set the Control Plane components URLs:
  * SCHEDULER_ENDPOINT_URL
  * ETCD_ENDPOINT_URL
  * CONTROLLER_MANAGER_ENDPOINT_URL
  * API_SERVER_ENDPOINT_URL

### Fixed

* Fix a bug that was preventing `selector.<key>` type attributes to not be
  added to some of the `K8sServiceSample`.
* The integration now uses `newrelic/infrastructure-bundle` as the base image. The version used
   is `1.2.0`, for more information on the release please see the [New Relic Infrastructure Bundle release notes](https://github.com/newrelic/infrastructure-bundle/releases/tag/1.2.0).

## 1.13.2

### Changed

* The integration now uses the infrastructure agent v1.9.0-bundle. For more
   information refer to the [infrastructure agent release notes](https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/)
   between versions v1.8.32 and v1.9.0.

## 1.13.1

### Added

* Added daemonsetName field to the K8sDaemonsetSample

## 1.13.0

### Added

* Added samples for Statefulsets, Daemonsets, Endpoints and Services.
* API Server metrics can now be queried using the secure port. Configure the port using the `API_SERVER_SECURE_PORT` environment variable. The ClusterRole has been updated to allow this query to happen.
* The integration now uses the infrastructure agent v1.8.32-bundle. For more
   information refer to the [infrastructure agent release notes](https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/)
   between versions v1.8.23 and v1.8.32.

   The bundle container contains a subset of [On-host integrations](https://docs.newrelic.com/docs/integrations/new-relic-integrations/get-started/introduction-infrastructure-integrations) that are supported by New Relic.
   This also includes the ability to "Auto Discover" services running on Kubernetes in a similar way to our [Container auto-discovery](https://docs.newrelic.com/docs/integrations/host-integrations/installation/container-auto-discovery)
* The integration has been renamed from `nr-kubernetes` to `nri-kubernetes`.

## 1.12.0

### Changed

* Control Plane components can now also be discovered using the `tier` and `component` labels, besides `k8s-app`.
* The integration now uses the infrastructure agent v1.8.23. For more
   information refer to the [infrastructure agent release notes](https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/)
   between versions v1.5.75 and v1.8.23.

## 1.11.0

### Changed

* The old way of determining Leader/Follower status has been switched to a
   job based architecture. The old Leader/Follower was needed to detect which nri-kubernetes Pod
   should query Kube State Metrics (a.k.a. the Leader), but it was hard to add additional scrape targets (e.g. control plane).
   Important notice: The output logs have been changed. Before the integration logged whether
   it was Follower or a Leader, and this has been changed to show which jobs are executed.

   Following are 2 examples of logs that are being output by the integration before and after this update.

   Before, logs for leaders:

   ```
   Auto-discovered role: Leader
   ```

   After, equivalent example, now using jobs:

   ```
   Running job: kubelet
   Running job: kube state metrics
   ```

   Before, logs for followers:

   ```
   Auto-disovered role: Follower
   ```

   After, equivalent example, now using jobs:

   ```
   Running job: kubelet
   ```

   These 2 before & after examples are identical in the targets & information they scrape.

* The e2e test package has been updated to work with this refactor.

### Added

* Control Plane Monitoring: the integration will automatically detect if it's running on a master node using
   its Kubernetes pod's labels, which are retrieved from the API Server. If it finds itself running on a master
   node, these additional jobs will run:
  * ETCD
  * API Server
  * Controller Manager
  * Scheduler

   All jobs, except ETCD, will work out of the box with no further configuration needed.
   ETCD exposes its metrics using Mutual TLS, which can be configured as follows.

   First, create a secret containing the following fields:

   ```
   key: <private_key_data, PEM format>
   cert: <certificate_belonging_to_private_key, PEM format>
   cacert: <optional, the ETCD cacert, PEM format>
   insecureSkipVerify: <optional, bool 'true' or 'false', default: 'false'>
   ```

   Which can be created like this (expecting the files `key`, `cert` and `cacert` to be present):

   ```
   kubectl create secret generic etcd-server-tls --from-file=./key --from-file=./cert --from-file=./cacert
   ```

   Then, configure the integration to use this secret using these environment variables.

   ```
   ETCD_TLS_SECRET_NAME: etcd-server-tls
   ETCD_TLS_SECRET_NAMESPACE: default
   ```

   If everything is configured properly the integration should start collecting ETCD metrics.
* A new command, called kubernetes-static has been added, which enables the
   integration to be run locally on your machine, without deploying it to k8s.
   It uses a static set of exports from kubelet & KSM.
* A new way to query a specific Kube State Metrics (KSM) pod  when running multiple redundant pods: by Label Selector.
   If you want to target a certain KSM instance, you can now use the `KUBE_STATE_METRICS_POD_LABEL` environment variable.
   If this variable has been set (and KUBE_STATE_METRICS_URL is unset) the integration will find the KSM pod by this variable.

   For example:

      ```shell script
      # Label a specific KSM pod. Always set the value to the string "true".
      kubectl label pod kube-state-metrics please-use-this-ksm-pod=true
      ```

   Configure `nri-kubernetes` to use this KSM pod:

   ```yaml
    env:
    - name: KUBE_STATE_METRICS_POD_LABEL
      value: please-use-this-ksm-pod
    ```

## 1.10.2

### Added

* The integration now uses the infrastructure agent v1.5.75. For more
  information refer to the [infrastructure agent release notes](https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/)
  between versions v1.5.31 and v1.5.75.

## 1.10.1

### Changed

* Rollback agent version to v1.5.31 because there is an issue with nodes
  reporting inventory using the node ip as entity key, this causes the nodes to
  be indexed as clusters.

## 1.10.0

### Added

* Node labes are added to the `K8sNodeSample`. They are retrieved from the k8s
  api and cached.

* The integration now uses the infrastructure agent v1.5.51. For more
  information refer to the [infrastructure agent release notes](https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/)
  between versions v1.5.31 and v1.5.51.

## 1.9.5

### Changed

* The integration now uses the Infrastructu Agent v1.5.31. The biggest changes were major improvements to logging and
  to the StorageSampler. For more information refer to the [infrastructure agent release notes](https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/) between versions v1.3.18 and v1.5.31.

## 1.9.4

### Fixed

* No code changes have been made. This fixes a regression at Docker image level related to https://github.com/moby/moby/issues/35443.

## 1.9.3

### Added

* Support for discovering KSMs when running with the label `app.kubernetes.io/name`.

## 1.9.2

### Fixed

* No code changes has been made. The fix is at docker image level. We got affected by https://github.com/moby/moby/issues/35443.

## 1.9.1

### Fixed

* The unprivileged integration runs always as `nri-agent` user. Fixes https://github.com/kubernetes/kubernetes/issues/78308.
* Infrastructure agent is now behaving in secure-forwarder mode.
* Autodiscovery cache directory permissions got changed from 644 to 744 in order to let the nri-agent user write inside.

## 1.9.0

### Changed

* The integraion now uses the infrastructure agent v1.3.18 instead of 1.1.14. Refer to the
  [infrastructure agent release notes](https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/new-relic-infrastructure-agent-1318)
  for more information about all the changes from this upgrade.

## 1.8.0

### Added

* The integration reports the name of the cluster as Infrastructure inventory.

* The integration reports a new event type `K8sClusterSample`. At this moment,
  these events contain only the cluster name as an attribute.

## 1.7.0

### Added

* Support for kube-state-metrics v1.5.

* Pod's status reason and status message are now sent in the `K8sPodSample` as `reason` and `message` fields.
* Container's `memory_working_set_bytes` is now sent in the `K8sContainerSample` as `workingSetBytes`.

### Changed

* Always request metrics from kube-state-metrics in the text format. In kube-state-metrics v1.5 this is the default
regardless of the format requested.

## 1.6.0

### Added

* `namespaceName` metric attribute was added to all the samples where `namespace` attribute is present.

### Deprecated

* `namespace` metric attribute will be removed soon. Please use `namespaceName` from now on.

## 1.5.0

### Changed

* Due to an issue in Kubelet, we stopped reporting the Status of static pods. See https://github.com/kubernetes/kubernetes/issues/61717.

## 1.4.0

### Changed

* Update base image in the dockerfile to use latest newrelic/infrastructure
  version: 0.0.62 (Infrastructure agent v1.1.14, released at: 2018-12-20)

## 1.3.1

### Added

* Add clusterName custom attribute to manifest file. This helps users correlate
  Kubernetes integration data with Infrastructure agent data.

### Changed

* `KUBE_STATE_METRICS_URL` environment variable can be specified containing only host & port
  or it can be the complete URL including also the `/metrics` path (ex:
  `http://my-service.my-ns.svc.cluster.local:8080/metrics`).

### Fixed

* Fix how the usage percentage is calculated for container filesystem metrics.

* Fix how the usage percentage is calculated for volumes.

## 1.3.0

### Added

* Add metrics for volumes (persistent and non-persistent volumes).

* Add container filesystem metrics.

## 1.2.0

### Added

* Add `reason` metric for terminated containers

## 1.1.0

### Added

* Support for specifying the K8s API Host and Port by setting the `KUBERNETES_SERVICE_HOST` and `KUBERNETES_SERVICE_PORT` env vars.

### Changed

* Improve readability of log messages, when verbose mode is enabled.

### Fixed

* Kubernetes API url discovery failed sometimes giving errors like "error trying to connect to...". Now this should be fixed.

## 1.0.0

### Changed

* The agent tag installed within the integration docker image is now fixed to 0.0.24.

## 1.0.0-beta2.4

### Added

* Add `hostNetwork: true` option and the required dns policy to daemonset file. This is a requirement for the Infrastructure Agent to report the proper hostname in New Relic.

### Changed

* Update newrelic-infra.yaml to force our objects to be deployed in `default` namespace.

* Add NoExecute toleration ensuring that our pod is being deployed when the NoExecute node taint is set.

### Fixed

* Add missing metric: `podsMaxUnavailable` for deployment

* Fix some of the metrics for pods in pending status
  * Adding missing metrics: `startTime`, `isReady`
  * Unifying `isScheduled` and `isReady` to be reported as `1` and `0` for `true` and `false` respectively.
* Fix pod metrics (`status` and `isReady`): non-scheduled or pending pods were not reported correctly.

## 1.0.0-beta2.3

### Added

* Add configurable flag for kube-state-metrics endpoint (only HTTP).

* Add additional label `app` for discovering kube-state-metrics endpoint.

### Changed

* Kubelet discovery process fetches now the nodeName directly from the spec using downward API.

## 1.0.0-beta2.2

### Fixed

* Fix bug in error handling where recoverable errors made the integration to panic.

## 1.0.0-beta2.1

### Added

* Allow direct connection to cAdvisor by specifying the port.

### Fixed

* Call to CAdvisor was failing when Kubelet was secure.

## 1.0.0-beta2.0

### Added

* nodes/metrics resource was added to the newrelic cluster role.

### Changed

* CAdvisor call is now bypassing Kubelet endpoint talking then directly to CAdvisor port

## 1.0.0-beta1.0

Initial public beta release.

## 1.0.0-alpha5.1

### Changed

* TransformFunc now handles errors.

* Add checks for missing data coming from kube-state-metrics.
* Boolean values have changed from `"true"` and `"false"` to `1` and `0` respectively from the following metrics:
  1. isReady and isScheduled for pods.
  2. isReady for containers.
* Update metrics
  1. `errorCountPerSecond` to `errorsPerSecond` for pods and nodes.
  2. `usageCoreSeconds` to `cpuUsedCoreMilliseconds` for nodes.
  3. `memoryMajorPageFaults` to `memoryMajorPageFaultsPerSecond` for nodes.

### Fixed

* Calculate properly RATE metrics.

## 1.0.0-alpha5

### Added

* TypeGenerator for entities.

* Caching discovered endpoints on disk.
* Implementation of Time-To-Live (TTL) cache expiry functionality.
* Added the concept of Leader and Follower roles.
  * Leader represents the node where Kube State Metrics is installed (so only 1 by cluster).
  * Follower represents any other node.
* Both Follower and Leader call kubelet /pods endpoint in order to get metrics that were previously fetched from KSM.
* Fetch metrics from KSM about pods with status "Pending".
* Prometheus TextToProtoHandleFunc as http.HandlerFunc.
  Useful for serving a Prometheus payload in protobuf format from a plain text reader.
* Both Follower and Leader call kubelet /metrics/cadvisor endpoint in order to fill some missing metrics coming from Kubelet.

### Changed

* Rename `endpoints` package to `client` package.

* Moved a bunch of functions related to `Prometheus` from `ksm` package to `prometheus` one.
* Renamed the recently moved `Prometheus` functions. Removed **Prometheus** word as it is considered redundant.
* Containers objects reported as their own entities (not as part of pod entities).
* NewRelic infra Daemonset updateStrategy set to RollingUpdate in newrelic-infra.yaml.
* Prometheus CounterValue type changed from uint to float64.
* Change our daemonset file to deploy the integration in "default" namespace.
* Prometheus queries now require to use an operator.
* Prometheus Do method now requires a metrics endpoint.

### Removed

* Follower does not call KSM endpoints anymore.

* Config package with default unknown namespace value
* Removed legacy Kubernetes spec files.

### Fixed

* Replace `log.Fatal()` by `log.Panic()` in order to call all defer statements.

* Skip missing data from /stats/summary endpoint, instead of reporting them as zero values.
* Entities not reported in case of problem with setting their name or type.

## 1.0.0-alpha4

### Added

* Adding node metrics. Data is fetched from Kubelet and kube-state-metrics.

* Adding toleration for the "NoSchedule" taint, so the integration is deployed on all nodes.
* Adding new autodiscovery flow with authentication and authorization mechanisms.

### Removed

* Custom arguments for kubelet and kube-state-metrics endpoints.

### Fixed

* Integration stops on KSM or Kubelet connection error, instead of continuing.

## 1.0.0-alpha3

### Changed

* `updatedAt` metric was renamed to `podsUpdated`.

* `cpuUsedCores` has been divided by 10^9, to show actual cores instead of nanocores.
* Update configurable timeout flag using it to connect to kubelet and kube-state-metrics.

### Fixed

* Fix debug log level when verbose. Some parts of the code didn't log debug information.

## 1.0.0-alpha2

### Added

* Metrics for unscheduled Pods.

### Fixed

* Fix format of inherited labels. Remove unnecessary prefix `label_` included by kube-state-metrics.

* Fix labels inheritance. Labels weren't propagating between "entities" correctly.

## 1.0.0-alpha

### Added

* Initial version reporting metrics about Namespaces, Deployments, ReplicaSets,
  Pods and Containers. This data is fetched from two different sources: Kubelet
  and kube-state-metrics.
