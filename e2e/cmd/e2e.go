package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	_ "github.com/newrelic/nri-kubernetes/v2/e2e/gcp"
	"github.com/newrelic/nri-kubernetes/v2/e2e/helm"
	"github.com/newrelic/nri-kubernetes/v2/e2e/jsonschema"
	"github.com/newrelic/nri-kubernetes/v2/e2e/k8s"
	"github.com/newrelic/nri-kubernetes/v2/e2e/retry"
	"github.com/newrelic/nri-kubernetes/v2/e2e/scenario"
	"github.com/newrelic/nri-kubernetes/v2/e2e/timer"
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
	nrLabel        = "name=newrelic-infra"
	namespace      = "default"
	nrContainer    = "newrelic-infra"
	ksmLabel       = "app.kubernetes.io/name=kube-state-metrics"
	minikubeFlavor = "Minikube"
	unknownFlavor  = "Unknown"
)

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

func (se *scenarioEnv) execIntegration(pod v1.Pod, ksmPod *v1.Pod) (*integrationData, error) {
	defer timer.Track(time.Now(), fmt.Sprintf("execIntegration func for pod %s", pod.Name), se.logger)

	d := &integrationData{
		podName: pod.Name,
	}

	args := []string{
		"/var/db/newrelic-infra/newrelic-integrations/bin/nri-kubernetes",
		"-timeout=30000",
		"-verbose",
	}

	args = append(args, se.scenario.ExtraArgs...)

	output, err := se.k8sClient.PodExec(namespace, pod.Name, nrContainer, args...)
	if err != nil {
		return nil, fmt.Errorf("executing command inside pod: %w", err)
	}

	d.stdOut = output.Stdout.Bytes()
	d.stdErr = output.Stderr.Bytes()

	for _, j := range allJobs {
		expectedStr := fmt.Sprintf("Running job: %s", string(j))
		if strings.Contains(string(d.stdErr), expectedStr) {
			d.expectedJobs = append(d.expectedJobs, j)
		}
	}

	se.logger.Printf("Pod: %s, hostIP %s, expected jobs: %#v", pod.Name, pod.Status.HostIP, d.expectedJobs)

	return d, nil
}

type testEnv struct {
	k8sClient *k8s.Client
	logger    *logrus.Logger
}

type scenarioEnv struct {
	testEnv
	scenario scenario.Scenario
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

	k8sClient, err := k8s.NewClient(cliArgs.Context)
	if err != nil {
		panic(err.Error())
	}

	testEnv := testEnv{
		k8sClient: k8sClient,
		logger:    logger,
	}

	nodes, err := k8sClient.NodesList()
	if err != nil {
		panic(err.Error())
	}

	// If there is more than one node on the cluster, some metrics may not be found, which makes tests to fail.
	expectedNumberOfNodes := 1

	if nodesCount := len(nodes.Items); nodesCount != expectedNumberOfNodes {
		testEnv.logger.Fatalf("e2e tests require %d number of nodes on the cluster, found %d", expectedNumberOfNodes, nodesCount)
	}

	testEnv.logger.Infof("Executing tests in %q cluster. K8s version: %s", k8sClient.Config.Host, k8sClient.ServerVersion())

	if cliArgs.CleanBeforeRun {
		testEnv.logger.Infof("Cleaning cluster")
		err := helm.DeleteAllReleases(cliArgs.Context, testEnv.logger)
		if err != nil {
			panic(err.Error())
		}
	}

	minikubeHost := testEnv.determineMinikubeHost()
	clusterFlavor := unknownFlavor
	if strings.Contains(k8sClient.Config.Host, minikubeHost) {
		clusterFlavor = minikubeFlavor
	}

	// TODO
	var errs []error
	scenarios := map[string]func(*scenario.Scenario){
		"latest_single_instance": func(s *scenario.Scenario) {},
		"latest_but_one_single_instance": func(s *scenario.Scenario) {
			s.KSMVersion = "v1.9.7"
			s.KSMImageRepository = "quay.io/coreos/kube-state-metrics"
		},
		"multiple_instances": func(s *scenario.Scenario) {
			// the behaviour for multiple KSMs only has to be tested for one version, because it's testing our logic,
			// not the logic of KSM. This might change if KSM sharding becomes enabled by default.
			s.TwoKSMInstances = true
		},
		"with_namespace_filtering": func(s *scenario.Scenario) {
			s.ExtraArgs = []string{"-kube_state_metrics_namespace=default"}
		},
	}

	for scenarioName, mutateTestScenario := range scenarios {
		testScenario := scenario.Scenario{
			Unprivileged:               cliArgs.Unprivileged,
			RBAC:                       cliArgs.Rbac,
			KSMVersion:                 "v1.9.8",
			KSMImageRepository:         "k8s.gcr.io/kube-state-metrics/kube-state-metrics",
			IntegrationImageRepository: cliArgs.IntegrationImageRepository,
			IntegrationImageTag:        cliArgs.IntegrationImageTag,
			ClusterFlavor:              clusterFlavor,
			K8sServerInfo:              k8sClient.ServerVersionInfo,
		}

		mutateTestScenario(&testScenario)

		testEnv.logger.Infof("#####################")
		testEnv.logger.Infof("Scenario %q: %s", scenarioName, testScenario)
		testEnv.logger.Infof("#####################")

		se := scenarioEnv{
			testEnv:  testEnv,
			scenario: testScenario,
		}

		if err := se.execute(); err != nil {
			if cliArgs.FailFast {
				testEnv.logger.Info("Finishing execution because 'FailFast' is true")
				testEnv.logger.Infof("Ran with the following configuration: %s", testScenario)

				testEnv.logger.Fatal(err.Error())
			}
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		testEnv.logger.Debugf("errors collected from all scenarios")
		for _, err := range errs {
			testEnv.logger.Errorf(err.Error())
		}
		testEnv.logger.Fatal("Error Detected")
	}

	testEnv.logger.Infof("OK")
}

func (e *testEnv) determineMinikubeHost() string {
	cmd := exec.Command("minikube", "ip")
	var out bytes.Buffer

	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		e.logger.Infof("Could not determine Minikube host: %v", err)
		return "https://this-will-never-be-the-minikube-host.com"
	}

	return strings.TrimSpace(out.String())
}

