# OpenShift E2E Test Scripts

Scripts for setting up and running E2E tests on OpenShift Local (CRC) clusters.

## Important Notice

**FOR TESTING PURPOSES ONLY**

These scripts are provided as-is for development and testing workflows. They come with **NO GUARANTEES** and may break at any time due to:
- Updates to nri-kubernetes chart or integration code
- Changes to OpenShift/CRC versions or APIs
- Modifications to helm chart structures
- Security policy changes

Use at your own risk. These scripts are not officially supported and are subject to change without notice.

---

## Requirements

### Files
- **pull-secret.txt**: Must be placed in this `openshift/` folder
  - Download from: https://console.redhat.com/openshift/create/local
  - Required for `crc_setup.sh` to authenticate with Red Hat registries

### Environment Variables
Export these before running E2E tests:

```bash
export LICENSE_KEY=your-new-relic-license-key
export ACCOUNT_ID=your-new-relic-account-id
export API_KEY=your-new-relic-api-key
export EXCEPTIONS_SOURCE_FILE=e2e/1_32-exceptions-openshift.yml
```

### Prerequisites
- [Red Hat OpenShift Local](https://developers.redhat.com/products/openshift-local/overview) installed
- Docker (macOS) or Podman (Linux)
- kubectl/oc CLI tools
- Helm 3+
- Go (for running E2E tests)

---

## Scripts

### `crc_setup.sh`

Automates OpenShift Local cluster setup with optimized configuration.

**What it does:**
1. Runs `crc setup` with preset for OpenShift
2. Enables cluster monitoring
3. Starts CRC with initial configuration
4. Reconfigures cluster resources:
   - 8 CPUs
   - 32GB memory
   - 90GB disk
5. Restarts with new configuration
6. Extracts and saves cluster credentials to `crc-credentials.txt`
7. Exports credentials as environment variables

**Usage:**
```bash
# From repo root or from openshift folder
./openshift/crc_setup.sh
```

**Output:**
- `crc-credentials.txt`: Contains admin and developer credentials for reference if needed
- `crc-output.txt`: Full CRC startup log
- Environment variables: `OC_ADMIN_USER`, `OC_ADMIN_PASS`, `OC_DEV_USER`, `OC_DEV_PASS`

**Important Notes:**
- Some operations may require `sudo` privileges (certificate trust, registry login)
- On macOS, you'll be prompted for your password when adding the registry certificate to the system keychain

---

### `run.sh`

Interactive menu-driven script for managing OpenShift E2E test workflows.

**Platform Support:**
- Linux (uses `podman`)
- macOS (uses `docker`)

**Usage:**
- select the kubernetes context that you just installed the openshift local cluster in
```bash
./openshift/run.sh
```

#### Menu Options

**Quick Test (Online Images):**
- **Option 1**: Run online-based scenario workflow
  - Tests with published images from registries (no code changes)
  - Sets up mTLS for etcd, configures E2E values, adds SCCs, runs tests

**Setup Functions** (one-time cluster configuration):
- **Option 2**: Add registry roles to OpenShift users
- **Option 3**: Expose default registry for external access
- **Option 4**: Run setup workflow (options 2-3 combined)

**Scenario Functions** (for testing code changes):
- **Option 5**: Build image (compile and build Docker image)
- **Option 6**: Push image to OpenShift internal registry
- **Option 7**: Setup mTLS for etcd
- **Option 8**: Configure and run E2E tests
- **Option 9**: Run scenario workflow (options 5-8 combined)

**Development Functions** (flexible namespace/release workflow):
- **Option 10**: Setup mTLS for etcd (dev)
- **Option 11**: Create e2e-values file (dev)
- **Option 12**: Deploy E2E resources (dev) - deploys KSM and test pods
- **Option 13**: Uninstall E2E resources (dev) - removes resources and namespace
- **Option 14**: Build image (dev) - compile and build Docker image locally
- **Option 15**: Push image (dev) - push to OpenShift internal registry
- **Option 16**: Deploy nri-kubernetes (dev) - deploys integration with custom values
- **Option 17**: Run E2E tests (dev)

---

## Typical Workflows

### Quick Test with Online Images
```bash
./openshift/run.sh
# Select option 1
# Enter scenario tag: test1
# Tests run with published images (no local code changes)
```

### Full E2E Test with Code Changes
```bash
./openshift/run.sh
# 1. First time only: Setup cluster
# Select option 4 (Run setup workflow)

# 2. Run complete scenario workflow
# Select option 9 (Run scenario workflow)
# Enter scenario tag: my-changes
# Builds image, pushes to registry, configures, and runs tests
```

### Development/Iterative Testing

**Workflow 1: Testing with Custom Images**
```bash
./openshift/run.sh

# 1. First time: Setup cluster (option 4)

# 2. Build and push your changes
# Select option 14 (Build image)
# Select option 15 (Push image)
# Enter namespace: my-dev-ns

# 3. Deploy E2E resources
# Select option 12 (Deploy E2E resources)
# Uses remembered namespace from step 2
# Enter release name: my-release

# 4. Deploy your code
# Select option 16 (Deploy nri-kubernetes)
# Uses remembered namespace/release
# Enter values file: e2e/e2e-values-openshift.yml

# 5. Configure for testing
# Select option 10 (Setup mTLS for etcd)
# Select option 11 (Create e2e-values file)

# 6. Run tests
# Select option 17 (Run E2E tests)
```

**Workflow 2: Testing with Online Images**
```bash
./openshift/run.sh

# 1. First time: Setup cluster (option 4)

# 2. Deploy E2E resources
# Select option 12 (Deploy E2E resources)
# Enter namespace: my-dev-ns
# Enter release name: my-release

# 3. Deploy nri-kubernetes with online images
# Select option 16 (Deploy nri-kubernetes)
# Uses remembered namespace/release from step 2
# Enter values file: /path/to/custom-values.yaml

# 4. Configure for testing
# Select option 10 (Setup mTLS for etcd)
# Select option 11 (Create e2e-values file)

# 5. Run tests
# Select option 17 (Run E2E tests)
```

---

## Files Generated

- `openshift-env-vars.txt`: Registry and scenario configuration
- `crc-credentials.txt`: Cluster credentials
- `crc-output.txt`: CRC startup logs
- `../e2e/e2e-values-openshift.yml`: nri-kubernetes values file used for openshift during e2e-tests
- `../etcd-secret.yaml`: mTLS secret for etcd (temporary)

---

## Troubleshooting

### Registry Login Issues
```bash
# Get registry host
oc get route default-route -n openshift-image-registry

# Login manually
docker login -u kubeadmin -p $(oc whoami -t) <registry-host>
```

### SCC Issues
```bash
# Check if SCCs are applied
oc get scc privileged -o yaml | grep -A 5 "users:"

# Manually add SCC to service account
oc adm policy add-scc-to-user privileged system:serviceaccount:<namespace>:<service-account>


# Check if service account has SCC added
oc adm policy add-scc-to-user privileged -z nri-bundle-sa -n <namespace>
```



## Notes

- The development workflow (options 10-17) remembers your namespace and release name during the same session
- Scenario workflows use the naming convention `nr-${scenario_tag}` for namespaces
- Development workflows support custom namespaces (without `nr-` prefix requirement)
- All service accounts automatically receive `privileged` SCC when using these scripts
- Registry setup (options 2-3) only needs to be run once per cluster
