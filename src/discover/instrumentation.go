package discover

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// InstrumentationStatus summarises observability coverage for a discovered workload.
// Each field is a distinct signal; Status is the derived summary across all of them.
type InstrumentationStatus struct {
	// Status is "instrumented", "partial", or "not_instrumented".
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

// nrInfraDaemonSetNames are the DaemonSet names used by the various nri-bundle release names.
var nrInfraDaemonSetNames = map[string]struct{}{
	"newrelic-infrastructure":          {},
	"nrk8s-infrastructure":             {},
	"nri-bundle-nrk8s-infrastructure":  {},
	"newrelic-infra":                   {},
}

// ohiConfigMapSubstrings maps each workload type to the nri-* substrings we look for
// in ConfigMap names when deciding whether the OHI is configured.
var ohiConfigMapSubstrings = map[WorkloadType][]string{
	WorkloadTypeRedis:         {"nri-redis"},
	WorkloadTypeKafka:         {"nri-kafka"},
	WorkloadTypeZookeeper:     {"nri-zookeeper", "nri-kafka"}, // ZK is often bundled alongside Kafka
	WorkloadTypePostgres:      {"nri-postgresql", "nri-postgres"},
	WorkloadTypeMySQL:         {"nri-mysql"},
	WorkloadTypeMongoDB:       {"nri-mongodb"},
	WorkloadTypeCassandra:     {"nri-cassandra"},
	WorkloadTypeElasticsearch: {"nri-elasticsearch"},
	WorkloadTypeOpenSearch:    {"nri-elasticsearch", "nri-opensearch"},
	WorkloadTypeRabbitMQ:      {"nri-rabbitmq"},
	WorkloadTypeMemcached:     {"nri-memcached"},
}

// nrAPMEnvVarNames signals a NR APM agent inside the container.
// We check the env var NAME only — never the value — so license key secrets are not read.
var nrAPMEnvVarNames = map[string]struct{}{
	"NEW_RELIC_LICENSE_KEY": {},
	"NEW_RELIC_APP_NAME":    {},
	"NEWRELIC_LICENSE_KEY":  {},
}

// otelEnvVarNames signals OpenTelemetry SDK / auto-instrumentation inside the container.
var otelEnvVarNames = map[string]struct{}{
	"OTEL_SERVICE_NAME":           {},
	"OTEL_EXPORTER_OTLP_ENDPOINT": {},
	"OTEL_RESOURCE_ATTRIBUTES":    {},
}

// otelSidecarImageSubstrings identify an OTel collector running as a sidecar container.
var otelSidecarImageSubstrings = []string{
	"otel/opentelemetry-collector",
	"otelcol",
	"otel-collector",
	"newrelic/newrelic-otel-collector",
}

// buildClusterCtx runs once per scrape cycle. It takes the pre-fetched DaemonSet and
// ConfigMap lists (both already obtained by the scraper) and extracts cluster-level
// instrumentation signals that are the same for every pod.
func buildClusterCtx(daemonSets []appsv1.DaemonSet, configMaps []corev1.ConfigMap) clusterInstrumentationCtx {
	ctx := clusterInstrumentationCtx{
		ohiConfiguredFor: make(map[WorkloadType]bool),
	}

	for _, ds := range daemonSets {
		if _, ok := nrInfraDaemonSetNames[ds.Name]; ok {
			ctx.infraAgentDeployed = true
			break
		}
	}

	// Build a lowercased slice of ConfigMap names for substring matching.
	cmNames := make([]string, 0, len(configMaps))
	for _, cm := range configMaps {
		cmNames = append(cmNames, strings.ToLower(cm.Name))
	}

	for wt, substrings := range ohiConfigMapSubstrings {
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

	// --- Targeted annotation checks (key prefix only, three patterns) ---
	for k, v := range pod.Annotations {
		kLower := strings.ToLower(k)
		switch {
		case kLower == "prometheus.io/scrape" && v == "true":
			s.PrometheusScraped = true
		case strings.HasPrefix(kLower, "newrelic.io/"):
			s.NRAnnotated = true
		case strings.HasPrefix(kLower, "instrumentation.opentelemetry.io/"):
			// OTel Operator auto-instrumentation annotation — treat as OTel signal.
			s.OTelPresent = true
		}
	}

	// --- Container-level env var and image checks ---
	for _, c := range pod.Spec.Containers {
		for _, env := range c.Env {
			if _, ok := nrAPMEnvVarNames[env.Name]; ok {
				s.APMPresent = true
			}
			if _, ok := otelEnvVarNames[env.Name]; ok {
				s.OTelPresent = true
			}
		}
		imgLower := strings.ToLower(c.Image)
		for _, substr := range otelSidecarImageSubstrings {
			if strings.Contains(imgLower, substr) {
				s.OTelPresent = true
				break
			}
		}
	}

	s.Status = deriveStatus(s)
	return s
}

// deriveStatus maps the individual boolean signals to the three-value summary string.
//
//   "instrumented"     — active, specific telemetry collection confirmed for this workload.
//   "partial"          — some monitoring infrastructure is present but the specific workload
//                        OHI is not configured, or only generic scraping is set up.
//   "not_instrumented" — no monitoring signals detected.
func deriveStatus(s InstrumentationStatus) string {
	// "instrumented": we have a complete picture — either the NR OHI is wired up for
	// this specific workload type, or an APM / OTel agent is inside the process.
	if (s.InfraAgentDeployed && s.OHIConfigured) || s.APMPresent || s.OTelPresent {
		return "instrumented"
	}

	// "partial": the NR infra agent or Prometheus is present at the cluster level,
	// but it is not specifically configured for this workload type.
	if s.InfraAgentDeployed || s.PrometheusScraped || s.NRAnnotated {
		return "partial"
	}

	return "not_instrumented"
}
