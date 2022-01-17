[![Community Plus header](https://github.com/newrelic/opensource-website/raw/master/src/images/categories/Community_Plus.png)](https://opensource.newrelic.com/oss-category/#community-plus)

# New Relic integration for Kubernetes

New Relic's Kubernetes integration gives you full observability into the health and performance of your environment,
no matter whether you run Kubernetes on-premises or in the cloud.
It gives you visibility about Kubernetes namespaces, deployments, replica sets, nodes, pods, and containers.
Metrics are collected from different sources:
* [kube-state-metrics service](https://github.com/kubernetes/kube-state-metrics) provides information about state of
Kubernetes objects like namespace, replicaset, deployments and pods (when they are not in running state)
* `/stats/summary` kubelet endpoint gives information about network, errors, memory and CPU usage
* `/pods` kubelet endpoint provides information about state of running pods and containers
* `/metrics/cadvisor` cAdvisor endpoint provides missing data that is not included in the previous sources.
* `/metrics` from control plane components: `ETCD`,`controllerManager`, `apiServer` and `scheduler`

Check out our [documentation](https://docs.newrelic.com/docs/kubernetes-integration-new-relic-infrastructure)
in order to find out more how to install and configure the integration, learn what metrics are captured
and how to query them.

## Table of contents

- [Table of contents](#table-of-contents)
- [Installation](#installation)
- [Usage](#usage)
- [Running the integration against a static data set](#running-the-integration-against-a-static-data-set)
- [Development](#development)
  - [E2E tests](#Run-e2e-Tests)
  - [Tests](#tests)
- [Running OpenShift locally using CodeReady Containers](#running-openshift-locally-using-codeready-containers)
- [Support](#support)
- [Contributing](#contributing)
- [License](#license)

## Installation

Start by checking the
[compatibility and requirements](https://docs.newrelic.com/docs/integrations/kubernetes-integration/get-started/kubernetes-integration-compatibility-requirements) 
and then follow the
[installation steps](https://docs.newrelic.com/docs/kubernetes-monitoring-integration).

For troubleshooting, see
[Not seeing data](https://docs.newrelic.com/docs/integrations/host-integrations/troubleshooting/kubernetes-integration-troubleshooting-not-seeing-data)
or [Error messages](https://docs.newrelic.com/docs/integrations/host-integrations/troubleshooting/kubernetes-integration-troubleshooting-error-messages).

## Usage

Learn how to 
[find and use data](https://docs.newrelic.com/docs/integrations/kubernetes-integration/understand-use-data/understand-use-data)
and review the description of all 
[captured data](https://docs.newrelic.com/docs/integrations/kubernetes-integration/understand-use-data/understand-use-data#event-types).

## Running the integration against a static data set

 - See [cmd/kubernetes-static/readme.md](./cmd/kubernetes-static/readme.md) for more details regarding running the integration.
 - See [internal/testutil/datagen/README.md](./internal/testutil/datagen/README.md) for more details regarding generate new data.

## Development

### Run e2e Tests
- See [e2e/README.md](./e2e/README.md) for more details regarding running e2e tests.

### Tests

For running unit tests, run

```bash
make test
```

## Running OpenShift locally using CodeReady Containers

- See [OpenShift.md](./OpenShift.md) for more details regarding running locally OpenShift environments.

## Support

Should you need assistance with New Relic products, you are in good hands with several support diagnostic tools and support channels.

>New Relic offers NRDiag, [a client-side diagnostic utility](https://docs.newrelic.com/docs/using-new-relic/cross-product-functions/troubleshooting/new-relic-diagnostics) that automatically detects common problems with New Relic agents. If NRDiag detects a problem, it suggests troubleshooting steps. NRDiag can also automatically attach troubleshooting data to a New Relic Support ticket. Remove this section if it doesn't apply.

If the issue has been confirmed as a bug or is a feature request, file a GitHub issue.

**Support Channels**

* [New Relic Documentation](https://docs.newrelic.com): Comprehensive guidance for using our platform
* [New Relic Community](https://discuss.newrelic.com/t/new-relic-kubernetes-open-source-integration/109093): The best place to engage in troubleshooting questions
* [New Relic Developer](https://developer.newrelic.com/): Resources for building a custom observability applications
* [New Relic University](https://learn.newrelic.com/): A range of online training for New Relic users of every level
* [New Relic Technical Support](https://support.newrelic.com/) 24/7/365 ticketed support. Read more about our [Technical Support Offerings](https://docs.newrelic.com/docs/licenses/license-information/general-usage-licenses/support-plan).

## Privacy

At New Relic we take your privacy and the security of your information seriously, and are committed to protecting your information. We must emphasize the importance of not sharing personal data in public forums, and ask all users to scrub logs and diagnostic information for sensitive information, whether personal, proprietary, or otherwise.

We define “Personal Data” as any information relating to an identified or identifiable individual, including, for example, your name, phone number, post code or zip code, Device ID, IP address, and email address.

For more information, review [New Relic’s General Data Privacy Notice](https://newrelic.com/termsandconditions/privacy).

## Contribute

We encourage your contributions to improve this project! Keep in mind that when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. You only have to sign the CLA one time per project.

If you have any questions, or to execute our corporate CLA (which is required if your contribution is on behalf of a company), drop us an email at opensource@newrelic.com.

**A note about vulnerabilities**

As noted in our [security policy](../../security/policy), New Relic is committed to the privacy and security of our customers and their data. We believe that providing coordinated disclosure by security researchers and engaging with the security community are important means to achieve our security goals.

If you believe you have found a security vulnerability in this project or any of New Relic's products or websites, we welcome and greatly appreciate you reporting it to New Relic through [HackerOne](https://hackerone.com/newrelic).

If you would like to contribute to this project, review [these guidelines](./CONTRIBUTING.md).

To all contributors, we thank you!  Without your contribution, this project would not be what it is today.

## License

nri-kubernetes is licensed under the [Apache 2.0](http://apache.org/licenses/LICENSE-2.0.txt) License.
