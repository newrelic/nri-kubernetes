package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/newrelic/nri-kubernetes/src/controlplane"
	"github.com/newrelic/nri-kubernetes/src/data"
	"github.com/newrelic/nri-kubernetes/src/definition"
	"github.com/newrelic/nri-kubernetes/src/prometheus"
)

const podEntityType = "pod"

type invalidTLSConfig struct {
	message string
}

func (i invalidTLSConfig) Error() string {
	return i.message
}

type authenticationMethod string

const (
	none           authenticationMethod = "None (http)"
	mTLS           authenticationMethod = "Mutual TLS"
	serviceAccount authenticationMethod = "Service account (Bearer token)"
)

// ControlPlaneComponentClient implements Client interface.
type ControlPlaneComponentClient struct {
	authenticationMethod     authenticationMethod
	httpClient               *http.Client
	tlsSecretName            string
	tlsSecretNamespace       string
	logger                   *logrus.Logger
	IsComponentRunningOnNode bool
	k8sClient                client.Kubernetes
	endpoint                 url.URL
	nodeIP                   string
	PodName                  string
}

func (c *ControlPlaneComponentClient) Do(method, urlPath string) (*http.Response, error) {
	e := c.endpoint
	e.Path = path.Join(c.endpoint.Path, urlPath)

	r, err := prometheus.NewRequest(method, e.String())
	if err != nil {
		return nil, fmt.Errorf("Error creating %s request to: %s. Got error: %v ", method, e.String(), err)
	}

	if err = c.configureAuthentication(); err != nil {
		return nil, errors.Wrapf(err, "could not configure request for authentication method %s", c.authenticationMethod)
	}

	c.logger.Debugf("Calling endpoint: %s, authentication method: %s", r.URL.String(), string(c.authenticationMethod))

	return c.httpClient.Do(r)
}

func (c *ControlPlaneComponentClient) configureAuthentication() error {

	if c.authenticationMethod == mTLS {
		tlsConfig, err := c.getTLSConfigFromSecret()
		if err != nil {
			return errors.Wrap(err, "could not load TLS configuration")
		}

		c.httpClient.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		return nil
	}

	if c.authenticationMethod == serviceAccount {

		config, err := rest.InClusterConfig()
		if err != nil {
			return errors.Wrapf(err, "could not create in cluster Kubernetes configuration to query pod: %s", c.PodName)
		}

		// Here we're using the default http.Transport configuration, but with a modified TLS config.
		// For some reason the DefaultTransport is casted to an http.RoundTripper interface, so we need to convert it back.
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

		// Use the default kubernetes Bearer token authentication RoundTripper
		c.httpClient.Transport = transport.NewBearerAuthRoundTripper(config.BearerToken, t)
		return nil
	}

	c.httpClient.Transport = http.DefaultTransport
	return nil
}

func (c *ControlPlaneComponentClient) getTLSConfigFromSecret() (*tls.Config, error) {

	namespace := c.tlsSecretNamespace
	if namespace == "" {
		c.logger.Debug("TLS Secret name configured, but not TLS Secret namespace. Defaulting to `default` namespace.")
		namespace = "default"
	}

	secret, err := c.k8sClient.FindSecret(c.tlsSecretName, namespace)

	if err != nil {
		return nil, errors.Wrapf(err, "could not find secret %s containing TLS configuration", c.tlsSecretName)
	}

	var cert, key, cacert []byte

	var ok bool
	if cert, ok = secret.Data["cert"]; !ok {
		return nil, invalidTLSConfig{
			message: fmt.Sprintf("could not find TLS certificate in `cert` field in secret %s", c.tlsSecretName),
		}
	}

	if key, ok = secret.Data["key"]; !ok {
		return nil, invalidTLSConfig{
			message: fmt.Sprintf("could not find TLS key in `key` field in secret %s", c.tlsSecretName),
		}
	}

	cacert, hasCACert := secret.Data["cacert"]
	insecureSkipVerifyRaw, hasInsecureSkipVerify := secret.Data["insecureSkipVerify"]

	if !hasCACert && !hasInsecureSkipVerify {
		return nil, invalidTLSConfig{
			message: "both cacert and insecureSkipVerify are not set. One of them need to be set to be able to call ETCD metrics",
		}
	}

	// insecureSkipVerify is set to false by default, and can be overridden with the insecureSkipVerify field
	insecureSkipVerify := false
	if hasInsecureSkipVerify {
		insecureSkipVerify = strings.ToLower(string(insecureSkipVerifyRaw)) == "true"
	}

	return parseTLSConfig(cert, key, cacert, insecureSkipVerify)
}