func (e *testEnv) waitForKSM() (*v1.Pod, error) {
	defer timer.Track(time.Now(), "waitForKSM", e.logger)
	var foundPod v1.Pod
	err := retry.Do(
		func() error {
			ksmPodList, err := e.k8sClient.PodsListByLabels(namespace, []string{ksmLabel})
			if err != nil {
				return err
			}
			if len(ksmPodList.Items) != 0 && ksmPodList.Items[0].Status.Phase == "Running" {
				for _, con := range ksmPodList.Items[0].Status.Conditions {
					e.logger.Debugf("Waiting for kube-state-metrics pod to be ready, current condition: %s - %s", con.Type, con.Status)

					if con.Type == "Ready" && con.Status == "True" {
						foundPod = ksmPodList.Items[0]
						return nil
					}
				}
			}
			return fmt.Errorf("kube-state-metrics is not ready yet")
		},
		retry.OnRetry(func(err error) {
			e.logger.Debugf("Retrying due to: %s", err)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("kube-state-metrics pod is not ready: %s", err)
	}
	return &foundPod, nil
}

func (se *scenarioEnv) execute() error {
	defer timer.Track(time.Now(), fmt.Sprintf("execute func for %s", se.scenario), se.logger)

	releaseName, err := se.installRelease()
	if err != nil {
		return err
	}

	defer func() {
		se.logger.Infof("deleting release %s", releaseName)
		err = helm.DeleteRelease(releaseName, cliArgs.Context, se.logger)
		if err != nil {
			se.logger.Errorf("error while deleting release %q", err)
		}
	}()

	// At least one of kube-state-metrics pods needs to be ready to enter to the newrelic-infra pod and execute the integration.
	// If the kube-state-metrics pod is not ready, then metrics from replicaset, namespace and deployment will not be populate and JSON schemas will fail.
	ksmPod, err := se.waitForKSM()
	if err != nil {
		return err
	}
	return se.executeTests(ksmPod, releaseName)
}

func (se *scenarioEnv) executeTests(ksmPod *v1.Pod, releaseName string) error {
	releaseLabel := fmt.Sprintf("releaseName=%s", releaseName)
	// We're fetching the list of NR pods here just to fetch it once. If for
	// some reason this list or the contents of it could change during the
	// execution of these tests, we could move it to `test*` functions.
	podsList, err := se.k8sClient.PodsListByLabels(namespace, []string{nrLabel, releaseLabel})
	if err != nil {
		return err
	}

	se.logger.Info("Found the following pods for test execution:")
	for _, pod := range podsList.Items {
		se.logger.Infof("[%s] status: %s %s", pod.Name, pod.Status.Message, pod.Status.Reason)
	}

	nodes, err := se.k8sClient.NodesList()
	if err != nil {
		return fmt.Errorf("error getting the list of nodes in the cluster: %s", err)
	}
	output, err := se.executeIntegrationForAllPods(ksmPod, podsList)
	if err != nil {
		return err
	}
	var execErr executionErr
	se.logger.Info("checking if the integrations are executed with the proper roles")
	err = testRoles(len(nodes.Items), output)
	if err != nil {
		execErr.errs = append(execErr.errs, err)
	}

	if se.scenario.ClusterFlavor == minikubeFlavor {
		// Minikube use hostPath provisioner for PVCs, which makes kubelet to not report PVC volumes in /stats/summary
		// See https://github.com/yashbhutwala/kubectl-df-pv/issues/2 for more info.
		se.logger.Info("Skipping `testSpecificEntities` because you're running them in Minikube.")
	} else {
		se.logger.Info("checking if specific entities match our JSON schemas")
		err = retry.Do(
			func() error {
				err := testSpecificEntities(output, releaseName)
				if err != nil {
					var otherErr error
					output, otherErr = se.executeIntegrationForAllPods(ksmPod, podsList)
					if otherErr != nil {
						return otherErr
					}
					return err
				}
				se.logger.Debugf("The test 'checking if specific entities match our JSON schemas' succeeded")
				return nil
			},
			retry.OnRetry(func(err error) {
				se.logger.Debugf("Retrying, the error might be caused by a not ready environment. Scenario: %s", se.scenario)
			}),
		)
		if err != nil {
			execErr.errs = append(execErr.errs, err)
		}
	}
	se.logger.Info("checking if the metric sets in all integrations match our JSON schemas")
	err = retry.Do(
		func() error {
			err := se.testEventTypes(output)
			if err != nil {
				var otherErr error
				output, otherErr = se.executeIntegrationForAllPods(ksmPod, podsList)
				if otherErr != nil {
					return otherErr
				}
				return err
			}
			se.logger.Debugf("Retrying, the error might be caused by a not ready environment. Scenario: %s", se.scenario)
			return nil
		},
		retry.OnRetry(func(err error) {
			se.logger.Debugf("Retrying since the error might be caused by the environment not being ready yet")
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

func (se *scenarioEnv) executeIntegrationForAllPods(ksmPod *v1.Pod, nrPods *v1.PodList) (map[string]*integrationData, error) {
	output := map[string]*integrationData{}

	for _, p := range nrPods.Items {
		se.logger.Debugf("Executing integration inside pod: %s", p.Name)

		integrationData, err := se.execIntegration(p, ksmPod)
		if err != nil {
			return output, fmt.Errorf("pod %q: %w", p.Name, err)
		}

		output[p.Name] = integrationData
	}

	return output, nil
}

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

func testSpecificEntities(output map[string]*integrationData, releaseName string) error {
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

func testRoles(nodeCount int, integrationOutput map[string]*integrationData) error {
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
		return fmt.Errorf("expected exactly 1 KSM job to run, found %d", count)
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

func (se *scenarioEnv) testEventTypes(output map[string]*integrationData) error {
	for podName, o := range output {
		schemasToMatch := make(map[string]jsonschema.EventTypeToSchemaFilename)
		for _, expectedJob := range o.expectedJobs {
			expectedSchema := se.scenario.GetSchemasForJob(string(expectedJob))
			logrus.Printf("Job: %s, types: %#v", expectedJob, expectedSchema)
			schemasToMatch[string(expectedJob)] = expectedSchema
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

func (se *scenarioEnv) installRelease() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	options := se.scenario.HelmValues()
	options = append(options,
		fmt.Sprintf("integration.k8sClusterName=%s", cliArgs.ClusterName),
		fmt.Sprintf("integration.newRelicLicenseKey=%s", cliArgs.NrLicenseKey),
		"integration.verbose=true",
		fmt.Sprintf("integration.collectorURL=%s", cliArgs.CollectorURL),
		fmt.Sprintf("daemonset.clusterFlavor=%s", se.scenario.ClusterFlavor),
	)

	releaseName := fmt.Sprintf("%s-%s", "release", rand.String(5))
	err = helm.InstallRelease(releaseName, filepath.Join(dir, cliArgs.NrChartPath), cliArgs.Context, se.logger, options...)
	if err != nil {
		return "", err
	}

	return releaseName, nil
}
