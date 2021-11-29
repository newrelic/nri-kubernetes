package deprecated

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v2/src/controlplane"
	clientControlPlane "github.com/newrelic/nri-kubernetes/v2/src/controlplane/client"
	"github.com/newrelic/nri-kubernetes/v2/src/data"
	kubeletClient "github.com/newrelic/nri-kubernetes/v2/src/kubelet/client"
	metric2 "github.com/newrelic/nri-kubernetes/v2/src/kubelet/metric"
	"github.com/newrelic/nri-kubernetes/v2/src/scrape"
)

var logger log.Logger = log.NewStdErr(true)

func RunControlPlane(config *config.Mock, k8s kubernetes.Interface, i *integration.Integration) error {
	const (
		nodeNameEnvVar = "NRK8S_NODE_NAME"
		defaultTimeout = time.Millisecond * 5000
	)

	node, err := k8s.CoreV1().Nodes().Get(context.Background(), config.NodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting info for node %q: %w", config.NodeName, err)
	}

	hostIP, err := getHostIP(node)
	if err != nil {
		return err
	}

	nodeName := os.Getenv(nodeNameEnvVar)
	if nodeName == "" {
		logger.Errorf("%s env var should be provided by Kubernetes and is mandatory", nodeNameEnvVar)
		os.Exit(1)
	}
	K8sConfig, _ := getK8sConfig(true)
	kubeletCli, err := kubeletClient.New(kubeletClient.DefaultConnector(k8s, config, K8sConfig, logger), kubeletClient.WithLogger(logger))
	if err != nil {
		return fmt.Errorf("building Kubelet client: %w", err)
	}

	cpJobs, err := controlPlaneJobs(
		logger,
		nodeName,
		defaultTimeout,
		hostIP,
		metric2.NewPodsFetcher(logger, kubeletCli).DoPodsFetch,
		k8s,
		config.ETCD.EtcdTLSSecretName,
		config.ETCD.EtcdTLSSecretNamespace,
		config.APIServer.APIServerSecurePort,
		config.Scheduler.SchedulerEndpointURL,
		config.ETCD.EtcdEndpointURL,
		config.ControllerManager.ControllerManagerEndpointURL,
		config.APIServer.APIServerEndpointURL,
	)

	if err != nil {
		logger.Errorf("couldn't configure control plane components jobs: %v", err)
	}

	K8sVersion, _ := k8s.Discovery().ServerVersion()

	successfulJobs := 0
	for _, job := range cpJobs {
		logger.Debugf("Running job: %s", job.Name)
		start := time.Now()
		result := job.Populate(i, config.ClusterName, logger, K8sVersion)
		measured := time.Since(start)
		logger.Debugf("Job %s took %s", job.Name, measured.Round(time.Millisecond))

		if result.Populated {
			successfulJobs++
		}

		if len(result.Errors) > 0 {
			logger.Infof("Error populating data from %s: %v", job.Name, result.Error())
		}
	}

	if successfulJobs == 0 {
		logger.Errorf("No data was populated")
		os.Exit(1)
	}
	return nil
}

func controlPlaneJobs(
	logger log.Logger,
	nodeName string,
	timeout time.Duration,
	nodeIP string,
	podsFetcher data.FetchFunc,
	k8sClient kubernetes.Interface,
	etcdTLSSecretName string,
	etcdTLSSecretNamespace string,
	apiServerSecurePort string,
	schedulerEndpointURL string,
	etcdEndpointURL string,
	controllerManagerEndpointURL string,
	apiServerEndpointURL string,
) ([]*scrape.Job, error) {

	// TODO No need to have this into the loop, it it a quick fix waiting for the refactor
	nodegetter, cl := discovery.NewNodeLister(k8sClient)
	defer close(cl)

	node, err := nodegetter.Get(nodeName)
	if err != nil {
		return nil, fmt.Errorf("couldn't query ApiServer server: %v", err)
	}

	if !IsMasterNode(node) {
		return nil, nil
	}

	var opts []controlplane.ComponentOption
	if etcdTLSSecretName != "" {
		opts = append(opts, controlplane.WithEtcdTLSConfig(etcdTLSSecretName, etcdTLSSecretNamespace))
	}

	// Make sure API Server Secure port is used first for backwards compatibility.
	if apiServerSecurePort != "" && apiServerEndpointURL != "" {
		return nil, fmt.Errorf("api server secure port and api server endpoint URL can not both be set")
	} else if apiServerSecurePort != "" {
		opts = append(opts, controlplane.WithAPIServerSecurePort(apiServerSecurePort))
	} else if apiServerEndpointURL != "" {
		opts = append(opts, controlplane.WithEndpointURL(controlplane.APIServer, apiServerEndpointURL))
	}

	if schedulerEndpointURL != "" {
		opts = append(opts, controlplane.WithEndpointURL(controlplane.Scheduler, schedulerEndpointURL))
	}

	if etcdEndpointURL != "" {
		opts = append(opts, controlplane.WithEndpointURL(controlplane.Etcd, etcdEndpointURL))
	}

	if controllerManagerEndpointURL != "" {
		opts = append(opts, controlplane.WithEndpointURL(controlplane.ControllerManager, controllerManagerEndpointURL))
	}

	var jobs []*scrape.Job
	for _, component := range controlplane.BuildComponentList(opts...) {

		// Components will be skipped if their configuration is not correct.
		if component.Skip {
			logger.Debugf("Skipping job creation for component %s: %s", component.Name, component.SkipReason)
			continue
		}

		componentDiscoverer := clientControlPlane.NewComponentDiscoverer(component, logger, nodeIP, podsFetcher, k8sClient)
		componentClient, err := componentDiscoverer.Discover(timeout)
		if err != nil {
			logger.Errorf("control plane component %s discovery failed: %v", component.Name, err)
		}

		c := componentClient.(*clientControlPlane.ControlPlaneComponentClient)

		if !c.IsComponentRunningOnNode {
			logger.Debugf(
				"Could not find component %s on this master node, skipping job. ",
				component.Name,
			)
			continue
		}

		componentGrouper := controlplane.NewComponentGrouper(
			componentClient,
			component.Queries,
			logger,
			c.PodName,
		)
		jobs = append(
			jobs,
			scrape.NewScrapeJob(string(component.Name), componentGrouper, component.Specs),
		)
	}

	return jobs, nil
}

func getHostIP(node *v1.Node) (string, error) {
	var ip string

	for _, address := range node.Status.Addresses {
		if address.Type == "InternalIP" {
			ip = address.Address
			break
		}
	}

	if ip == "" {
		return "", fmt.Errorf("could not get Kubelet host IP")
	}

	return ip, nil
}

func getK8sConfig(tryLocalKubeConfig bool) (*rest.Config, error) {
	c, err := rest.InClusterConfig()
	if err == nil || !tryLocalKubeConfig {
		return c, nil
	}

	kubeconf := path.Join(homedir.HomeDir(), ".kube", "config")
	c, err = clientcmd.BuildConfigFromFlags("", kubeconf)
	if err != nil {
		return nil, fmt.Errorf("could not load local kube config: %w", err)
	}
	return c, nil
}

// IsMasterNode returns true if the NodeInfo contains the labels that
// identify a node as master.
func IsMasterNode(node *v1.Node) bool {
	if val, ok := node.Labels["kubernetes.io/role"]; ok && val == "master" {
		return true
	}
	if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
		return true
	}
	return false
}
