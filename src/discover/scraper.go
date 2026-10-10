package discover

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/newrelic/infra-integrations-sdk/data/attribute"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	sdk "github.com/newrelic/infra-integrations-sdk/integration"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
)

const (
	entityType    = "k8s:discoveredWorkload"
	metricSetType = "K8sDiscoveredWorkloadSample"

	// listTimeout is the per-call timeout for Kubernetes API list operations.
	listTimeout = 30 * time.Second

	kindStatefulSet = "StatefulSet"

	invCatWorkload        = "workload"
	invCatLabels          = "labels"
	invCatStorage         = "storage"
	invCatInstrumentation = "instrumentation"

	metricClusterName = "clusterName"
	metricImage       = "image"
)

// Providers holds the Kubernetes clients needed by the Scraper.
type Providers struct {
	K8s kubernetes.Interface
}

// ScraperOpt is a functional option for Scraper.
type ScraperOpt func(*Scraper) error

// WithLogger sets the logger on the Scraper.
func WithLogger(logger *log.Logger) ScraperOpt {
	return func(s *Scraper) error {
		s.logger = logger
		return nil
	}
}

// WithFilterer sets a namespace filterer on the Scraper.
func WithFilterer(filterer discovery.NamespaceFilterer) ScraperOpt {
	return func(s *Scraper) error {
		s.filterer = filterer
		return nil
	}
}

// Scraper fetches Kubernetes object metadata and emits NR inventory entities
// for infrastructure workloads (databases, message brokers, caches, etc.).
type Scraper struct {
	config   *Config
	k8s      kubernetes.Interface
	filterer discovery.NamespaceFilterer
	logger   *log.Logger
}

// NewScraper constructs a Scraper with the given config, providers, and options.
func NewScraper(cfg *Config, p Providers, opts ...ScraperOpt) (*Scraper, error) {
	s := &Scraper{
		config: cfg,
		k8s:    p.K8s,
		logger: log.StandardLogger(),
	}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("applying scraper option: %w", err)
		}
	}
	return s, nil
}

// discoveredPod groups all metadata needed to emit one NR entity.
type discoveredPod struct {
	pod             corev1.Pod
	workloadType    WorkloadType
	ownerKind       string
	ownerName       string
	isStateful      bool
	pvcNames        []string
	serviceNames    []string
	instrumentation InstrumentationStatus
}

// Run performs a single discovery pass and populates i with inventory entities.
// Only pods that look like infrastructure are emitted (StatefulSet-owned, PVC-backed,
// or matching a known workload classifier).
func (s *Scraper) Run(i *sdk.Integration) error {
	ctx, cancel := context.WithTimeout(context.Background(), listTimeout)
	defer cancel()

	pods, err := s.k8s.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing pods: %w", err)
	}

	statefulSets, err := s.k8s.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing statefulsets: %w", err)
	}

	pvcs, err := s.k8s.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing pvcs: %w", err)
	}

	services, err := s.k8s.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing services: %w", err)
	}

	// DaemonSets (cluster-wide) and ConfigMaps (NR namespace only) drive instrumentation detection.
	daemonSets, err := s.k8s.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing daemonsets: %w", err)
	}

	configMaps, err := s.k8s.CoreV1().ConfigMaps(s.config.NRNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		// Non-fatal: if the NR namespace does not exist yet, OHI detection returns false.
		s.logger.Warnf("listing configmaps in namespace %q: %v — OHI detection disabled", s.config.NRNamespace, err)
		configMaps = &corev1.ConfigMapList{}
	}

	ssIndex := indexStatefulSets(statefulSets.Items)
	pvcIndex := indexPVCsByNamespace(pvcs.Items)
	svcIndex := indexServicesByNamespace(services.Items)
	instrCtx := buildClusterCtx(daemonSets.Items, configMaps.Items)

	for idx := range pods.Items {
		pod := &pods.Items[idx]

		if s.filterer != nil && !s.filterer.IsAllowed(pod.Namespace) {
			continue
		}

		dp := s.classify(pod, ssIndex, pvcIndex, svcIndex, instrCtx)
		if !isInfrastructure(dp) {
			continue
		}

		if err := s.populateEntity(i, dp); err != nil {
			s.logger.Warnf("populating entity for %s/%s: %v", pod.Namespace, pod.Name, err)
		}
	}

	return nil
}

