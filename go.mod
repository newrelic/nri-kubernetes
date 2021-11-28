module github.com/newrelic/nri-kubernetes/v2

go 1.16

require (
	github.com/google/go-cmp v0.5.6
	github.com/newrelic/infra-integrations-sdk v3.7.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/prom2json v1.3.0
	github.com/segmentio/go-camelcase v0.0.0-20160726192923-7085f1e3c734
	github.com/sethgrid/pester v1.1.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.7.0
	google.golang.org/protobuf v1.27.1
	k8s.io/api v0.22.3
	k8s.io/apimachinery v0.22.3
	k8s.io/client-go v0.22.3
	k8s.io/kubelet v0.22.3
)
