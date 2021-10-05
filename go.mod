module github.com/newrelic/nri-kubernetes/v2

go 1.16

require (
	github.com/golang/protobuf v1.5.2
	github.com/newrelic/infra-integrations-sdk v2.0.1-0.20180410150501-14a5386f9150+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/prom2json v1.3.0
	github.com/segmentio/go-camelcase v0.0.0-20160726192923-7085f1e3c734
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/xeipuuv/gojsonschema v1.2.0
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5
	golang.org/x/oauth2 v0.0.0-20210514164344-f6687ab2804c
	k8s.io/api v0.22.0
	k8s.io/apimachinery v0.22.0
	k8s.io/client-go v0.22.0
	k8s.io/kubelet v0.22.0
)
