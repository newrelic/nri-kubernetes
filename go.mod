module github.com/newrelic/nri-kubernetes/v2

go 1.16

require (
	github.com/golang/protobuf v1.5.2
	github.com/golangci/golangci-lint v1.41.1
	github.com/newrelic/infra-integrations-sdk v3.6.8+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/prom2json v1.3.0
	github.com/segmentio/go-camelcase v0.0.0-20160726192923-7085f1e3c734
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/xeipuuv/gojsonschema v1.2.0
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e
	golang.org/x/oauth2 v0.0.0-20210628180205-a41e5a781914
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/kubelet v0.21.2
)