// classify builds a discoveredPod from raw K8s objects.
func (s *Scraper) classify(
	pod *corev1.Pod,
	ssIndex map[string]struct{},
	pvcIndex map[string][]corev1.PersistentVolumeClaim,
	svcIndex map[string][]corev1.Service,
	instrCtx clusterInstrumentationCtx,
) discoveredPod {
	dp := discoveredPod{pod: *pod}

	// Determine the direct owner (Pod is often owned by ReplicaSet/StatefulSet).
	for _, ref := range pod.OwnerReferences {
		if dp.ownerKind == "" {
			dp.ownerKind = ref.Kind
			dp.ownerName = ref.Name
		}
		if ref.Kind == kindStatefulSet {
			dp.ownerKind = kindStatefulSet
			dp.ownerName = ref.Name
			dp.isStateful = true
		}
	}

	// Cross-check against the StatefulSet index to confirm the SS exists.
	if dp.ownerKind == kindStatefulSet {
		key := pod.Namespace + "/" + dp.ownerName
		if _, ok := ssIndex[key]; ok {
			dp.isStateful = true
		}
	}

	// Collect PVCs mounted by this pod via its volume list.
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			dp.pvcNames = append(dp.pvcNames, vol.PersistentVolumeClaim.ClaimName)
		}
	}

	// Also pick up PVCs in the same namespace that carry VolumeClaimTemplates
	// naming convention: <template-name>-<statefulset-name>-<ordinal>.
	// This is a best-effort fallback for pods where the volume is not listed.
	if len(dp.pvcNames) == 0 && dp.isStateful {
		for _, pvc := range pvcIndex[pod.Namespace] {
			for _, ref := range pvc.OwnerReferences {
				if ref.Kind == kindStatefulSet && ref.Name == dp.ownerName {
					dp.pvcNames = append(dp.pvcNames, pvc.Name)
				}
			}
		}
	}

	// Find services whose selector matches this pod's labels.
	for _, svc := range svcIndex[pod.Namespace] {
		if selectorMatchesPod(svc.Spec.Selector, pod.Labels) {
			dp.serviceNames = append(dp.serviceNames, svc.Name)
		}
	}

	dp.workloadType = Classify(containerImages(pod), pod.Labels)
	dp.instrumentation = detectPodInstrumentation(pod, dp.workloadType, instrCtx)

	return dp
}

// isInfrastructure returns true for pods that represent infrastructure workloads.
// We emit entities for pods that are StatefulSet-owned, PVC-backed, or match a
// known classifier. Plain Deployment pods (frontend apps, etc.) are skipped.
func isInfrastructure(dp discoveredPod) bool {
	return dp.isStateful || len(dp.pvcNames) > 0 || dp.workloadType != WorkloadTypeUnknown
}

