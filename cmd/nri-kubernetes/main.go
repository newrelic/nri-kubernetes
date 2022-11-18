package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	sdk "github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v3/src/client"
	"github.com/newrelic/nri-kubernetes/v3/src/controlplane"
	"github.com/newrelic/nri-kubernetes/v3/src/ksm"
	ksmClient "github.com/newrelic/nri-kubernetes/v3/src/ksm/client"
	"github.com/newrelic/nri-kubernetes/v3/src/kubelet"
	kubeletClient "github.com/newrelic/nri-kubernetes/v3/src/kubelet/client"
	"github.com/newrelic/nri-kubernetes/v3/src/prometheus"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	integrationName = "com.newrelic.kubernetes"

	_ = iota
	exitClients
	exitConfig
	exitIntegration
	exitLoop
	exitSetup
)

var (
	integrationVersion = "0.0.0"
	gitCommit          = ""
	buildDate          = ""
)

var logger *log.Logger

type clusterClients struct {
	k8s      kubernetes.Interface
	ksm      prometheus.MetricFamiliesGetFunc
	cAdvisor prometheus.MetricFamiliesGetFunc
	kubelet  client.HTTPGetter
}

func main() {
	logger = log.StandardLogger()

	c, err := config.LoadConfig(config.DefaultConfigFolderName, config.DefaultConfigFileName)
	if err != nil {
		log.Error(err.Error())
		os.Exit(exitIntegration)
	}

	if c.Verbose {
		logger.SetLevel(log.DebugLevel)
	}

	if c.LogLevel != "" {
		level, err := log.ParseLevel(c.LogLevel)
		if err != nil {
			log.Warnf("Cannot parse log level %q: %v", c.LogLevel, err)
		} else {
			logger.SetLevel(level)
		}
	}

	/*
		integrationOptions := []integration.OptionFunc{
			integration.WithLogger(logger),
			integration.WithMetadata(integration.Metadata{
				Name:    integrationName,
				Version: integrationVersion,
			}),
		}

		switch c.Sink.Type {
		case config.SinkTypeHTTP:
			integrationOptions = append(integrationOptions, integration.WithHTTPSink(c.Sink.HTTP))
		case config.SinkTypeStdout:
			// We don't need to do anything here to sink to stdout, as it's the default behavior of integration.Wrapper.
			logger.Warn("Sinking metrics to stdout")
		default:
			log.Errorf("Unknown sink type %s", c.Sink.Type)
			os.Exit(exitConfig)
		}

		iw, err := integration.NewWrapper(integrationOptions...)
		if err != nil {
			logger.Errorf("creating integration wrapper: %v", err)
			os.Exit(exitIntegration)
		}

		i, err := iw.Integration()
		if err != nil {
			logger.Errorf("creating integration with http sink: %v", err)
			os.Exit(exitIntegration)
		}
	*/

	logger.Infof(
		"New Relic %s integration Version: %s, Platform: %s, GoVersion: %s, GitCommit: %s, BuildDate: %s\n",
		strings.Title(strings.Replace(integrationName, "com.newrelic.", "", 1)),
		integrationVersion,
		fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		runtime.Version(),
		gitCommit,
		buildDate)

	clients, err := buildClients(c)
	if err != nil {
		logger.Errorf("building clients: %v", err)
		os.Exit(exitClients)
	}

	namespaceCache := discovery.NewNamespaceInMemoryStore(logger)

	/*
		var kubeletScraper *kubelet.Scraper
		if c.Kubelet.Enabled {
			kubeletScraper, err = setupKubelet(c, clients, namespaceCache)
			if err != nil {
				logger.Errorf("setting up ksm scraper: %v", err)
				os.Exit(exitSetup)
			}
		}
	*/

	var ksmScraper *ksm.Scraper
	if c.KSM.Enabled {
		ksmScraper, err = setupKSM(c, clients, namespaceCache)
		if err != nil {
			logger.Errorf("setting up ksm scraper: %v", err)
			os.Exit(exitSetup)
		}
		defer ksmScraper.Close()
	}

	var controlplaneScraper *controlplane.Scraper
	if c.ControlPlane.Enabled {
		controlplaneScraper, err = setupControlPlane(c, clients)
		if err != nil {
			logger.Errorf("setting up control plane scraper: %v", err)
			os.Exit(exitSetup)
		}
		defer controlplaneScraper.Close()
	}

	var o *operator
	if c.Operator.Enabled {
		o = createOperator(clients.k8s)
		o.log = logger
	}

	for {
		start := time.Now()

		logger.Debugf("scraping data from all the scrapers defined: KSM: %t, Kubelet: %t, ControlPlane: %t",
			c.KSM.Enabled, c.Kubelet.Enabled, c.ControlPlane.Enabled)

		/*
			// TODO think carefully to the signature of this function
			err := runScrapers(c, ksmScraper, kubeletScraper, controlplaneScraper, i)
			if err != nil {
				logger.Errorf("retrieving scraper data: %v", err)
				os.Exit(exitLoop)
			}
		*/

		logger.Debugf("publishing data")
		/*
			err = i.Publish()
			if err != nil {
				logger.Errorf("publishing integration: %v", err)
				os.Exit(exitLoop)
			}
		*/
		if c.Operator.Enabled {
			o.run()
			o.log = logger
		}

		namespaceCache.Vacuum()

		logger.Debugf("waiting %f seconds for next interval", c.Interval.Seconds())

		// Sleep interval minus the time that took to scrape.
		time.Sleep(c.Interval - time.Since(start))
	}
}

