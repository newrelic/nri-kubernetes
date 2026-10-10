package main

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	internalconfig "github.com/newrelic/nri-kubernetes/v3/internal/config"
	"github.com/newrelic/nri-kubernetes/v3/internal/discovery"
	"github.com/newrelic/nri-kubernetes/v3/src/discover"
	"github.com/newrelic/nri-kubernetes/v3/src/integration"
)

const integrationName = "com.newrelic.kubernetes.discover"

const (
	_ = iota
	exitConfig
	exitIntegration
	exitClients
	exitSetup
	exitLoop
)

//nolint:gochecknoglobals // set via ldflags at build time
var (
	integrationVersion = "0.0.0"
	gitCommit          = ""
	buildDate          = ""
)

func main() {
	logger := log.StandardLogger()

	c, err := discover.LoadConfig(discover.DefaultConfigFolderName, discover.DefaultConfigFileName)
	if err != nil {
		logger.Errorf("loading config: %v", err)
		os.Exit(exitConfig)
	}

	if c.Verbose {
		logger.SetLevel(log.DebugLevel)
	}
	if c.LogLevel != "" {
		if level, parseErr := log.ParseLevel(c.LogLevel); parseErr != nil {
			logger.Warnf("parsing log level %q: %v", c.LogLevel, parseErr)
		} else {
			logger.SetLevel(level)
		}
	}

	logger.Infof(
		"New Relic %s integration Version: %s, Platform: %s, GoVersion: %s, GitCommit: %s, BuildDate: %s\n",
		strings.Title(strings.Replace(integrationName, "com.newrelic.", "", 1)), //nolint:staticcheck
		integrationVersion,
		fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		runtime.Version(),
		gitCommit,
		buildDate,
	)

	integrationOptions := []integration.OptionFunc{
		integration.WithLogger(logger),
		integration.WithMetadata(integration.Metadata{
			Name:    integrationName,
			Version: integrationVersion,
		}),
	}

	switch c.Sink.Type {
	case internalconfig.SinkTypeHTTP:
		integrationOptions = append(integrationOptions, integration.WithHTTPSink(c.Sink.HTTP))
	case internalconfig.SinkTypeStdout:
		logger.Warn("sinking to stdout")
	default:
		logger.Errorf("unknown sink type %q", c.Sink.Type)
		os.Exit(exitConfig)
	}

	iw, err := integration.NewWrapper(integrationOptions...)
	if err != nil {
		logger.Errorf("creating integration wrapper: %v", err)
		os.Exit(exitIntegration)
	}

	// Create the sdk.Integration once; reuse across cycles so the delta storer
	// can skip unchanged inventory entries on subsequent runs.
	i, err := iw.Integration()
	if err != nil {
		logger.Errorf("creating integration: %v", err)
		os.Exit(exitIntegration)
	}

	k8sConfig, err := getK8sConfig(c, logger)
	if err != nil {
		logger.Errorf("building k8s config: %v", err)
		os.Exit(exitClients)
	}

	k8s, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		logger.Errorf("building kubernetes client: %v", err)
		os.Exit(exitClients)
	}

	namespaceCache := discovery.NewNamespaceInMemoryStore(logger)
	scraperOpts := []discover.ScraperOpt{discover.WithLogger(logger)}

	if c.NamespaceSelector != nil {
		nsFilter := discovery.NewNamespaceFilter(c.NamespaceSelector, k8s, logger)
		scraperOpts = append(scraperOpts,
			discover.WithFilterer(discovery.NewCachedNamespaceFilter(nsFilter, namespaceCache)),
		)
	}

	scraper, err := discover.NewScraper(c, discover.Providers{K8s: k8s}, scraperOpts...)
	if err != nil {
		logger.Errorf("creating discovery scraper: %v", err)
		os.Exit(exitSetup)
	}

	for {
		start := time.Now()

		if runErr := scraper.Run(i); runErr != nil {
			logger.Errorf("running discovery scraper: %v", runErr)
			os.Exit(exitLoop)
		}

		if pubErr := i.Publish(); pubErr != nil {
			logger.Errorf("publishing integration: %v", pubErr)
			os.Exit(exitLoop)
		}

		namespaceCache.Vacuum()

		elapsed := time.Since(start)
		nextTick := c.Interval - (elapsed % c.Interval)
		if elapsed > c.Interval {
			logger.Warnf("discovery pass took %dms, exceeds configured interval %dms",
				elapsed.Milliseconds(), c.Interval.Milliseconds())
		}
		logger.Debugf("discovery pass took %dms, next in %dms", elapsed.Milliseconds(), nextTick.Milliseconds())
		time.Sleep(nextTick)
	}
}

// getK8sConfig returns in-cluster config when available, falling back to a
// local kubeconfig — mirroring the pattern in cmd/nri-kubernetes/main.go.
func getK8sConfig(c *discover.Config, logger *log.Logger) (*rest.Config, error) {
	cfg, err := rest.InClusterConfig()
	if err == nil {
		return cfg, nil
	}
	logger.Warnf("collecting in-cluster config: %v — falling back to local kubeconfig", err)

	kubeconf := c.KubeconfigPath
	if kubeconf == "" {
		kubeconf = path.Join(homedir.HomeDir(), ".kube", "config")
	}
	return clientcmd.BuildConfigFromFlags("", kubeconf)
}