// populateEntity creates a NR inventory entity and a queryable metric set.
func (s *Scraper) populateEntity(i *sdk.Integration, dp discoveredPod) error {
	pod := dp.pod
	entityID := fmt.Sprintf("%s:%s/%s", s.config.ClusterName, pod.Namespace, pod.Name)

	e, err := i.Entity(entityID, entityType)
	if err != nil {
		return fmt.Errorf("creating entity: %w", err)
	}

	e.AddAttributes(
		attribute.Attr(metricClusterName, s.config.ClusterName),
		attribute.Attr("displayName", pod.Name),
	)

	inv := func(category, key string, val interface{}) {
		if setErr := e.Inventory.SetItem(category, key, val); setErr != nil {
			s.logger.Warnf("inventory %s/%s: %v", category, key, setErr)
		}
	}

	inv(invCatWorkload, "type", string(dp.workloadType))
	inv(invCatWorkload, "namespace", pod.Namespace)
	inv(invCatWorkload, "podName", pod.Name)
	inv(invCatWorkload, "nodeName", pod.Spec.NodeName)
	inv(invCatWorkload, "phase", string(pod.Status.Phase))
	inv(invCatWorkload, "ownerKind", dp.ownerKind)
	inv(invCatWorkload, "ownerName", dp.ownerName)
	inv(invCatWorkload, "isStateful", strconv.FormatBool(dp.isStateful))

	imgs := containerImages(&pod)
	if len(imgs) > 0 {
		inv(invCatWorkload, metricImage, imgs[0])
		if len(imgs) > 1 {
			inv(invCatWorkload, "allImages", strings.Join(imgs, ","))
		}
	}

	if len(dp.pvcNames) > 0 {
		inv(invCatStorage, "pvcs", strings.Join(dp.pvcNames, ","))
	}

	if len(dp.serviceNames) > 0 {
		inv(invCatWorkload, "services", strings.Join(dp.serviceNames, ","))
	}

	for k, v := range pod.Labels {
		inv(invCatLabels, k, v)
	}

	instr := dp.instrumentation
	inv(invCatInstrumentation, "status", instr.Status)
	inv(invCatInstrumentation, "infraAgentDeployed", strconv.FormatBool(instr.InfraAgentDeployed))
	inv(invCatInstrumentation, "ohiConfigured", strconv.FormatBool(instr.OHIConfigured))
	inv(invCatInstrumentation, "apmPresent", strconv.FormatBool(instr.APMPresent))
	inv(invCatInstrumentation, "otelPresent", strconv.FormatBool(instr.OTelPresent))
	inv(invCatInstrumentation, "prometheusScraped", strconv.FormatBool(instr.PrometheusScraped))
	inv(invCatInstrumentation, "nrAnnotated", strconv.FormatBool(instr.NRAnnotated))

	ms := e.NewMetricSet(metricSetType)

	attr := func(name string, val interface{}) {
		if setErr := ms.SetMetric(name, val, metric.ATTRIBUTE); setErr != nil {
			s.logger.Warnf("metric %s: %v", name, setErr)
		}
	}

	attr(metricClusterName, s.config.ClusterName)
	attr("workloadType", string(dp.workloadType))
	attr("namespace", pod.Namespace)
	attr("podName", pod.Name)
	attr("nodeName", pod.Spec.NodeName)
	attr("phase", string(pod.Status.Phase))
	attr("ownerKind", dp.ownerKind)
	attr("ownerName", dp.ownerName)
	attr("isStateful", strconv.FormatBool(dp.isStateful))
	if len(imgs) > 0 {
		attr(metricImage, imgs[0])
	}
	attr("instrumentationStatus", instr.Status)
	attr("infraAgentDeployed", strconv.FormatBool(instr.InfraAgentDeployed))
	attr("ohiConfigured", strconv.FormatBool(instr.OHIConfigured))
	attr("apmPresent", strconv.FormatBool(instr.APMPresent))
	attr("otelPresent", strconv.FormatBool(instr.OTelPresent))

	return nil
}

// indexStatefulSets builds a "namespace/name" presence set.
func indexStatefulSets(sss []appsv1.StatefulSet) map[string]struct{} {
	idx := make(map[string]struct{}, len(sss))
	for i := range sss {
		idx[sss[i].Namespace+"/"+sss[i].Name] = struct{}{}
	}
	return idx
}

// indexPVCsByNamespace groups PVCs by namespace.
func indexPVCsByNamespace(pvcs []corev1.PersistentVolumeClaim) map[string][]corev1.PersistentVolumeClaim {
	idx := make(map[string][]corev1.PersistentVolumeClaim)
	for _, pvc := range pvcs {
		idx[pvc.Namespace] = append(idx[pvc.Namespace], pvc)
	}
	return idx
}

// indexServicesByNamespace groups Services by namespace.
func indexServicesByNamespace(svcs []corev1.Service) map[string][]corev1.Service {
	idx := make(map[string][]corev1.Service)
	for _, svc := range svcs {
		idx[svc.Namespace] = append(idx[svc.Namespace], svc)
	}
	return idx
}

// selectorMatchesPod returns true when all selector key-value pairs are present
// in podLabels (empty selector never matches).
func selectorMatchesPod(selector, podLabels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if podLabels[k] != v {
			return false
		}
	}
	return true
}

// containerImages returns the unique images from all containers in the pod spec.
func containerImages(pod *corev1.Pod) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, c := range pod.Spec.Containers {
		if _, ok := seen[c.Image]; !ok {
			seen[c.Image] = struct{}{}
			out = append(out, c.Image)
		}
	}
	return out
}
