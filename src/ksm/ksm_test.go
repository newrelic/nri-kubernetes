package ksm_test

// This file holds the integration tests for the KSM package.

import (
	"context"
	_ "embed"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/newrelic/nri-kubernetes/v2/internal/config"
	"github.com/newrelic/nri-kubernetes/v2/src/ksm"
	ksmClient "github.com/newrelic/nri-kubernetes/v2/src/ksm/client"
)

// ksmData holds sample data as returned by KSM.
//go:embed testdata/ksmData
var ksmData []byte

// servicesYml contains a list of services obtained with `kc get services -A -o yaml`.
//go:embed testdata/services.yml
var servicesYml []byte

func TestScraper(t *testing.T) {
	ksmServer := fakeKsm(t)

	ksmCli, err := ksmClient.New()
	if err != nil {
		t.Fatalf("error creating ksm client: %v", err)
	}

	fakeK8s := fake.NewSimpleClientset()
	addK8sData(t, fakeK8s)

	scraper, err := ksm.NewScraper(&config.Mock{
		KSM: config.KSM{
			StaticEndpoint: ksmServer.URL,
		},
	}, ksm.Providers{
		K8s: fakeK8s,
		KSM: ksmCli,
	})

	// TODO: WIP
	err = scraper.Run(nil)
	if err != nil {
		t.Fatal(err)
	}
}

func fakeKsm(t *testing.T) *httptest.Server {
	ksmServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("content-type", "text/plain")
		_, err := rw.Write(ksmData)
		if err != nil {
			t.Errorf("Error writing KSM data to HTTP response: %v", err)
		}
	}))

	go func() {
		ksmServer.Start()
		t.Logf("Fake KSM server started at %s", ksmServer.URL)
	}()

	t.Cleanup(func() {
		t.Logf("Shutting down KSM server")
		ksmServer.Close()
	})

	return ksmServer
}

func addK8sData(t *testing.T, cs *fake.Clientset) {
	var services []*corev1.Service
	err := yaml.Unmarshal(servicesYml, &services)
	if err != nil {
		t.Fatalf("cannot unmarshal testdata/services.yml: %v", err)
	}

	for _, svc := range services {
		_, err := cs.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: svc.Namespace,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("error creating namespace in fake clientSet: %v", err)
		}

		_, err = cs.CoreV1().Services(svc.Namespace).Create(context.Background(), svc, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("error creating svc in fake clientSet: %v", err)
		}
	}
}
