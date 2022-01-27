module github.com/newrelic/nri-kubernetes/tools

go 1.17

require github.com/golangci/golangci-lint v1.41.1

replace (
	// To mitigate CVE-2021-3121.
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
	// To avoid CVE-2018-16886 triggering a security scan.
	go.etcd.io/etcd => go.etcd.io/etcd v0.5.0-alpha.5.0.20190108173120-83c051b701d3
)
