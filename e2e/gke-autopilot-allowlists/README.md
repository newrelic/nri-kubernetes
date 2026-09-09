# GKE Autopilot allowlist confirmation (manual e2e)

Manual end-to-end check that a New Relic `WorkloadAllowlist` actually admits the privileged
nri-kubernetes workload on GKE Autopilot and that the expected node/host metrics show up. This is
**not** wired into CI. Run it by hand against an existing GKE Autopilot cluster.

## What is here

| File | Purpose |
|---|---|
| `newrelic-nri-kubernetes-node-scoped-hostnet-on.workloadallowlist.yaml` | Fixture CR, node-scoped shape (`hostNetwork: true`). |
| `newrelic-nri-kubernetes-pod-scoped-hostnet-off.workloadallowlist.yaml` | Fixture CR, pod-scoped shape (`hostNetwork: false`). |
| `.env.example` | Template for the runner config (copy to `.env`, which is gitignored). |
| `../test-specs-gke-autopilot.yml` | The two scenarios and their metric assertions. |
| `../e2e-values-gke-autopilot-node.yml` / `-pod.yml` | Helm values per shape (kubelet DaemonSet only; ksm + controlPlane disabled). |
| `../run-gke-autopilot-e2e.sh` | The runner. |

These two CRs are the New Relic `WorkloadAllowlist` submission candidates for GKE Autopilot — the same
manifests submitted to Google. Keep them in sync with what is submitted: they are both the test
fixtures here and the source a drift-check would diff against.

## Image source: released vs local

The runner asks which images the workload should run (`IMAGE_SOURCE`):

- **`released`** (default): the published `newrelic/nri-kubernetes` image (chart default). No registry,
  no build, no push. Confirms the allowlist against the shipped artifact. This is the quickest path.
- **`local`**: build the integration image from local source and push it to a registry the cluster
  can pull, then run that. Confirms your source/chart changes. Requires `REGISTRY` + docker auth. The
  image is pushed to a path ending in `newrelic/nri-kubernetes` so the CR image regex still matches.

## Prerequisites

- An existing GKE Autopilot cluster you can reach (the runner defaults to your active kube-context and
  only verifies it, never switches). The `WorkloadAllowlist` CRD only exists on Autopilot, and applying
  a CR directly requires a "blessed" project.
- A New Relic production account: `ACCOUNT_ID`, a USER API key, an INGEST license key.
- `helm`, `kubectl`, `go`. **Local mode only:** `docker`, `make compile-multiarch`, and a container
  registry the cluster can pull from (Artifact Registry).

## Run it

```bash
bash e2e/run-gke-autopilot-e2e.sh
```

The runner prompts for anything it needs (context, registry, tag, account id, keys), shows sensible
defaults, and offers to save your answers to a gitignored `.env` so later runs are non-interactive.
It builds a dev image, pushes it, then runs both scenarios. It never switches your kube-context or
docker/gcloud identity: if the context is wrong or the push is unauthenticated, it prints the exact
command for you to run and exits.

## What each scenario asserts

Both scenarios deploy only the kubelet DaemonSet and assert:

- `k8s.node.*`, `k8s.pod.*`, `k8s.container.*` are present (the agent is scraping the kubelet).
- The host layer is present (`SystemSample` / `ProcessSample`) — the value the allowlist unlocks.

Shape-specific:

- **node** (`hostNetwork: true`): `NetworkSample` reports multiple interfaces (node `eth0` plus
  `cilium_*` / `lxc*`), so network metrics are node-scoped.
- **pod** (`hostNetwork: false`): `NetworkSample` is present but scoped to the agent pod's `eth0`.

The e2e assertion schema has a lower bound but no upper bound, so the node-vs-pod distinction is an
observational check: compare the `net ifaces` value logged by each scenario. Node should be clearly
higher than pod.

## Host-layer query model

The host layer lands as infra-agent **events** — `SystemSample` / `ProcessSample` / `NetworkSample`,
scoped by `clusterName` — **not** as `host.*` Metric names (`metricName LIKE 'host.%'` is empty). The
scenarios assert those event types accordingly. If you re-run against a different cluster or agent
version, sanity-check that these events populate for your `clusterName`.

## Negative check (manual)

To prove the CR is what admits the workload, delete the CR and redeploy the same values: Warden
should deny the privileged pod.

```bash
# released images (default). For a local dev image, append the same --set images.integration.* flags
# the runner uses.
kubectl delete -f gke-autopilot-allowlists/newrelic-nri-kubernetes-node-scoped-hostnet-on.workloadallowlist.yaml
helm upgrade --install neg --namespace nr-neg --create-namespace ../charts/newrelic-infrastructure \
  --values e2e-values-gke-autopilot-node.yml --set global.licenseKey=$LICENSE_KEY --set global.cluster=neg
kubectl get pods -n nr-neg   # expect the kubelet pod blocked by Warden
```
