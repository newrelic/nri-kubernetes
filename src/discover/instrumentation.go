package discover

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// Instrumentation status values returned in InstrumentationStatus.Status.
const (
	StatusInstrumented    = "instrumented"
	StatusPartial         = "partial"
	StatusNotInstrumented = "not_instrumented"
)

// NR APM env var names — checked by name only, value is never read.
const (
	envNRLicenseKey    = "NEW_RELIC_LICENSE_KEY"
	envNRLicenseKeyAlt = "NEWRELIC_LICENSE_KEY"
	envNRAppName       = "NEW_RELIC_APP_NAME"
)

// OTel SDK / auto-instrumentation env var names.
const (
	envOTelServiceName  = "OTEL_SERVICE_NAME"
	envOTelEndpoint     = "OTEL_EXPORTER_OTLP_ENDPOINT"
	envOTelResourceAttr = "OTEL_RESOURCE_ATTRIBUTES"
)

// InstrumentationStatus summarises observability coverage for a discovered workload.
// Each field is a distinct signal; Status is the derived summary across all of them.
type InstrumentationStatus struct {
	// Status is one of StatusInstrumented, StatusPartial, or StatusNotInstrumented.
	Status string

	// InfraAgentDeployed is true when a NR infrastructure DaemonSet is running on the cluster.
	// Necessary but not sufficient: the specific OHI for this workload type may not be configured.
	InfraAgentDeployed bool

	// OHIConfigured is true when a ConfigMap whose name contains the expected nri-<type>
	// pattern exists in the NR namespace, indicating the on-host integration is wired up.
	OHIConfigured bool

	// APMPresent is true when any container in the pod spec declares a NEW_RELIC_LICENSE_KEY
	// or NEW_RELIC_APP_NAME env var (by name only — value is never read).
	APMPresent bool

	// OTelPresent is true when any container declares OTEL_SERVICE_NAME /
	// OTEL_EXPORTER_OTLP_ENDPOINT env vars, has an OTel collector sidecar image, or
	// carries an instrumentation.opentelemetry.io/* annotation (OTel Operator).
	OTelPresent bool

	// PrometheusScraped is true when the pod carries prometheus.io/scrape: "true".
	PrometheusScraped bool

	// NRAnnotated is true when any newrelic.io/* annotation key is present on the pod,
	// indicating that NR K8s auto-discovery is configured for this pod.
	NRAnnotated bool
}

// clusterInstrumentationCtx is computed once per Run() cycle and reused for every pod.
type clusterInstrumentationCtx struct {
	infraAgentDeployed bool
	ohiConfiguredFor   map[WorkloadType]bool
}

// nrInfraDaemonSetNames returns the set of DaemonSet names used by nri-bundle releases.
// Returned as a function to avoid gochecknoglobals.
func nrInfraDaemonSetNames() map[string]struct{} {
	return map[string]struct{}{
		"newrelic-infrastructure":         {},
		"nrk8s-infrastructure":            {},
		"nri-bundle-nrk8s-infrastructure": {},
		"newrelic-infra":                  {},
	}
}

// ohiConfigMapSubstrings returns the nri-* substrings to look for in ConfigMap names
// when deciding whether the OHI for a given workload type is configured.
// Returned as a function to avoid gochecknoglobals.
func ohiConfigMapSubstrings() map[WorkloadType][]string {
	return map[WorkloadType][]string{
		WorkloadTypeRedis:         {"nri-redis"},
		WorkloadTypeKafka:         {"nri-kafka"},
		WorkloadTypeZookeeper:     {"nri-zookeeper", "nri-kafka"}, // ZK is often bundled alongside Kafka.
		WorkloadTypePostgres:      {"nri-postgresql", "nri-postgres"},
		WorkloadTypeMySQL:         {"nri-mysql"},
		WorkloadTypeMongoDB:       {"nri-mongodb"},
		WorkloadTypeCassandra:     {"nri-cassandra"},
		WorkloadTypeElasticsearch: {"nri-elasticsearch"},
		WorkloadTypeOpenSearch:    {"nri-elasticsearch", "nri-opensearch"},
		WorkloadTypeRabbitMQ:      {"nri-rabbitmq"},
		WorkloadTypeMemcached:     {"nri-memcached"},
	}
}

// nrAPMEnvVarNames returns env var names whose presence signals a NR APM agent.
// We check the env var NAME only — never the value — so license key secrets are not read.
// Returned as a function to avoid gochecknoglobals.
func nrAPMEnvVarNames() map[string]struct{} {
	return map[string]struct{}{
		envNRLicenseKey:    {},
		envNRAppName:       {},
		envNRLicenseKeyAlt: {},
	}
}

