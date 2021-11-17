package client

import (
	"context"
	"crypto/tls"
	"net/http"
	"path"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Kubernetes provides an interface to common Kubernetes API operations
type Kubernetes interface {

	// FindSecret returns the secret with the given name, if any
	FindSecret(name, namespace string) (*v1.Secret, error)

	// GetClient return the client used internally
	GetClient() *kubernetes.Clientset
}

type goClientImpl struct {
	client *kubernetes.Clientset
	config *rest.Config
}

func (ka *goClientImpl) FindSecret(name, namespace string) (*v1.Secret, error) {
	return ka.client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (ka *goClientImpl) GetClient() *kubernetes.Clientset {
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
