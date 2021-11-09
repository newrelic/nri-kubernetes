package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Kubernetes provides an interface to common Kubernetes API operations
type Kubernetes interface {
	// FindNode returns a Node reference containing the pod named as the argument, if any
	FindNode(name string) (*v1.Node, error)

	// FindPodsByLabel returns a PodList reference containing the pods matching the provided label selector.
	FindPodsByLabel(namespace string, labelSelector metav1.LabelSelector) (*v1.PodList, error)

	// FindServicesByLabel returns a ServiceList containing the services matching the provided label selector.
	FindServicesByLabel(namespace string, labelSelector metav1.LabelSelector) (*v1.ServiceList, error)

	// ListServices returns a ServiceList containing all the services.
	ListServices(namespace string) (*v1.ServiceList, error)

	// Config returns a config of API client
	Config() *rest.Config

	// SecureHTTPClient returns http.Client configured with timeout and CA Cert
	SecureHTTPClient(time.Duration) (*http.Client, error)

	// FindSecret returns the secret with the given name, if any
	FindSecret(name, namespace string) (*v1.Secret, error)

	// ServerVersion returns the kubernetes server version.
	ServerVersion() (*version.Info, error)

	// GetClient return the client used internally
	GetClient() kubernetes.Interface
}

type goClientImpl struct {
	client *kubernetes.Clientset
	config *rest.Config
}

func (ka *goClientImpl) ServerVersion() (*version.Info, error) {
	return ka.client.ServerVersion()
}

func (ka *goClientImpl) Config() *rest.Config {
	return ka.config
}

func (ka *goClientImpl) FindNode(name string) (*v1.Node, error) {
	return ka.client.CoreV1().Nodes().Get(context.TODO(), name, metav1.GetOptions{})
}

func (ka *goClientImpl) FindPodsByLabel(namespace string, labelSelector metav1.LabelSelector) (*v1.PodList, error) {
	selectorMap, err := metav1.LabelSelectorAsMap(&labelSelector)
	if err != nil {
		return nil, fmt.Errorf("converting label selector %q to map: %w", labelSelector, err)
	}

	return ka.client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{ // nosemgrep: context-todo
		LabelSelector: labels.SelectorFromSet(selectorMap).String(),
	})
}

func (ka *goClientImpl) FindServicesByLabel(namespace string, labelSelector metav1.LabelSelector) (*v1.ServiceList, error) {
	selectorMap, err := metav1.LabelSelectorAsMap(&labelSelector)
	if err != nil {
		return nil, fmt.Errorf("converting label selector %q to map: %w", labelSelector, err)
	}

	return ka.client.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{ // nosemgrep: context-todo
		LabelSelector: labels.SelectorFromSet(selectorMap).String(),
	})
}

func (ka *goClientImpl) ListServices(namespace string) (*v1.ServiceList, error) {
	return ka.client.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{}) // nosemgrep: context-todo
}

func (ka *goClientImpl) SecureHTTPClient(t time.Duration) (*http.Client, error) {
	c, ok := ka.client.RESTClient().(*rest.RESTClient)
	if !ok {
		return nil, errors.New("failed to set up a client for connecting to Kubelet through API proxy")
	}
	return c.Client, nil
}

func (ka *goClientImpl) FindSecret(name, namespace string) (*v1.Secret, error) {
	return ka.client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (ka *goClientImpl) GetClient() kubernetes.Interface {
	return ka.client
}

// BasicHTTPClient returns http.Client configured with timeout
func BasicHTTPClient(t time.Duration) *http.Client {
	return &http.Client{
		Timeout: t,
	}
}

// InsecureHTTPClient returns http.Client configured with timeout
// and InsecureSkipVerify flag enabled
func InsecureHTTPClient(t time.Duration) *http.Client {
	client := BasicHTTPClient(t)
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return client
}

// NewKubernetes instantiates a Kubernetes API client
// if tryLocalKubeConfig is true, this will try to load your kubeconfig from ~/.kube/config
func NewKubernetes(tryLocalKubeConfig bool) (Kubernetes, error) {
	ka := new(goClientImpl)
	var err error

	ka.config, err = rest.InClusterConfig()
	if err != nil {
		if !tryLocalKubeConfig {
			return nil, err
		}

		kubeconf := path.Join(homedir.HomeDir(), ".kube", "config")
		config, err := clientcmd.BuildConfigFromFlags("", kubeconf)
		if err != nil {
			return nil, errors.Wrap(err, "could not load local kube config")
		}
		ka.config = config
	}

	ka.client, err = kubernetes.NewForConfig(ka.config)
	if err != nil {
		return nil, err
	}

	return ka, nil
}