func runScrapers(c *config.Config, ksmScraper *ksm.Scraper, kubeletScraper *kubelet.Scraper, controlplaneScraper *controlplane.Scraper, i *sdk.Integration) error {
	if c.KSM.Enabled {
		err := ksmScraper.Run(i)
		if err != nil {
			return fmt.Errorf("retrieving ksm data: %w", err)
		}
	}

	if c.Kubelet.Enabled {
		err := kubeletScraper.Run(i)
		if err != nil {
			return fmt.Errorf("retrieving kubelet data: %w", err)
		}
	}

	if c.ControlPlane.Enabled {
		err := controlplaneScraper.Run(i)
		if err != nil {
			return fmt.Errorf("retrieving control plane data: %w", err)
		}
	}

	return nil
}

type operator struct {
	lister v1.PodNamespaceLister
	client kubernetes.Interface
	log    *log.Logger
}

const operatorNamespace = "redis"

func createOperator(client kubernetes.Interface) *operator {
	listMap, _ := discovery.NewNamespacePodListerer(discovery.PodListererConfig{Client: client, Namespaces: []string{operatorNamespace}})

	l, _ := listMap.Lister(operatorNamespace)

	return &operator{
		lister: l,
		client: client,
	}
}

func (o operator) run() {
	logger.Debugf("running operator")

	listWorkLoads, listIntegrations := o.listInterestingPods()

	logger.Debugf("CREATING INTEGRATIONS")

	for _, w := range listWorkLoads {
		found := false
		for _, i := range listIntegrations {
			if strings.Contains(i.Name, w.Name) {
				logger.Debugf("workload already monitored %s by %s", w.Name, i.Name)

				found = true
			} else {
				logger.Debugf("%q does not contain %q", i.Name, w.Name)
			}
		}

		if w.Status.PodIP == "" {
			logger.Warnf("SKIPPIN FOR NOW %q since IP is empty", w.Name)
		}

		if !found && w.Status.PodIP != "" {
			o.deployIntegration(w)
		}
	}

	logger.Debugf("CLEANING INTEGRATIONS")

	for _, i := range listIntegrations {
		found := false
		for _, w := range listWorkLoads {
			if strings.Contains(i.Name, w.Name) { //TODO this is an example, we should also check that config did not change
				logger.Debugf("integration monitoring workload %s by %s", w.Name, i.Name)

				found = true
			}
		}

		if !found {
			o.deleteIntegration(i)
		}

	}
}

