module github.com/newrelic/nri-kubernetes

go 1.14

require (
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.5 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/newrelic/infra-integrations-sdk v2.0.1-0.20180410150501-14a5386f9150+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/prom2json v1.1.1-0.20190107153843-47e3ee600b5a
	github.com/segmentio/go-camelcase v0.0.0-20160726192923-7085f1e3c734
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.6.1
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v0.0.0-20180525140004-9ff6d6c47f3f
	golang.org/x/net v0.0.0-20210520170846-37e1c6afe023
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/kubelet v0.21.1
	sigs.k8s.io/structured-merge-diff/v4 v4.1.1 // indirect
)