// otelEnvVarNames returns env var names whose presence signals OTel instrumentation.
// Returned as a function to avoid gochecknoglobals.
func otelEnvVarNames() map[string]struct{} {
	return map[string]struct{}{
		envOTelServiceName:  {},
		envOTelEndpoint:     {},
		envOTelResourceAttr: {},
	}
}

// otelSidecarImageSubstrings returns image substrings that identify an OTel collector sidecar.
// Returned as a function to avoid gochecknoglobals.
func otelSidecarImageSubstrings() []string {
	return []string{
		"otel/opentelemetry-collector",
		"otelcol",
		"otel-collector",
		"newrelic/newrelic-otel-collector",
	}
}

// buildClusterCtx runs once per scrape cycle. It takes the pre-fetched DaemonSet and
// ConfigMap lists (both already obtained by the scraper) and extracts cluster-level
// instrumentation signals that are the same for every pod.
func buildClusterCtx(daemonSets []appsv1.DaemonSet, configMaps []corev1.ConfigMap) clusterInstrumentationCtx {
	ctx := clusterInstrumentationCtx{
		ohiConfiguredFor: make(map[WorkloadType]bool),
	}

	dsNames := nrInfraDaemonSetNames()
	for _, ds := range daemonSets {
		if _, ok := dsNames[ds.Name]; ok {
			ctx.infraAgentDeployed = true
			break
		}
	}

	// Build a lowercased slice of ConfigMap names for substring matching.
	cmNames := make([]string, 0, len(configMaps))
	for _, cm := range configMaps {
		cmNames = append(cmNames, strings.ToLower(cm.Name))
	}

	for wt, substrings := range ohiConfigMapSubstrings() {
		for _, name := range cmNames {
			for _, substr := range substrings {
				if strings.Contains(name, substr) {
					ctx.ohiConfiguredFor[wt] = true
				}
			}
		}
	}

	return ctx
}

// detectPodInstrumentation derives InstrumentationStatus for a single pod.
// Annotation checks are intentionally limited to three well-known key prefixes;
// we do not collect or store arbitrary annotation values.
func detectPodInstrumentation(pod *corev1.Pod, wt WorkloadType, ctx clusterInstrumentationCtx) InstrumentationStatus {
	s := InstrumentationStatus{
		InfraAgentDeployed: ctx.infraAgentDeployed,
		OHIConfigured:      ctx.ohiConfiguredFor[wt],
	}

	// 1. Check annotations
	checkAnnotations(pod.Annotations, &s)

	// 2. Check container specs (Env vars and Sidecars)
	checkContainers(pod.Spec.Containers, &s)

	s.Status = deriveStatus(s)
	return s
}

// Helper 1: Evaluates annotations for scraping and telemetry signals
func checkAnnotations(annotations map[string]string, s *InstrumentationStatus) {
	for k, v := range annotations {
		kLower := strings.ToLower(k)
		switch {
		case kLower == "prometheus.io/scrape" && v == "true":
			s.PrometheusScraped = true
		case strings.HasPrefix(kLower, "newrelic.io/"):
			s.NRAnnotated = true
		case strings.HasPrefix(kLower, "instrumentation.opentelemetry.io/"):
			s.OTelPresent = true
		}
	}
}

// Helper 2: Loops through containers to inspect environments and images
func checkContainers(containers []corev1.Container, s *InstrumentationStatus) {
	apmEnvVars := nrAPMEnvVarNames()
	otelEnvVars := otelEnvVarNames()
	otelSidecars := otelSidecarImageSubstrings()
	for _, c := range containers {
		for _, env := range c.Env {
			if _, ok := apmEnvVars[env.Name]; ok {
				s.APMPresent = true
			}
			if _, ok := otelEnvVars[env.Name]; ok {
				s.OTelPresent = true
			}
		}
		imgLower := strings.ToLower(c.Image)
		for _, substr := range otelSidecars {
			if strings.Contains(imgLower, substr) {
				s.OTelPresent = true
				break
			}
		}
	}
}

// deriveStatus maps the individual boolean signals to the three-value summary string.
//
// StatusInstrumented     — active, specific telemetry collection confirmed for this workload.
// StatusPartial          — monitoring infrastructure is present but the specific workload
// OHI is not configured, or only generic scraping is set up.
// StatusNotInstrumented  — no monitoring signals detected.
func deriveStatus(s InstrumentationStatus) string {
	if (s.InfraAgentDeployed && s.OHIConfigured) || s.APMPresent || s.OTelPresent {
		return StatusInstrumented
	}

	if s.InfraAgentDeployed || s.PrometheusScraped || s.NRAnnotated {
		return StatusPartial
	}

	return StatusNotInstrumented
}
