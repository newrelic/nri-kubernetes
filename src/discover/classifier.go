package discover

import (
	"strings"
)

// WorkloadType is an inferred infrastructure workload category.
type WorkloadType string

const (
	WorkloadTypeRedis         WorkloadType = "redis"
	WorkloadTypeKafka         WorkloadType = "kafka"
	WorkloadTypeZookeeper     WorkloadType = "zookeeper"
	WorkloadTypePostgres      WorkloadType = "postgres"
	WorkloadTypeMySQL         WorkloadType = "mysql"
	WorkloadTypeMongoDB       WorkloadType = "mongodb"
	WorkloadTypeCassandra     WorkloadType = "cassandra"
	WorkloadTypeElasticsearch WorkloadType = "elasticsearch"
	WorkloadTypeOpenSearch    WorkloadType = "opensearch"
	WorkloadTypeRabbitMQ      WorkloadType = "rabbitmq"
	WorkloadTypeMemcached     WorkloadType = "memcached"
	WorkloadTypeUnknown       WorkloadType = "unknown"
)

// workloadSignal defines how to recognise a workload type from pod metadata.
type workloadSignal struct {
	workloadType WorkloadType
	// labelValues is checked against the value of well-known label keys.
	// Matching any entry is sufficient (OR semantics).
	labelValues []string
	// labelKeyExact requires an exact label key to be present with any value.
	labelKeyExact []string
	// imageSubstrings is checked against each container image after stripping
	// the registry host and image tag. Matching any entry is sufficient.
	imageSubstrings []string
}

// workloadSignals returns the signal table ordered most-specific first.
// Returned as a function (not a package-level var) to avoid gochecknoglobals.
func workloadSignals() []workloadSignal {
	return []workloadSignal{
	{
		workloadType:    WorkloadTypeRedis,
		labelValues:     []string{"redis"},
		imageSubstrings: []string{"redis"},
	},
	{
		workloadType:    WorkloadTypeKafka,
		labelValues:     []string{"kafka"},
		labelKeyExact:   []string{"strimzi.io/kind"},
		imageSubstrings: []string{"cp-kafka", "bitnami/kafka", "strimzi/kafka", "apache/kafka"},
	},
	{
		workloadType:    WorkloadTypeZookeeper,
		labelValues:     []string{"zookeeper"},
		imageSubstrings: []string{"zookeeper", "cp-zookeeper", "bitnami/zookeeper"},
	},
	{
		workloadType:    WorkloadTypePostgres,
		labelValues:     []string{"postgresql", "postgres"},
		imageSubstrings: []string{"postgres", "bitnami/postgresql", "timescaledb", "crunchy-postgres"},
	},
	{
		workloadType:    WorkloadTypeMySQL,
		labelValues:     []string{"mysql", "mariadb"},
		imageSubstrings: []string{"mysql", "bitnami/mysql", "mariadb", "bitnami/mariadb", "percona/percona-xtradb"},
	},
	{
		workloadType:    WorkloadTypeMongoDB,
		labelValues:     []string{"mongodb"},
		imageSubstrings: []string{"mongo", "bitnami/mongodb", "percona/percona-server-mongodb"},
	},
	{
		workloadType:    WorkloadTypeCassandra,
		labelValues:     []string{"cassandra"},
		imageSubstrings: []string{"cassandra", "bitnami/cassandra", "scylladb/scylla"},
	},
	{
		workloadType:    WorkloadTypeElasticsearch,
		labelValues:     []string{"elasticsearch"},
		imageSubstrings: []string{"elasticsearch", "bitnami/elasticsearch", "elastic/elasticsearch"},
	},
	{
		workloadType:    WorkloadTypeOpenSearch,
		labelValues:     []string{"opensearch"},
		imageSubstrings: []string{"opensearch", "opensearchproject/opensearch"},
	},
	{
		workloadType:    WorkloadTypeRabbitMQ,
		labelValues:     []string{"rabbitmq"},
		imageSubstrings: []string{"rabbitmq", "bitnami/rabbitmq"},
	},
	{
		workloadType:    WorkloadTypeMemcached,
		labelValues:     []string{"memcached"},
		imageSubstrings: []string{"memcached", "bitnami/memcached"},
	},
	}
}

// labelDiscoveryKeys returns the label keys whose values carry workload identity,
// in priority order. Returned as a function to avoid gochecknoglobals.
func labelDiscoveryKeys() []string {
	return []string{
		"app.kubernetes.io/name",
		"app.kubernetes.io/component",
		"app",
		"helm.sh/chart", // Helm chart names often embed the workload type
	}
}

// Classify infers the workload type from pod labels and container images.
// Labels are checked first (higher confidence), then images.
// Returns WorkloadTypeUnknown if no signal matches.
func Classify(images []string, podLabels map[string]string) WorkloadType {
	// 1a. Check for operator-specific label keys (e.g. strimzi.io/kind).
	//     These are definitive signals regardless of their value.
	sigs := workloadSignals()
	for _, sig := range sigs {
		for _, k := range sig.labelKeyExact {
			if _, has := podLabels[k]; has {
				return sig.workloadType
			}
		}
	}

	// 1b. Check well-known label keys whose values identify the workload.
	for _, key := range labelDiscoveryKeys() {
		val, ok := podLabels[key]
		if !ok || val == "" {
			continue
		}
		valLower := strings.ToLower(val)
		for _, sig := range sigs {
			for _, lv := range sig.labelValues {
				if strings.Contains(valLower, lv) {
					return sig.workloadType
				}
			}
		}
	}

	// 2. Check container images (strip registry host and tag first).
	for _, image := range images {
		bare := stripImageMeta(image)
		for _, sig := range sigs {
			for _, substr := range sig.imageSubstrings {
				if strings.Contains(bare, substr) {
					return sig.workloadType
				}
			}
		}
	}

	return WorkloadTypeUnknown
}

// stripImageMeta removes the registry host and tag from an image reference so
// that "docker.io/bitnami/redis:7.0.12" becomes "bitnami/redis".
func stripImageMeta(image string) string {
	// Remove tag / digest
	if idx := strings.LastIndex(image, ":"); idx > 0 {
		// Ensure the colon is not part of a port in the registry host
		afterColon := image[idx+1:]
		if !strings.Contains(afterColon, "/") {
			image = image[:idx]
		}
	}
	if idx := strings.Index(image, "@"); idx > 0 {
		image = image[:idx]
	}

	// Strip registry host: a registry host contains a dot or port, and is the
	// first path segment. If the first segment contains "." or ":", drop it.
	parts := strings.SplitN(image, "/", 3)
	if len(parts) > 1 && (strings.ContainsAny(parts[0], ".:") || parts[0] == "localhost") {
		image = strings.Join(parts[1:], "/")
	}

	return strings.ToLower(image)
}
