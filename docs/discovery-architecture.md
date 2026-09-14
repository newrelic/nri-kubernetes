# nri-k8s-discover: Architecture

Two diagrams: the component overview and the per-cycle sequence.

---

## 1. Component overview — two paths to infra discovery

```mermaid
graph TB
    subgraph cluster["Kubernetes Cluster"]
        api["Kubernetes API Server"]

        subgraph nrns["newrelic namespace"]
            discover["nri-k8s-discover\nDeployment · 1 replica\ncmd/nri-k8s-discover"]
            full["nri-kubernetes\nDaemonSet (full agent)\ncmd/nri-kubernetes"]
            infraagent["NR Infra Agent\nsidecar · HTTP sink"]
        end

        subgraph workloads["application namespaces"]
            redis["Redis\nStatefulSet + PVC"]
            kafka["Kafka\nStatefulSet + PVC"]
            pg["Postgres\nStatefulSet + PVC"]
            app["App pods\nDeployment"]
        end
    end

    subgraph nr["New Relic Platform"]
        events["K8sDiscoveredWorkloadSample\nevents · 60 s interval"]
        kcs["K8sContainerSample\nevents · 15 s interval"]
        inventory["Inventory store\ndelta-compressed"]
        entities["Entity Catalog\nREDISINSTANCE\nPOSTGRESQLINSTANCE …"]
        ng["NerdGraph API"]
    end

    %% --- Path A: discovery agent only (dark / lightweight cluster) ---
    discover -->|"LIST pods · statefulsets · pvcs\nservices · daemonsets · configmaps\n src/discover/scraper.go"| api
    discover -->|"HTTP POST delta"| infraagent
    infraagent --> events
    infraagent --> inventory

    %% --- Path B: full agent already running ---
    full -->|"kubelet + KSM + control-plane\nsrc/kubelet · src/ksm · src/controlplane"| api
    full -->|"HTTP POST"| infraagent
    infraagent --> kcs

    %% --- Gap queries ---
    events -. "FROM K8sDiscoveredWorkloadSample\nWHERE instrumentationStatus\n= 'not_instrumented'" .-> ng
    kcs    -. "FROM K8sContainerSample\nWHERE containerImage\nRLIKE 'redis|kafka|…'" .-> ng
    entities -. "entitySearch type IN\n('REDISINSTANCE' …)" .-> ng

    style discover fill:#0052cc,color:#fff
    style full    fill:#555,color:#fff
    style nr      fill:#f5f5f5
```

**When to use each path**

| | Discovery agent only | Full agent (+ optional discovery agent) |
|---|---|---|
| Cluster has no NR agent yet | ✓ | — |
| Instrumentation gap detection | ✓ `instrumentationStatus` field | NerdGraph entity search |
| Full K8s metrics (CPU/mem/net) | ✗ | ✓ `K8sContainerSample` |
| Monthly ingest (2-node GKE) | ~22 MB | ~3.5 GB |

---

## 2. Discovery cycle sequence — one `Run()` call

```mermaid
sequenceDiagram
    autonumber

    participant main  as main()<br/>cmd/nri-k8s-discover
    participant scraper as Scraper.Run()<br/>src/discover/scraper.go
    participant k8s   as Kubernetes API
    participant cls   as Classify()<br/>src/discover/classifier.go
    participant instr as detectPodInstrumentation()<br/>src/discover/instrumentation.go
    participant sdk   as infra-integrations-sdk
    participant sink  as NR Infra Agent<br/>HTTP sink

    loop every cfg.Interval (default 60 s)

        main ->> scraper: Run(integration)

        note over scraper,k8s: Six parallel LIST calls — no watches, no informers

        scraper ->> k8s: LIST Pods (all namespaces)
        scraper ->> k8s: LIST StatefulSets (all namespaces)
        scraper ->> k8s: LIST PersistentVolumeClaims (all namespaces)
        scraper ->> k8s: LIST Services (all namespaces)
        scraper ->> k8s: LIST DaemonSets (all namespaces)
        scraper ->> k8s: LIST ConfigMaps (newrelic namespace only)

        k8s -->> scraper: six lists

        scraper ->> instr: buildClusterCtx(daemonsets, configmaps)
        note over instr: infraAgentDeployed? — match DS name<br/>ohiConfiguredFor[type]? — match CM name substring

        loop for each Pod (namespace filter applied)

            scraper ->> cls: Classify(containerImages, pod.Labels)
            note over cls: 1. operator label keys  e.g. strimzi.io/kind<br/>2. label values  e.g. app.kubernetes.io/name<br/>3. image substrings  e.g. bitnami/redis → redis

            cls -->> scraper: WorkloadType

            scraper ->> scraper: isInfrastructure?
            note over scraper: pass if: StatefulSet owner<br/>OR any PVC mount<br/>OR WorkloadType ≠ unknown<br/>skip plain Deployment pods

            scraper ->> instr: detectPodInstrumentation(pod, type, clusterCtx)
            note over instr: APM env var name?  NEW_RELIC_LICENSE_KEY …<br/>OTel env var name?  OTEL_SERVICE_NAME …<br/>OTel sidecar image?  otelcol / otel-collector<br/>OTel Operator?  instrumentation.opentelemetry.io/* annotation<br/>Prometheus?  prometheus.io/scrape = true<br/>NR annotated?  newrelic.io/* annotation key

            instr -->> scraper: InstrumentationStatus{Status, …signals}

            scraper ->> sdk: Entity("cluster:ns/pod", "k8s:discoveredWorkload")
            scraper ->> sdk: Inventory.SetItem("workload/*")
            scraper ->> sdk: Inventory.SetItem("labels/*")
            scraper ->> sdk: Inventory.SetItem("instrumentation/*")
            scraper ->> sdk: NewMetricSet("K8sDiscoveredWorkloadSample")

        end

        main ->> sdk: Publish()
        sdk  ->> sink: HTTP POST<br/>delta inventory + metric events
        sink -->> main: 200 OK

        main ->> main: namespaceCache.Vacuum()
        main ->> main: sleep(interval − elapsed)

    end
```

---

## 3. Instrumentation status decision tree

```mermaid
flowchart TD
    A([pod passes isInfrastructure check]) --> B{infraAgentDeployed\nAND ohiConfigured?}
    B -- yes --> FULL[status = instrumented\nfull OHI coverage]

    B -- no --> C{APMPresent\nNEW_RELIC_* env var?}
    C -- yes --> FULL

    C -- no --> D{OTelPresent?\nenv var · sidecar · operator annotation}
    D -- yes --> FULL

    D -- no --> E{infraAgentDeployed\nOR prometheusScraped\nOR nrAnnotated?}
    E -- yes --> PART[status = partial\nmonitoring present but\nno specific OHI for this type]

    E -- no --> NONE[status = not_instrumented]

    style FULL fill:#1a7f37,color:#fff
    style PART fill:#9a6700,color:#fff
    style NONE fill:#cf222e,color:#fff
```
