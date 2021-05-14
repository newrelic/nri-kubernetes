package client

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/transport"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/newrelic/nri-kubernetes/src/prometheus"
)

var ksmAppLabelNames = []string{"app.kubernetes.io/name", "k8s-app", "app"}
var errNoKSMPodsFound = errors.New("no KSM pods found")

const (
	ksmAppLabelValue         = "kube-state-metrics"
	ksmPortName              = "http-metrics"
	k8sTCP                   = "TCP"
	ksmQualifiedName         = "kube-state-metrics.kube-system.svc.cluster.local"
	ksmDNSService            = "http-metrics"
	ksmDNSProto              = "tcp"
	headlessServiceClusterIP = "None"
)

type lookupSRVFunc func(service, proto, name string) (cname string, addrs []*net.SRV, err error)

// discoverer implements Discoverer interface by using official Kubernetes' Go client
type discoverer struct {
	lookupSRV         lookupSRVFunc
	apiClient         client.Kubernetes
	logger            *logrus.Logger
	overridenEndpoint string
}

// ksm implements Client interface
type ksm struct {
	httpClient *http.Client
	endpoint   url.URL
	nodeIP     string
	logger     *logrus.Logger
}

func (sd *discoverer) Discover(timeout time.Duration) (client.HTTPClient, error) {
	var endpoint url.URL
	if sd.overridenEndpoint != "" {
		sd.logger.Debug("Using user-defined KSM endpoint " + sd.overridenEndpoint)
		ep, err := url.Parse(sd.overridenEndpoint)
		if err != nil {
			return nil, fmt.Errorf("wrong user-provided KSM endpoint: %s", err)
		}
		endpoint = *ep
	} else {
		var err error
		sd.logger.Debug("Attempting DNS discovery of KSM endpoint")
		endpoint, err = sd.dnsDiscover()
		if err != nil {
			// if DNS discovery fails, we dig into Kubernetes API to get the service data
			sd.logger.Debug("Attempting API server discovery of KSM endpoint")
			endpoint, err = sd.apiDiscover()
			if err != nil {
				return nil, fmt.Errorf("failed to discover kube-state-metrics endpoint, got error: %s", err)
			}
		}
	}

	// KSM and Prometheus only work with HTTP
	endpoint.Scheme = "http"
	nodeIP, err := sd.nodeIP()
	if err != nil {
		return nil, fmt.Errorf("failed to discover nodeIP with kube-state-metrics, got error: %s", err)
	}

	sd.logger.Debugf("KSM client created with endpoint=%v and nodeIP=%v", endpoint, nodeIP)
	return newKSMClient(timeout, nodeIP, endpoint, sd.logger, sd.apiClient), nil
}

func newKSMClient(timeout time.Duration, nodeIP string, endpoint url.URL, logger *logrus.Logger, k8s client.Kubernetes) *ksm {
	bearer := k8s.Config().BearerToken
	rt := newBearerRoundTripper(bearer)

	return &ksm{
		nodeIP:   nodeIP,
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: rt,
		},
		logger: logger,
	}
}

func newBearerRoundTripper(bearer string) http.RoundTripper {
	baseTransport := http.DefaultTransport.(*http.Transport).Clone()
	baseTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return transport.NewBearerAuthRoundTripper(bearer, baseTransport)
}

func (c *ksm) NodeIP() string {
	return c.nodeIP
}

func (c *ksm) Do(method, urlPath string) (*http.Response, error) {
	e := c.endpoint
	e.Path = path.Join(c.endpoint.Path, urlPath)

	r, err := prometheus.NewRequest(method, e.String())
	if err != nil {
		return nil, fmt.Errorf("Error creating %s request to: %s. Got error: %s ", method, e.String(), err)
	}

	c.logger.Debugf("Calling kube-state-metrics endpoint: %s", r.URL.String())

	return c.httpClient.Do(r)
}

