package k8s

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"bytes"

	"strings"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/remotecommand"
)

// Client is a simple k8s api client for testing purposes.
type Client struct {
	Clientset     *kubernetes.Clientset
	Config        *rest.Config
	serverVersion *version.Info
}

// NewClient returns a k8s api client.
func NewClient(context string) (*Client, error) {
	var c *rest.Config
	var err error

	configFilepath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	if context != "" {
		c, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: configFilepath},
			&clientcmd.ConfigOverrides{
				ClusterInfo:    clientcmdapi.Cluster{Server: ""},
				CurrentContext: context,
			}).ClientConfig()
	} else {
		c, err = clientcmd.BuildConfigFromFlags("", configFilepath)
	}

	if err != nil {
		return nil, err
	}

	if c.Timeout == 0 {
		c.Timeout = 5 * time.Second
	}

	cs, err := kubernetes.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	sv, err := cs.ServerVersion()
	if err != nil {
		return nil, err
	}

	return &Client{
		Clientset:     cs,
		Config:        c,
		serverVersion: sv,
	}, nil
}

// ReqOutput is the output of a request made to k8s api.
type ReqOutput struct {
	Stdout bytes.Buffer
	Stderr bytes.Buffer
}

// ServerVersion returns the k8s server version.
func (c Client) ServerVersion() string {
	return c.serverVersion.String()
}

// NodesList list nodes.
func (c Client) NodesList() (*v1.NodeList, error) {
	return c.Clientset.CoreV1().Nodes().List(metav1.ListOptions{})
}

// ServiceAccount finds a serviceaccount into the namespace a service account with the given name
func (c Client) ServiceAccount(namespace, name string) (*v1.ServiceAccount, error) {
	return c.Clientset.CoreV1().ServiceAccounts(namespace).Get(name, metav1.GetOptions{})
}

// CreateServiceAccount creates a serviceaccount into the namespace a service account with the given name
func (c Client) CreateServiceAccount(namespace, name string) (*v1.ServiceAccount, error) {
	return c.Clientset.CoreV1().ServiceAccounts(namespace).Create(&v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	})
}

// ClusterRoleBinding finds a clusterrolebinding with the given name
func (c Client) ClusterRoleBinding(name string) (*rbacv1.ClusterRoleBinding, error) {
	return c.Clientset.RbacV1().ClusterRoleBindings().Get(name, metav1.GetOptions{})
}

// ClusterRole finds a clusterrole with the given name
func (c Client) ClusterRole(name string) (*rbacv1.ClusterRole, error) {
	return c.Clientset.RbacV1().ClusterRoles().Get(name, metav1.GetOptions{})
}

// CreateClusterRoleBinding creates a clusterrolebinding with the given name and links it with the serviceaccount
func (c Client) CreateClusterRoleBinding(name string, sa *v1.ServiceAccount, cr *rbacv1.ClusterRole) (*rbacv1.ClusterRoleBinding, error) {
	return c.Clientset.RbacV1().ClusterRoleBindings().Create(&rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name: cr.Name,
			Kind: "ClusterRole",
		},
	})
}

// PodsListByLabels list pods filtered by labels.
func (c Client) PodsListByLabels(namespace string, labels []string) (*v1.PodList, error) {
	labelStr := strings.Join(labels, ",")

	pods, err := c.Clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: labelStr,
	})
	if err != nil {
		return nil, fmt.Errorf("error retrieving pod list by labels %s - %s", labelStr, err)
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("pods not found by labels %s", labelStr)
	}

	return pods, nil
}

// PodExec executes a command on a pod container.
func (c Client) PodExec(namespace, podName, containerName string, command ...string) (ReqOutput, error) {
	execReq := c.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		Param("container", containerName).
		Param("stdin", "false").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "false")

	for _, c := range command {
		execReq.Param("command", c)
	}

	return c.apiRequest(execReq, "POST")
}

func (c Client) apiRequest(r *rest.Request, method string) (ReqOutput, error) {
	var output ReqOutput

	exec, err := remotecommand.NewSPDYExecutor(c.Config, method, r.URL())
	if err != nil {
		return output, err
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &output.Stdout,
		Stderr: &output.Stderr,
	})

	if err != nil {
		return output, fmt.Errorf("%s. Output:\n\n%s", err, output.Stderr.String())
	}

	return output, nil
}
