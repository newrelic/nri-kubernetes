# E2E tests

In order to run it locally you can do the following
```shell
eval $(minikube -p minikube docker-env)
```

Then you need to build the binary and the image. Notice that  since the Dockerfile includes multiarch
support, you may need to set `DOCKER_BUILDKIT=1` when running `docker build` for the `TARGETARCH`
and `TARGETOS` args to be populated.
```shell
GOOS=linux GOARCH=amd64 make compile
docker build -t test_image_normal:test  .
```

Then you can run manually any scenario you are interested into
```shell
helm dependencies update ./e2e/charts/newrelic-infrastructure-k8s-e2e
go run e2e/cmd/e2e.go --verbose --cluster_name=e2e --nr_license_key="fakeLicense" --rbac=true --integration_image_tag=test --integration_image_repository=test_image_normal
```

Note: On macOS you might have to do run `minikube start --vm` to get a proper IP that will be used by the e2e test to check the cluster flavor.
