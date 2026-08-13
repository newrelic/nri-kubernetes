package discover

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassify_byLabel(t *testing.T) {
	cases := []struct {
		name   string
		labels map[string]string
		want   WorkloadType
	}{
		{"redis app.kubernetes.io/name", map[string]string{"app.kubernetes.io/name": "redis"}, WorkloadTypeRedis},
		{"kafka strimzi label key", map[string]string{"strimzi.io/kind": "Kafka"}, WorkloadTypeKafka},
		{"kafka app label", map[string]string{"app": "kafka"}, WorkloadTypeKafka},
		{"postgres app label", map[string]string{"app": "postgresql"}, WorkloadTypePostgres},
		{"mysql app label", map[string]string{"app.kubernetes.io/name": "mysql"}, WorkloadTypeMySQL},
		{"mongodb app label", map[string]string{"app.kubernetes.io/name": "mongodb"}, WorkloadTypeMongoDB},
		{"zookeeper app label", map[string]string{"app": "zookeeper"}, WorkloadTypeZookeeper},
		{"elasticsearch app label", map[string]string{"app.kubernetes.io/name": "elasticsearch"}, WorkloadTypeElasticsearch},
		{"rabbitmq app label", map[string]string{"app.kubernetes.io/name": "rabbitmq"}, WorkloadTypeRabbitMQ},
		{"memcached app label", map[string]string{"app.kubernetes.io/name": "memcached"}, WorkloadTypeMemcached},
		{"no match", map[string]string{"app": "frontend"}, WorkloadTypeUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(nil, tc.labels)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestClassify_byImage(t *testing.T) {
	cases := []struct {
		name   string
		images []string
		want   WorkloadType
	}{
		{"plain redis", []string{"redis:7.0.12"}, WorkloadTypeRedis},
		{"bitnami redis", []string{"docker.io/bitnami/redis:latest"}, WorkloadTypeRedis},
		{"bitnami kafka", []string{"docker.io/bitnami/kafka:3.6"}, WorkloadTypeKafka},
		{"confluent kafka", []string{"confluentinc/cp-kafka:7.5.0"}, WorkloadTypeKafka},
		{"postgres", []string{"postgres:15"}, WorkloadTypePostgres},
		{"bitnami postgresql", []string{"bitnami/postgresql:15.3.0"}, WorkloadTypePostgres},
		{"mysql", []string{"mysql:8.0"}, WorkloadTypeMySQL},
		{"mariadb", []string{"mariadb:10.11"}, WorkloadTypeMySQL},
		{"mongo", []string{"mongo:6"}, WorkloadTypeMongoDB},
		{"zookeeper", []string{"zookeeper:3.8"}, WorkloadTypeZookeeper},
		{"cassandra", []string{"cassandra:4.1"}, WorkloadTypeCassandra},
		{"elasticsearch", []string{"docker.elastic.co/elasticsearch/elasticsearch:8.11.0"}, WorkloadTypeElasticsearch},
		{"opensearch", []string{"opensearchproject/opensearch:2.11"}, WorkloadTypeOpenSearch},
		{"rabbitmq", []string{"rabbitmq:3.12-management"}, WorkloadTypeRabbitMQ},
		{"memcached", []string{"memcached:1.6"}, WorkloadTypeMemcached},
		{"unknown app", []string{"my-custom-app:latest"}, WorkloadTypeUnknown},
		{"sidecar alongside redis", []string{"my-app:1.0", "redis:7.0"}, WorkloadTypeRedis},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.images, nil)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestClassify_labelBeatsImage(t *testing.T) {
	// A pod labelled as redis but running a generic image should classify as redis.
	got := Classify([]string{"my-custom-redis-fork:1.0"}, map[string]string{"app.kubernetes.io/name": "redis"})
	assert.Equal(t, WorkloadTypeRedis, got)
}

func TestStripImageMeta(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"redis:7.0.12", "redis"},
		{"docker.io/bitnami/redis:latest", "bitnami/redis"},
		{"gcr.io/google-containers/pause:3.6", "google-containers/pause"},
		{"confluentinc/cp-kafka:7.5.0", "confluentinc/cp-kafka"},
		{"postgres", "postgres"},
		{"localhost/myimage:dev", "myimage"},
		{"registry.k8s.io/kube-apiserver:v1.28.0", "kube-apiserver"},
		{"image@sha256:abc123", "image"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, stripImageMeta(tc.in))
		})
	}
}