func (o operator) listInterestingPods() ([]*corev1.Pod, []*corev1.Pod) {
	listWorkLoads, err := o.lister.List(
		labels.SelectorFromSet(labels.Set{
			"monitoring-role": "workload-to-monitor",
		}))
	if err != nil {
		o.log.Errorf("listing workloads %v", err)
	}

	logger.Debugf("found pods %d", len(listWorkLoads))

	listIntegrations, err := o.lister.List(
		labels.SelectorFromSet(labels.Set{
			"monitoring-role": "integration-monitoring-workload",
		}))
	if err != nil {
		o.log.Errorf("listing integrations %v", err)
	}

	logger.Debugf("found integrations %d", len(listIntegrations))

	return listWorkLoads, listIntegrations
}

func (o operator) deleteIntegration(pod *corev1.Pod) {
	logger.Debugf("deleting integrations and secret %q", pod.Name)
	err := o.client.CoreV1().Pods(operatorNamespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
	if err != nil {
		o.log.Errorf("deleting integration %v", err)
	}
	err = o.client.CoreV1().Secrets(operatorNamespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
	if err != nil {
		o.log.Errorf("deleting secret %v", err)
	}
}

func (o operator) deployIntegration(pod *corev1.Pod) {
	o.log.Infof("creating a new integration for %s", pod.Name)

	// The image to grab will come from an in the monitored service's pod annotation
	containerIntegration := corev1.Container{
		Name:  "integration",
		Image: "acabanas977/nri-redis:latest",
		Env: []corev1.EnvVar{
			{
				Name:  "WORKLOAD_NODE_IP",
				Value: pod.Status.HostIP,
			},
		},
		EnvFrom: []corev1.EnvFromSource{
			{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: pod.Name + "-integration",
					},
				},
			},
		},
		Resources: corev1.ResourceRequirements{},
	}

	// The port exposed can always be the same in all our prometheus-exporter images
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name + "-integration",
			Namespace: operatorNamespace,
			Labels: map[string]string{
				"monitoring-role": "integration-monitoring-workload",
			},
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/port":   "9121",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				containerIntegration,
			},
		},
	}

	_, err := o.client.CoreV1().Pods(operatorNamespace).Create(context.Background(), p, metav1.CreateOptions{})
	if err != nil {
		o.log.Errorf("creating pod %v", err)
	}

	data := map[string][]byte{}
	populatedString := strings.Replace(pod.Annotations["config"], "${discovery.ip}", pod.Status.PodIP, 100)
	rows := strings.Split(populatedString, "\n")
	for _, r := range rows {
		val := strings.Split(r, ": ")
		if len(val) == 2 {
			data[val[0]] = []byte(val[1])
		} else {
			o.log.Errorf("Unexpected string %q", rows)
		}

	}

	_, err = o.client.CoreV1().Secrets(operatorNamespace).Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name + "-integration",
			Namespace: operatorNamespace,
		},
		Data: data,
	}, metav1.CreateOptions{})
	if err != nil {
		o.log.Errorf("creating secret %v", err)
	}
}

func setupKSM(c *config.Config, clients *clusterClients, namespaceCache *discovery.NamespaceInMemoryStore) (*ksm.Scraper, error) {
	providers := ksm.Providers{
		K8s: clients.k8s,
		KSM: clients.ksm,
	}

	scraperOpts := []ksm.ScraperOpt{ksm.WithLogger(logger)}

	if c.NamespaceSelector != nil {
		nsFilter := discovery.NewNamespaceFilter(c.NamespaceSelector, clients.k8s, logger)
		scraperOpts = append(
			scraperOpts,
			ksm.WithFilterer(discovery.NewCachedNamespaceFilter(nsFilter, namespaceCache)),
		)
	}

	ksmScraper, err := ksm.NewScraper(c, providers, scraperOpts...)
	if err != nil {
		return nil, fmt.Errorf("building KSM scraper: %w", err)
	}

	return ksmScraper, nil
}

