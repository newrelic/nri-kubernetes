module github.com/newrelic/nri-kubernetes/v2

go 1.16

require (
	github.com/google/go-cmp v0.5.6
	github.com/newrelic/infra-integrations-sdk v3.7.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/prom2json v1.3.0
	github.com/segmentio/go-camelcase v0.0.0-20160726192923-7085f1e3c734
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/xeipuuv/gojsonschema v1.2.0
	golang.org/x/net v0.0.0-20211005001312-d4b1ae081e3b
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f
	google.golang.org/protobuf v1.27.1
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/kubelet v0.22.2
)