func parseTLSConfig(certPEMBlock, keyPEMBlock, cacertPEMBlock []byte, insecureSkipVerify bool) (*tls.Config, error) {

	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return nil, err
	}

	clientCertPool := x509.NewCertPool()

	if len(cacertPEMBlock) > 0 {
		clientCertPool.AppendCertsFromPEM(cacertPEMBlock)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            clientCertPool,
		InsecureSkipVerify: insecureSkipVerify,
	}

	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}

func (c *ControlPlaneComponentClient) NodeIP() string {
	return c.nodeIP
}

// discoverer implements Discoverer interface by using official
// Kubernetes' Go client.
type discoverer struct {
	logger      *logrus.Logger
	component   controlplane.Component
	nodeIP      string
	podsFetcher data.FetchFunc
	k8sClient   client.Kubernetes
}

func (sd *discoverer) Discover(timeout time.Duration) (client.HTTPClient, error) {
	nodePods, err := sd.podsFetcher()
	if err != nil {
		return nil, err
	}
	podName, isComponentRunningOnNode := sd.findComponentOnNode(nodePods)

	var authMethod authenticationMethod

	// Let mTLS take precedence over service account
	switch {
	case sd.component.UseMTLSAuthentication:
		authMethod = mTLS
	case sd.component.UseServiceAccountAuthentication:
		authMethod = serviceAccount
	default:
		authMethod = none
	}

	return &ControlPlaneComponentClient{
		endpoint:                 sd.component.Endpoint,
		tlsSecretName:            sd.component.TLSSecretName,
		tlsSecretNamespace:       sd.component.TLSSecretNamespace,
		IsComponentRunningOnNode: isComponentRunningOnNode,
		PodName:                  podName,
		authenticationMethod:     authMethod,
		logger:                   sd.logger,
		nodeIP:                   sd.nodeIP,
		k8sClient:                sd.k8sClient,
		httpClient:               &http.Client{Timeout: timeout},
	}, nil
}

func (sd *discoverer) findComponentOnNode(nodePods definition.RawGroups) (string, bool) {
	for _, podData := range nodePods[podEntityType] {
		rawValueLabels, ok := podData["labels"]
		if !ok {
			continue
		}

		podLabels, ok := rawValueLabels.(map[string]string)
		if !ok {
			continue
		}

		// Loop over the different sets of labels that this component might have, and check if this pod has all the labels from one set.
		// e.g., for the scheduler, these are the sets:
		// Labels[0] = {"k8s-app": "kube-scheduler"}
		// Labels[1] = {"tier": "control-plane", "component": "kube-scheduler"}
		for _, labels := range sd.component.Labels {
			foundLabels := 0

			// check if each label of this set is present on the pod
			for labelKey, labelValue := range labels {
				if podLabels[labelKey] == labelValue {
					foundLabels++
				}
			}

			// Is every label from this set present on the pod? If not, continue
			if foundLabels != len(labels) {
				continue
			}

			rawValuePodName, ok := podData["podName"]
			if !ok {
				continue
			}

			podName, ok := rawValuePodName.(string)
			if !ok {
				continue
			}
			return podName, true
		}
	}
	return "", false
}

// NewComponentDiscoverer returns a `Discoverer` that will find the
// control plane components that are running on this node.
func NewComponentDiscoverer(
	component controlplane.Component,
	logger *logrus.Logger,
	nodeIP string,
	podsFetcher data.FetchFunc,
	k8sClient client.Kubernetes,
) client.Discoverer {
	return &discoverer{
		logger:      logger,
		component:   component,
		nodeIP:      nodeIP,
		podsFetcher: podsFetcher,
		k8sClient:   k8sClient,
	}
}