func setupControlPlane(c *config.Config, clients *clusterClients) (*controlplane.Scraper, error) {
	providers := controlplane.Providers{
		K8s: clients.k8s,
	}

	restConfig, err := getK8sConfig(c)
	if err != nil {
		return nil, err
	}

	controlplaneScraper, err := controlplane.NewScraper(
		c,
		providers,
		controlplane.WithLogger(logger),
		controlplane.WithRestConfig(restConfig),
	)
	if err != nil {
		return nil, fmt.Errorf("building control plane scraper: %w", err)
	}

	return controlplaneScraper, nil
}

func setupKubelet(c *config.Config, clients *clusterClients, namespaceCache *discovery.NamespaceInMemoryStore) (*kubelet.Scraper, error) {
	providers := kubelet.Providers{
		K8s:      clients.k8s,
		Kubelet:  clients.kubelet,
		CAdvisor: clients.cAdvisor,
	}

	scraperOpts := []kubelet.ScraperOpt{kubelet.WithLogger(logger)}

	if c.NamespaceSelector != nil {
		nsFilter := discovery.NewNamespaceFilter(c.NamespaceSelector, clients.k8s, logger)
		scraperOpts = append(
			scraperOpts,
			kubelet.WithFilterer(discovery.NewCachedNamespaceFilter(nsFilter, namespaceCache)),
		)
	}

	ksmScraper, err := kubelet.NewScraper(c, providers, scraperOpts...)
	if err != nil {
		return nil, fmt.Errorf("building kubelet scraper: %w", err)
	}

	return ksmScraper, nil
}

func buildClients(c *config.Config) (*clusterClients, error) {
	k8sConfig, err := getK8sConfig(c)
	if err != nil {
		return nil, fmt.Errorf("retrieving k8s config: %w", err)
	}

	k8s, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("building kubernetes client: %w", err)
	}

	var ksmCli *ksmClient.Client
	if c.KSM.Enabled {
		ksmCli, err = ksmClient.New(
			ksmClient.WithLogger(logger),
			ksmClient.WithTimeout(c.KSM.Timeout),
			ksmClient.WithMaxRetries(c.KSM.Retries),
		)
		if err != nil {
			return nil, fmt.Errorf("building KSM client: %w", err)
		}
	}

	var kubeletCli *kubeletClient.Client
	if c.Kubelet.Enabled {
		kubeletCli, err = kubeletClient.New(
			kubeletClient.DefaultConnector(k8s, c, k8sConfig, logger),
			kubeletClient.WithLogger(logger),
			kubeletClient.WithMaxRetries(c.Kubelet.Retries),
		)
		if err != nil {
			return nil, fmt.Errorf("building Kubelet client: %w", err)
		}
	}

	return &clusterClients{
		k8s:      k8s,
		ksm:      ksmCli,
		kubelet:  kubeletCli,
		cAdvisor: kubeletCli,
	}, nil
}

func getK8sConfig(c *config.Config) (*rest.Config, error) {
	inclusterConfig, err := rest.InClusterConfig()
	if err == nil {
		return inclusterConfig, nil
	}
	logger.Warnf("collecting in cluster config: %v", err)

	kubeconf := c.KubeconfigPath
	if kubeconf == "" {
		kubeconf = path.Join(homedir.HomeDir(), ".kube", "config")
	}

	inclusterConfig, err = clientcmd.BuildConfigFromFlags("", kubeconf)
	if err != nil {
		return nil, fmt.Errorf("could not load local kube config: %w", err)
	}

	logger.Warnf("using local kube config: %q", kubeconf)

	return inclusterConfig, nil
}
