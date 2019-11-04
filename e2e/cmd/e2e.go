package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"

	_ "github.com/newrelic/nri-kubernetes/e2e/gcp"
	"github.com/newrelic/nri-kubernetes/e2e/helm"
	"github.com/newrelic/nri-kubernetes/e2e/jsonschema"
	"github.com/newrelic/nri-kubernetes/e2e/k8s"
	"github.com/newrelic/nri-kubernetes/e2e/retry"
	"github.com/newrelic/nri-kubernetes/e2e/timer"
)

var cliArgs = struct {
	NrChartPath                string `default:"e2e/charts/newrelic-infrastructure-k8s-e2e" help:"Path to the newrelic-infrastructure-k8s-e2e chart"`
	SchemasDirectory           string `default:"e2e/schema" help:"Directory where JSON schema files are defined"`
	IntegrationImageTag        string `default:"1.0.0" help:"Integration image tag"`
	IntegrationImageRepository string `default:"newrelic/infrastructure-k8s" help:"Integration image repository"`
	Rbac                       bool   `default:"false" help:"Enable rbac"`
	ClusterName                string `help:"Identifier of your cluster. You could use it later to filter data in your New Relic account"`
	NrLicenseKey               string `help:"New Relic account license key"`
	Verbose                    bool   `default:"false" help:"When enabled, more detailed output will be printed"`
	CollectorURL               string `default:"https://staging-infra-api.newrelic.com" help:"New Relic backend collector url"`
	Context                    string `default:"" help:"Kubernetes context"`
	CleanBeforeRun             bool   `default:"true" help:"Clean the cluster before running the tests"`
	FailFast                   bool   `default:"false" help:"Fail the whole suit on the first failure"`
	Unprivileged               bool   `default:"false" help:"Deploy and run the integration in unprivileged mode"`
}{}

const (
	nrLabel      = "name=newrelic-infra"
	namespace    = "default"
	nrContainer  = "newrelic-infra"
	ksmLabel     = "app=kube-state-metrics"
	minikubeHost = "https://192.168.99.100:8443"
)

type eventTypeSchemasPerEntity map[entityID]jsonschema.EventTypeToSchemaFilename

type entityID string

func (e entityID) Name() string {
	s := e.split()
	return s[len(s)-1]
}

func (e entityID) Type() string {
	s := e.split()
	return strings.Join(s[:len(s)-1], ":")
}

func (e entityID) split() []string {
	return strings.Split(string(e), ":")
}

func scenarios(integrationImageRepository string, integrationImageTag string, rbac bool, unprivileged bool) []string {
	return []string{
		s(rbac, unprivileged, integrationImageRepository, integrationImageTag, "v1.1.0", false),
		s(rbac, unprivileged, integrationImageRepository, integrationImageTag, "v1.1.0", true),
		s(rbac, unprivileged, integrationImageRepository, integrationImageTag, "v1.2.0", false),
		s(rbac, unprivileged, integrationImageRepository, integrationImageTag, "v1.2.0", true),
		s(rbac, unprivileged, integrationImageRepository, integrationImageTag, "v1.3.0", false),
		s(rbac, unprivileged, integrationImageRepository, integrationImageTag, "v1.3.0", true),
		s(rbac, unprivileged, integrationImageRepository, integrationImageTag, "v1.4.0", false),
		s(rbac, unprivileged, integrationImageRepository, integrationImageTag, "v1.4.0", true),
		s(rbac, unprivileged, integrationImageRepository, integrationImageTag, "v1.5.0", false),
		s(rbac, unprivileged, integrationImageRepository, integrationImageTag, "v1.5.0", true),
	}
}

func s(rbac bool, unprivileged bool, integrationImageRepository, integrationImageTag, ksmVersion string, twoKSMInstances bool) string {
	str := fmt.Sprintf("rbac=%v,ksm-instance-one.rbac.create=%v,ksm-instance-one.image.tag=%s,daemonset.unprivileged=%v,daemonset.image.repository=%s,daemonset.image.tag=%s", rbac, rbac, ksmVersion, unprivileged, integrationImageRepository, integrationImageTag)
	if twoKSMInstances {
		return fmt.Sprintf("%s,ksm-instance-two.rbac.create=%v,ksm-instance-two.image.tag=%s,two-ksm-instances=true", str, rbac, ksmVersion)
	}

	return str
}

type integrationData struct {
	expectedJobs []job
	podName      string
	stdOut       []byte
	stdErr       []byte
	err          error
}

type executionErr struct {
	errs []error
}

// Error implements Error interface
func (err executionErr) Error() string {
	var errsStr string
	for _, e := range err.errs {
		errsStr += fmt.Sprintf("%s\n", e)
	}

	return errsStr
}

type job string