// dnsDiscover uses DNS to discover KSM
func (sd *discoverer) dnsDiscover() (url.URL, error) {
	var endpoint url.URL
	_, addrs, err := sd.lookupSRV(ksmDNSService, ksmDNSProto, ksmQualifiedName)
	if err == nil {
		for _, addr := range addrs {
			if addr.Target == headlessServiceClusterIP {
				continue
			}
			endpoint.Host = fmt.Sprintf("%v:%v", ksmQualifiedName, addr.Port)
			return endpoint, nil
		}
	}
	return endpoint, fmt.Errorf("can't get DNS port for %s", ksmQualifiedName)
}

// apiDiscover uses Kubernetes API to discover KSM
func (sd *discoverer) apiDiscover() (url.URL, error) {
	var endpoint url.URL
	var services *v1.ServiceList
	var err error

	for _, label := range ksmAppLabelNames {
		services, err = sd.apiClient.FindServicesByLabel(label, ksmAppLabelValue)
		if err == nil && len(services.Items) > 0 {
			break
		}
	}

	if err != nil {
		return endpoint, err
	}
	if len(services.Items) == 0 {
		return endpoint, fmt.Errorf("no services found by any of labels %v with value %s", ksmAppLabelNames, ksmAppLabelValue)
	}

	for _, service := range services.Items {
		if service.Spec.ClusterIP != "" && len(service.Spec.Ports) > 0 {
			// Look for a port called "http-metrics"
			for _, port := range service.Spec.Ports {
				if port.Name == ksmPortName {
					endpoint.Host = fmt.Sprintf("%v:%v", service.Spec.ClusterIP, port.Port)
					return endpoint, nil
				}
			}
			// If not found, return the first TCP port
			for _, port := range service.Spec.Ports {
				if port.Protocol == k8sTCP {
					endpoint.Host = fmt.Sprintf("%v:%v", service.Spec.ClusterIP, port.Port)
					return endpoint, nil
				}
			}
		}
	}

	return endpoint, fmt.Errorf("could not guess the Kube State Metrics host/port")
}

// nodeIP discover IP of a node, where kube-state-metrics is installed
func (sd *discoverer) nodeIP() (string, error) {
	var pods *v1.PodList
	var err error

	for _, label := range ksmAppLabelNames {
		pods, err = sd.apiClient.FindPodsByLabel(label, ksmAppLabelValue)
		if err == nil && len(pods.Items) > 0 {
			break
		}
	}

	if err != nil {
		return "", err
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found by any of labels %v with value %s", ksmAppLabelNames, ksmAppLabelValue)
	}

	// In case there are multiple pods for the same service, we must be sure we always show the Node IP of the
	// same pod. So we chose, for example, the HostIp with highest precedence in alphabetical order
	var nodeIP string
	for _, pod := range pods.Items {
		if pod.Status.HostIP != "" && (nodeIP == "" || strings.Compare(pod.Status.HostIP, nodeIP) < 0) {
			nodeIP = pod.Status.HostIP
		}
	}
	if nodeIP == "" {
		return "", errors.New("no HostIP address found for KSM node")
	}
	return nodeIP, nil
}

// NewDiscoverer instantiates a new Discoverer required for discovering node IP
// of kube-state-metrics pod and endpoint of kube-state-metrics service
func NewDiscoverer(logger *logrus.Logger, kubernetes client.Kubernetes) client.Discoverer {
	return NewStaticEndpointDiscoverer("", logger, kubernetes)
}

// NewStaticEndpointDiscoverer instantiates a new Discoverer required for discovering only
// node IP of kube-state-metrics pod
func NewStaticEndpointDiscoverer(ksmEndpoint string, logger *logrus.Logger, kubernetes client.Kubernetes) client.Discoverer {
	return &discoverer{
		lookupSRV:         net.LookupSRV,
		apiClient:         kubernetes,
		logger:            logger,
		overridenEndpoint: ksmEndpoint,
	}
}
