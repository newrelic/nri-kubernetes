package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/version"

	_ "github.com/newrelic/nri-kubernetes/e2e/gcp"
	"github.com/newrelic/nri-kubernetes/e2e/helm"
	"github.com/newrelic/nri-kubernetes/e2e/jsonschema"
	"github.com/newrelic/nri-kubernetes/e2e/k8s"
	"github.com/newrelic/nri-kubernetes/e2e/retry"
	"github.com/newrelic/nri-kubernetes/e2e/scenario"
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
	K8sVersion                 string `default:"v1.19.3" help:"SetK8s version, currently used for endpoints"`
}{}

const (
	nrLabel        = "name=newrelic-infra"
	namespace      = "default"
	nrContainer    = "newrelic-infra"
	ksmLabel       = "app.kubernetes.io/name=kube-state-metrics"
	minikubeFlavor = "Minikube"
	unknownFlavor  = "Unknown"
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

func generateScenarios(
	integrationImageRepository string,
	integrationImageTag string,
	rbac bool,
	unprivileged bool,
	serverInfo *version.Info,
	clusterFlavor string,
	k8sVersion string,
) []scenario.Scenario {
	return []scenario.Scenario{
		// 2 latest versions, single KSM instance
		scenario.New(rbac, unprivileged, integrationImageRepository, integrationImageTag, "v1.9.7", false, serverInfo, clusterFlavor, k8sVersion),
		scenario.New(rbac, unprivileged, integrationImageRepository, integrationImageTag, "v1.9.8", false, serverInfo, clusterFlavor, k8sVersion),

		// the behaviour for multiple KSMs only has to be tested for one version, because it's testing our logic,
		// not the logic of KSM. This might change if KSM sharding becomes enabled by default.
		scenario.New(rbac, unprivileged, integrationImageRepository, integrationImageTag, "v1.9.8", true, serverInfo, clusterFlavor, k8sVersion),
	}
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

	output, err := c.PodExec(namespace, pod.Name, nrContainer, "/var/db/newrelic-infra/newrelic-integrations/bin/nri-kubernetes", "-timeout=30000", "-verbose")
	if err != nil {
		d.err = err
		logger.Debugf("Error detecting running pod exec: %s", d.err.Error())
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

	if cliArgs.CleanBeforeRun {
		logger.Infof("Cleaning cluster")
		err := helm.DeleteAllReleases(cliArgs.Context, logger)
		if err != nil {
			panic(err.Error())
		}
	}

	minikubeHost := determineMinikubeHost(logger)
	clusterFlavor := unknownFlavor
	if strings.Contains(c.Config.Host, minikubeHost) {
		clusterFlavor = minikubeFlavor
	}

	// TODO
	var errs []error
	ctx := context.TODO()
	scenarios := generateScenarios(
		cliArgs.IntegrationImageRepository,
		cliArgs.IntegrationImageTag,
		cliArgs.Rbac,
		cliArgs.Unprivileged,
		c.ServerVersionInfo,
		clusterFlavor,
		cliArgs.K8sVersion,
	)
	for _, s := range scenarios {
		logger.Infof("#####################")
		logger.Infof("Scenario: %q", s)
		logger.Infof("#####################")

		err := executeScenario(ctx, s, c, logger)
		if err != nil {
			if cliArgs.FailFast {
				logger.Info("Finishing execution because 'FailFast' is true")
				logger.Infof("Ran with the following configuration: %s", s)

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
		logger.Fatal("Error Detected")
	} else {
		logger.Infof("OK")
	}
}

func determineMinikubeHost(logger *logrus.Logger) string {
	cmd := exec.Command("minikube", "ip")
	var out bytes.Buffer

	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		logger.Infof("Could not determine Minikube host: %v", err)
		return "https://this-will-never-be-the-minikube-host.com"
	}

	return strings.TrimSpace(out.String())
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

func executeScenario(
	ctx context.Context,
	currentScenario scenario.Scenario,
	c *k8s.Client,
	logger *logrus.Logger,
) error {
	defer timer.Track(time.Now(), fmt.Sprintf("executeScenario func for %s", currentScenario), logger)

	releaseName, err := installRelease(ctx, currentScenario, logger)
	if err != nil {
		return err
	}

	defer func() {
		logger.Infof("deleting release %s", releaseName)
		err = helm.DeleteRelease(releaseName, cliArgs.Context, logger)
		if err != nil {
			logger.Errorf("error while deleting release %q", err)
		}
	}()

	// At least one of kube-state-metrics pods needs to be ready to enter to the newrelic-infra pod and execute the integration.
	// If the kube-state-metrics pod is not ready, then metrics from replicaset, namespace and deployment will not be populate and JSON schemas will fail.
	ksmPod, err := waitForKSM(c, logger)
	if err != nil {
		return err
	}
	return executeTests(c, ksmPod, releaseName, logger, currentScenario)
}

func executeTests(
	c *k8s.Client,
	ksmPod *v1.Pod,
	releaseName string,
	logger *logrus.Logger,
	currentScenario scenario.Scenario,
) error {

	releaseLabel := fmt.Sprintf("releaseName=%s", releaseName)
	// We're fetching the list of NR pods here just to fetch it once. If for
	// some reason this list or the contents of it could change during the
	// execution of these tests, we could move it to `test*` functions.
	podsList, err := c.PodsListByLabels(namespace, []string{nrLabel, releaseLabel})
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

	if currentScenario.ClusterFlavor == minikubeFlavor {
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
				logger.Debugf("The test 'checking if specific entities match our JSON schemas' succeeded")
				return nil
			},
			retry.OnRetry(func(err error) {
				logger.Debugf("Retrying, the error might be caused by a not ready environment. Scenario: %s", currentScenario)
			}),
		)
		if err != nil {
			execErr.errs = append(execErr.errs, err)
		}
	}
	logger.Info("checking if the metric sets in all integrations match our JSON schemas")
	err = retry.Do(
		func() error {
			err := testEventTypes(output, currentScenario)
			if err != nil {
				var otherErr error
				output, otherErr = executeIntegrationForAllPods(c, ksmPod, podsList, logger)
				if otherErr != nil {
					return otherErr
				}
				return err
			}
			logger.Debugf("Retrying, the error might be caused by a not ready environment. Scenario: %s", currentScenario)
			return nil
		},
		retry.OnRetry(func(err error) {
			logger.Debugf("Retrying since the error might be caused by the environment not being ready yet")
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

func testEventTypes(output map[string]integrationData, s scenario.Scenario) error {
	for podName, o := range output {
		schemasToMatch := make(map[string]jsonschema.EventTypeToSchemaFilename)
		for _, expectedJob := range o.expectedJobs {
			expectedSchema := s.GetSchemasForJob(string(expectedJob))
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

func installRelease(_ context.Context, s scenario.Scenario, logger *logrus.Logger) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	versionSplitted := strings.Split(s.K8sVersion, ".")

	options := strings.Split(s.String(), ",")
	options = append(options,
		fmt.Sprintf("integration.k8sClusterName=%s", cliArgs.ClusterName),
		fmt.Sprintf("integration.newRelicLicenseKey=%s", cliArgs.NrLicenseKey),
		"integration.verbose=true",
		fmt.Sprintf("integration.collectorURL=%s", cliArgs.CollectorURL),
		fmt.Sprintf("daemonset.clusterFlavor=%s", s.ClusterFlavor),
		fmt.Sprintf("daemonset.clusterVersion=%s.%s.x", versionSplitted[0], versionSplitted[1]),
	)

	releaseName := fmt.Sprintf("%s-%s", "release", rand.String(5))
	err = helm.InstallRelease(releaseName, filepath.Join(dir, cliArgs.NrChartPath), cliArgs.Context, logger, options...)
	if err != nil {
		return "", err
	}

	return releaseName, nil
}
