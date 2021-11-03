# E2E tests

In order to run it locally you can do the following
```shell
eval $(minikube -p minikube docker-env)
```

Note that the control plane flags in `e2e-values.yml` have been set meeting the minikube specifications. 

Then you need to build the binary and the image. Notice that  since the Dockerfile includes multiarch
support, you may need to set `DOCKER_BUILDKIT=1` when running `docker build` for the `TARGETARCH`
and `TARGETOS` args to be populated.
```shell
GOOS=linux GOARCH=amd64 make compile # Set GOOS and GOARCH explicitly since Dockerfile expects them in the binary name
export  DOCKER_BUILDKIT=1
docker build -t test_image:test --build-arg 'MODE=normal' .
docker build -t test_image:test-unprivileged --build-arg 'MODE=unprivileged' .
```

You need to install the binary `https://github.com/newrelic/newrelic-integration-e2e-action/tree/main/newrelic-integration-e2e` used in the e2e test
```shell
git clone https://github.com/newrelic/newrelic-integration-e2e-action
cd newrelic-integration-e2e
go build -o  $GOPATH/newrelic-integration-e2e
```

You can now run the e2e tests locally
```shell
LICENSE_KEY=${LICENSE_KEY} newrelic-integration-e2e --commit_sha=test-string --retry_attempts=5 --retry_seconds=60 \
	 --account_id=${ACCOUNT_ID} --api_key=${API_KEY} --license_key=${LICENSE_KEY} \
	 --spec_path=./e2e/test-specs.yml --verbose_mode=true --agent_enabled="false"
```
