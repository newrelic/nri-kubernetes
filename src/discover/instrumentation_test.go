package discover

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// --- buildClusterCtx ---

func TestBuildClusterCtx_infraAgentDetected(t *testing.T) {
	dss := []appsv1.DaemonSet{
		{ObjectMeta: metav1.ObjectMeta{Name: "newrelic-infrastructure"}},
	}
	ctx := buildClusterCtx(dss, nil)
	assert.True(t, ctx.infraAgentDeployed)
}

func TestBuildClusterCtx_infraAgentBundleName(t *testing.T) {
	dss := []appsv1.DaemonSet{
		{ObjectMeta: metav1.ObjectMeta{Name: "nri-bundle-nrk8s-infrastructure"}},
	}
	ctx := buildClusterCtx(dss, nil)
	assert.True(t, ctx.infraAgentDeployed)
}

func TestBuildClusterCtx_noInfraAgent(t *testing.T) {
	dss := []appsv1.DaemonSet{
		{ObjectMeta: metav1.ObjectMeta{Name: "fluentd"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "kube-proxy"}},
	}
	ctx := buildClusterCtx(dss, nil)
	assert.False(t, ctx.infraAgentDeployed)
}

func TestBuildClusterCtx_ohiConfigMapDetected(t *testing.T) {
	cms := []corev1.ConfigMap{
		{ObjectMeta: metav1.ObjectMeta{Name: "nri-bundle-nri-redis"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "nri-postgresql-config"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "my-app-config"}},
	}
	ctx := buildClusterCtx(nil, cms)
	assert.True(t, ctx.ohiConfiguredFor[WorkloadTypeRedis], "redis OHI should be detected")
	assert.True(t, ctx.ohiConfiguredFor[WorkloadTypePostgres], "postgres OHI should be detected")
	assert.False(t, ctx.ohiConfiguredFor[WorkloadTypeKafka], "kafka OHI should not be detected")
}

func TestBuildClusterCtx_zookeeperDetectedViaKafkaConfigMap(t *testing.T) {
	cms := []corev1.ConfigMap{
		{ObjectMeta: metav1.ObjectMeta{Name: "nri-kafka-config"}},
	}
	ctx := buildClusterCtx(nil, cms)
	assert.True(t, ctx.ohiConfiguredFor[WorkloadTypeZookeeper], "zookeeper OHI should be inferred from kafka configmap")
}

// --- detectPodInstrumentation ---

func pod(annotations map[string]string, envVars []corev1.EnvVar, images ...string) *corev1.Pod {
	containers := make([]corev1.Container, len(images))
	for i, img := range images {
		containers[i] = corev1.Container{Image: img, Env: envVars}
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Annotations: annotations},
		Spec:       corev1.PodSpec{Containers: containers},
	}
}

func emptyCtx() clusterInstrumentationCtx {
	return clusterInstrumentationCtx{ohiConfiguredFor: map[WorkloadType]bool{}}
}

func TestDetectPodInstrumentation_nrAPMEnvVar(t *testing.T) {
	p := pod(nil, []corev1.EnvVar{{Name: "NEW_RELIC_LICENSE_KEY"}}, "redis:7")
	s := detectPodInstrumentation(p, WorkloadTypeRedis, emptyCtx())
	assert.True(t, s.APMPresent)
	assert.Equal(t, "instrumented", s.Status)
}

func TestDetectPodInstrumentation_otelEnvVar(t *testing.T) {
	p := pod(nil, []corev1.EnvVar{{Name: "OTEL_SERVICE_NAME", Value: "redis-svc"}}, "redis:7")
	s := detectPodInstrumentation(p, WorkloadTypeRedis, emptyCtx())
	assert.True(t, s.OTelPresent)
	assert.Equal(t, "instrumented", s.Status)
}

func TestDetectPodInstrumentation_otelSidecar(t *testing.T) {
	p := pod(nil, nil, "redis:7", "otel/opentelemetry-collector:0.90.0")
	s := detectPodInstrumentation(p, WorkloadTypeRedis, emptyCtx())
	assert.True(t, s.OTelPresent)
	assert.Equal(t, "instrumented", s.Status)
}

func TestDetectPodInstrumentation_otelOperatorAnnotation(t *testing.T) {
	p := pod(map[string]string{
		"instrumentation.opentelemetry.io/inject-java": "true",
	}, nil, "redis:7")
	s := detectPodInstrumentation(p, WorkloadTypeRedis, emptyCtx())
	assert.True(t, s.OTelPresent)
	assert.Equal(t, "instrumented", s.Status)
}

func TestDetectPodInstrumentation_infraAgentAndOHI(t *testing.T) {
	ctx := clusterInstrumentationCtx{
		infraAgentDeployed: true,
		ohiConfiguredFor:   map[WorkloadType]bool{WorkloadTypeRedis: true},
	}
	p := pod(nil, nil, "redis:7")
	s := detectPodInstrumentation(p, WorkloadTypeRedis, ctx)
	assert.True(t, s.InfraAgentDeployed)
	assert.True(t, s.OHIConfigured)
	assert.Equal(t, "instrumented", s.Status)
}

func TestDetectPodInstrumentation_infraAgentWithoutOHI(t *testing.T) {
	ctx := clusterInstrumentationCtx{
		infraAgentDeployed: true,
		ohiConfiguredFor:   map[WorkloadType]bool{},
	}
	p := pod(nil, nil, "redis:7")
	s := detectPodInstrumentation(p, WorkloadTypeRedis, ctx)
	assert.True(t, s.InfraAgentDeployed)
	assert.False(t, s.OHIConfigured)
	assert.Equal(t, "partial", s.Status)
}

func TestDetectPodInstrumentation_prometheusScraped(t *testing.T) {
	p := pod(map[string]string{"prometheus.io/scrape": "true"}, nil, "redis:7")
	s := detectPodInstrumentation(p, WorkloadTypeRedis, emptyCtx())
	assert.True(t, s.PrometheusScraped)
	assert.Equal(t, "partial", s.Status)
}

func TestDetectPodInstrumentation_nrAnnotated(t *testing.T) {
	p := pod(map[string]string{"newrelic.io/integrations-src": "redis"}, nil, "redis:7")
	s := detectPodInstrumentation(p, WorkloadTypeRedis, emptyCtx())
	assert.True(t, s.NRAnnotated)
	assert.Equal(t, "partial", s.Status)
}

func TestDetectPodInstrumentation_noSignals(t *testing.T) {
	p := pod(nil, nil, "redis:7")
	s := detectPodInstrumentation(p, WorkloadTypeRedis, emptyCtx())
	assert.Equal(t, "not_instrumented", s.Status)
	assert.False(t, s.APMPresent)
	assert.False(t, s.OTelPresent)
	assert.False(t, s.InfraAgentDeployed)
	assert.False(t, s.PrometheusScraped)
}

func TestDetectPodInstrumentation_envVarNameCheckedNotValue(t *testing.T) {
	// Env var references a Secret — Value is empty, ValueFrom is set.
	// We only check Name, so this still counts as APM present.
	p := pod(nil, []corev1.EnvVar{
		{
			Name: "NEW_RELIC_LICENSE_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{Key: "licenseKey"},
			},
		},
	}, "redis:7")
	s := detectPodInstrumentation(p, WorkloadTypeRedis, emptyCtx())
	assert.True(t, s.APMPresent, "env var from secretRef should still be detected by name")
}