const (
	jobKSM               job = "kube-state-metrics"
	jobKubelet           job = "kubelet"
	jobScheduler         job = "scheduler"
	jobEtcd              job = "etcd"
	jobControllerManager job = "controller-manager"
	jobAPIServer         job = "api-server"
)

var allJobs = [...]job{jobKSM, jobKubelet, jobScheduler, jobEtcd, jobControllerManager, jobAPIServer}

func execIntegration(pod v1.Pod, ksmPod *v1.Pod, dataChannel chan integrationData, wg *sync.WaitGroup, c *k8s.Client, logger *logrus.Logger) {
	defer timer.Track(time.Now(), fmt.Sprintf("execIntegration func for pod %s", pod.Name), logger)
	defer wg.Done()
	d := integrationData{
		podName: pod.Name,
	}

	output, err := c.PodExec(namespace, pod.Name, nrContainer, "/var/db/newrelic-infra/newrelic-integrations/bin/nr-kubernetes", "-timeout=15000", "-verbose")
	if err != nil {
		d.err = err
		dataChannel <- d
		return
	}

	d.stdOut = output.Stdout.Bytes()
	d.stdErr = output.Stderr.Bytes()

	for _, j := range allJobs {
		expectedStr := fmt.Sprintf("Running job: %s", string(j))
		if strings.Contains(string(d.stdErr), expectedStr) {
			d.expectedJobs = append(d.expectedJobs, j)
		}
	}

	logrus.Printf("Pod: %s, hostIP %s, expected jobs: %#v", pod.Name, pod.Status.HostIP, d.expectedJobs)

	dataChannel <- d
}

