# E2E tests
You can run e2e tests on any cluster, please notice that scraping control plane could be not possible or needing 
specific values depending on the flavour. The following instructions assume that e2e tests are ran inside a `Minikube` cluster with `containerd` runtime. 

In order to run e2e tests locally you can do the following.

Initialize a test cluster.
```shell
minikube start --kubernetes-version=vX.X.X --container-runtime=containerd
minikube addons enable metrics-server
```

Note that the control plane flags in `e2e-values.yml` have been set meeting the minikube specifications. 

Then you need to build the binary and the image. Notice that  since the Dockerfile includes multiarch
support, you may need to set `DOCKER_BUILDKIT=1` when running `docker build` for the `TARGETARCH`
and `TARGETOS` args to be populated.
```shell
make compile-multiarch # Compile the repo binaries that will be used to create an image for testing.
export  DOCKER_BUILDKIT=1
docker build -t e2e/nri-kubernetes:e2e  .
minikube image load e2e/nri-kubernetes:e2e
```

Then, include helm needed repositories.
```shell
helm repo add newrelic https://helm-charts.newrelic.com
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
```

You need to install the binary `https://github.com/newrelic/newrelic-integration-e2e-action/tree/main/newrelic-integration-e2e` used in the e2e test
```shell
go install github.com/newrelic/newrelic-integration-e2e-action@latest

```

You need New Relic license key (Ingest - License), Api key (User key) and Account before running the tests. More information on how to find these keys, please see [this](https://docs.newrelic.com/docs/apis/intro-apis/new-relic-api-keys/). 

Set the following environment variables:
```shell
export EXCEPTION_FILE="1_22-exceptions.yml"
export LICENSE_KEY=xxx
export API_KEY=xxx
export ACCOUNT_ID=xxx
```

Since some metrics are removed and added depending on the k8s version, the `EXCEPTION_FILE` should point, depending on the k8s version you are testing on, to one of the `/e2e/*-exception.yml` files.

Run the following command to execute the test and make sure that it is ran at the root of the repo:

```shell
LICENSE_KEY=${LICENSE_KEY} EXCEPTIONS_SOURCE_FILE=${EXCEPTION_FILE}  go run github.com/newrelic/newrelic-integration-e2e-action@latest \
     --commit_sha=test-string --retry_attempts=5 --retry_seconds=60 \
	 --account_id=${ACCOUNT_ID} --api_key=${API_KEY} --license_key=${LICENSE_KEY} \
	 --spec_path=./e2e/test-specs.yml --verbose_mode=true --agent_enabled="false"
```

NOTES: 
For local testing purposes, we usually test against a staging environment. In order to enable testing against staging environment, the following modifications need to be made:
- Open the the `/e2e/test-specs.yml` and add `--set global.nrStaging=true` to the end of any occurrences of this line `- helm upgrade --install ${SCENARIO_TAG} -n nr-${SCENARIO_TAG} --create-namespace ../charts/newrelic-infrastructure ...` .
- Add and set `--region="Staging"` the command that executes the tests. For example:
```shell
LICENSE_KEY=${LICENSE_KEY} EXCEPTIONS_SOURCE_FILE=${EXCEPTION_FILE}  go run github.com/newrelic/newrelic-integration-e2e-action@latest \
     --commit_sha=test-string --retry_attempts=5 --retry_seconds=60 \
	 --account_id=${ACCOUNT_ID} --api_key=${API_KEY} --license_key=${LICENSE_KEY} \
	 --spec_path=./e2e/test-specs.yml --verbose_mode=true --agent_enabled="false" --region="Staging"
```   

You may check [e2e workflow](../.github/workflows/e2e.yaml) to have more details about how this is used in development workflow.