func main() {
	err := args.SetupArgs(&cliArgs)
	if err != nil {
		panic(err.Error())
	}

	if cliArgs.NrLicenseKey == "" || cliArgs.ClusterName == "" {
		panic("license key and cluster name are required args")
	}
	logger := log.New(cliArgs.Verbose)

	c, err := k8s.NewClient(cliArgs.Context)
	if err != nil {
		panic(err.Error())
	}
	logger.Infof("Executing tests in %q cluster. K8s version: %s", c.Config.Host, c.ServerVersion())

	err = initHelm(c, cliArgs.Rbac, logger)
	if err != nil {
		panic(err.Error())
	}

	if cliArgs.CleanBeforeRun {
		logger.Infof("Cleaning cluster")
		err := helm.DeleteAllReleases(cliArgs.Context, logger)
		if err != nil {
			panic(err.Error())
		}
	}

	// TODO
	var errs []error
	ctx := context.TODO()
	for _, s := range scenarios(cliArgs.IntegrationImageRepository, cliArgs.IntegrationImageTag, cliArgs.Rbac, cliArgs.Unprivileged) {
		logger.Infof("Scenario: %q", s)
		err := executeScenario(ctx, s, c, logger)
		if err != nil {
			if cliArgs.FailFast {
				logger.Info("Finishing execution because 'FailFast' is true")
				logger.Fatal(err.Error())
			}
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		logger.Debugf("errors collected from all scenarios")
		for _, err := range errs {
			logger.Errorf(err.Error())
		}
	} else {
		logger.Infof("OK")
	}
}

func initHelm(c *k8s.Client, rbac bool, logger *logrus.Logger) error {
	var initArgs []string
	if rbac {
		ns := "kube-system"
		n := "tiller"
		sa, err := c.ServiceAccount(ns, n)
		if err != nil {
			sa, err = c.CreateServiceAccount(ns, n)
			if err != nil {
				return err
			}
		}
		_, err = c.ClusterRoleBinding(n)
		if err != nil {
			cr, err := c.ClusterRole("cluster-admin")
			if err != nil {
				return err
			}
			_, err = c.CreateClusterRoleBinding(n, sa, cr)
			if err != nil {
				return err
			}
		}
		initArgs = []string{"--service-account", n}
	}

	err := helm.Init(
		cliArgs.Context,
		logger,
		initArgs...,
	)

	if err != nil {
		return err
	}

	return helm.DependencyBuild(cliArgs.Context, cliArgs.NrChartPath, logger)
}

func waitForKSM(c *k8s.Client, logger *logrus.Logger) (*v1.Pod, error) {
	defer timer.Track(time.Now(), "waitForKSM", logger)
	var foundPod v1.Pod
	err := retry.Do(
		func() error {
			ksmPodList, err := c.PodsListByLabels(namespace, []string{ksmLabel})
			if err != nil {
				return err
			}
			if len(ksmPodList.Items) != 0 && ksmPodList.Items[0].Status.Phase == "Running" {
				for _, con := range ksmPodList.Items[0].Status.Conditions {
					logger.Debugf("Waiting for kube-state-metrics pod to be ready, current condition: %s - %s", con.Type, con.Status)

					if con.Type == "Ready" && con.Status == "True" {
						foundPod = ksmPodList.Items[0]
						return nil
					}
				}
			}
			return fmt.Errorf("kube-state-metrics is not ready yet")
		},
		retry.OnRetry(func(err error) {
			logger.Debugf("Retrying due to: %s", err)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("kube-state-metrics pod is not ready: %s", err)
	}
	return &foundPod, nil
}

func executeScenario(ctx context.Context, scenario string, c *k8s.Client, logger *logrus.Logger) error {
	defer timer.Track(time.Now(), fmt.Sprintf("executeScenario func for %s", scenario), logger)

	releaseName, err := installRelease(ctx, scenario, logger)
	if err != nil {
		return err
	}

	defer func() {
		_ = helm.DeleteRelease(releaseName, cliArgs.Context, logger)
	}()

	// At least one of kube-state-metrics pods needs to be ready to enter to the newrelic-infra pod and execute the integration.
	// If the kube-state-metrics pod is not ready, then metrics from replicaset, namespace and deployment will not be populate and JSON schemas will fail.
	ksmPod, err := waitForKSM(c, logger)
	if err != nil {
		return err
	}
	return executeTests(c, ksmPod, releaseName, logger)
}

func executeTests(c *k8s.Client, ksmPod *v1.Pod, releaseName string, logger *logrus.Logger) error {
	// We're fetching the list of NR pods here just to fetch it once. If for
	// some reason this list or the contents of it could change during the
	// execution of these tests, we could move it to `test*` functions.
	podsList, err := c.PodsListByLabels(namespace, []string{nrLabel})
	if err != nil {
		return err
	}

	logger.Info("Found the following pods for test execution:")
	for _, pod := range podsList.Items {
		logger.Infof("[%s] status: %s %s", pod.Name, pod.Status.Message, pod.Status.Reason)
	}

	nodes, err := c.NodesList()
	if err != nil {
		return fmt.Errorf("error getting the list of nodes in the cluster: %s", err)
	}
	output, err := executeIntegrationForAllPods(c, ksmPod, podsList, logger)
	if err != nil {
		return err
	}
	var execErr executionErr
	logger.Info("checking if the integrations are executed with the proper roles")
	err = testRoles(len(nodes.Items), output)
	if err != nil {
		execErr.errs = append(execErr.errs, err)
	}
	if c.Config.Host == minikubeHost {
		logger.Info("Skipping `testSpecificEntities` because you're running them in Minikube (persistent volumes don't work well in Minikube)")
	} else {
		logger.Info("checking if specific entities match our JSON schemas")
		err = retry.Do(
			func() error {
				err := testSpecificEntities(output, releaseName)
				if err != nil {
					var otherErr error
					output, otherErr = executeIntegrationForAllPods(c, ksmPod, podsList, logger)
					if otherErr != nil {
						return otherErr
					}
					return err
				}
				return nil
			},
			retry.OnRetry(func(err error) {
				logger.Debugf("Retrying due to: %s", err)
			}),
		)
		if err != nil {
			execErr.errs = append(execErr.errs, err)
		}
	}
	logger.Info("checking if the metric sets in all integrations match our JSON schemas")
	err = retry.Do(
		func() error {
			err := testEventTypes(output)
			if err != nil {
				var otherErr error
				output, otherErr = executeIntegrationForAllPods(c, ksmPod, podsList, logger)
				if otherErr != nil {
					return otherErr
				}
				return err
			}
			return nil
		},
		retry.OnRetry(func(err error) {
			logger.Debugf("Retrying due to: %s", err)
		}),
	)
	if err != nil {
		execErr.errs = append(execErr.errs, fmt.Errorf("failure during JSON schema validation, %s", err))
	}
	if len(execErr.errs) > 0 {
		return execErr
	}
	return nil
}

func executeIntegrationForAllPods(c *k8s.Client, ksmPod *v1.Pod, nrPods *v1.PodList, logger *logrus.Logger) (map[string]integrationData, error) {
	output := make(map[string]integrationData)
	dataChannel := make(chan integrationData)

	var wg sync.WaitGroup
	wg.Add(len(nrPods.Items))
	go func() {
		wg.Wait()
		close(dataChannel)
	}()

	for _, p := range nrPods.Items {
		logger.Debugf("Executing integration inside pod: %s", p.Name)
		go execIntegration(p, ksmPod, dataChannel, &wg, c, logger)
	}

	for d := range dataChannel {
		if d.err != nil {
			return output, fmt.Errorf("pod: %s. %s", d.podName, d.err.Error())
		}
		output[d.podName] = d
	}
	return output, nil
}

func testSpecificEntities(output map[string]integrationData, releaseName string) error {
	entitySchemas := eventTypeSchemasPerEntity{
		entityID(fmt.Sprintf("k8s:%s:%s:volume:%s", cliArgs.ClusterName, namespace, fmt.Sprintf("default_busybox-%s_busybox-persistent-storage", releaseName))): {
			"K8sVolumeSample": "persistentvolume.json",
		},
	}
	foundEntities := make(map[entityID]error)
	for _, o := range output {
		var i sdk.IntegrationProtocol2
		err := json.Unmarshal(o.stdOut, &i)
		if err != nil {
			return err
		}
		for eid, s := range entitySchemas {
			entityData, err := i.Entity(eid.Name(), eid.Type())
			if err != nil {
				return err
			}
			if len(entityData.Metrics) > 0 {
				jobEventTypeSchema := map[string]jsonschema.EventTypeToSchemaFilename{
					"dummy": s,
				}
				foundEntities[eid] = jsonschema.MatchEntities([]*sdk.EntityData{entityData}, jobEventTypeSchema, cliArgs.SchemasDirectory)
			}
		}
	}
	var execErr executionErr
	for eid := range entitySchemas {
		if _, ok := foundEntities[eid]; !ok {
			execErr.errs = append(execErr.errs, fmt.Errorf("expected entity %s not found", eid))
		}
	}
	if len(execErr.errs) > 0 {
		return execErr
	}
	return nil
}

func testRoles(nodeCount int, integrationOutput map[string]integrationData) error {
	jobRunCount := map[job]int{}

	for podName, output := range integrationOutput {
		for _, job := range output.expectedJobs {
			jobRunCount[job]++
			stderr := string(output.stdErr)
			found := strings.Count(stderr, fmt.Sprintf("Running job: %s", job)) > 0
			if !found {
				return fmt.Errorf("cannot find job %s for pod %s", job, podName)
			}
		}
	}

	count, ok := jobRunCount[jobKSM]
	if !ok || count != 1 {
		return fmt.Errorf("expected exactly 1 KSM job to run, foud %d", count)
	}

	count, ok = jobRunCount[jobKubelet]
	if !ok || count != nodeCount {
		return fmt.Errorf(
			"expected %d kubelet jobs to run, got %d",
			nodeCount,
			count,
		)
	}
	return nil
}

var eventTypeSchemas = map[string]jsonschema.EventTypeToSchemaFilename{
	"kube-state-metrics": {
		"K8sReplicasetSample": "replicaset.json",
		"K8sNamespaceSample":  "namespace.json",
		"K8sDeploymentSample": "deployment.json",
	},
	"kubelet": {
		"K8sPodSample":       "pod.json",
		"K8sContainerSample": "container.json",
		"K8sNodeSample":      "node.json",
		"K8sVolumeSample":    "volume.json",
		"K8sClusterSample":   "cluster.json",
	},
	"scheduler": {
		"K8sSchedulerSample": "scheduler.json",
	},
	"etcd": {
		"K8sEtcdSample": "etcd.json",
	},
	"controller-manager": {
		"K8sControllerManagerSample": "controller-manager.json",
	},
	"api-server": {
		"K8sApiServerSample": "apiserver.json",
	},
}

func testEventTypes(output map[string]integrationData) error {
	for podName, o := range output {
		schemasToMatch := make(map[string]jsonschema.EventTypeToSchemaFilename)
		for _, expectedJob := range o.expectedJobs {
			logrus.Printf("Job: %s, types: %#v", expectedJob, eventTypeSchemas[string(expectedJob)])
			schemasToMatch[string(expectedJob)] = eventTypeSchemas[string(expectedJob)]
		}

		i := sdk.IntegrationProtocol2{}
		err := json.Unmarshal(o.stdOut, &i)
		if err != nil {
			return err
		}
		err = jsonschema.MatchIntegration(&i)
		if err != nil {
			return fmt.Errorf("pod %s failed with: %s", podName, err)
		}

		err = jsonschema.MatchEntities(i.Data, schemasToMatch, cliArgs.SchemasDirectory)
		if err != nil {
			return fmt.Errorf("pod %s failed with: %s", podName, err)
		}
	}
	return nil
}

func installRelease(_ context.Context, scenario string, logger *logrus.Logger) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	options := strings.Split(scenario, ",")
	options = append(options,
		fmt.Sprintf("integration.k8sClusterName=%s", cliArgs.ClusterName),
		fmt.Sprintf("integration.newRelicLicenseKey=%s", cliArgs.NrLicenseKey),
		"integration.verbose=true",
		fmt.Sprintf("integration.collectorURL=%s", cliArgs.CollectorURL),
	)

	o, err := helm.InstallRelease(filepath.Join(dir, cliArgs.NrChartPath), cliArgs.Context, logger, options...)
	if err != nil {
		return "", err
	}

	r := bufio.NewReader(bytes.NewReader(o))
	v, _, err := r.ReadLine()
	if err != nil {
		return "", err
	}

	releaseName := bytes.TrimPrefix(v, []byte("NAME:   "))

	return string(releaseName), nil
}
