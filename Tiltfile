# -*- mode: Python -*-

# Settings and defaults.

project_name = 'nri-kubernetes'
# If using a cloud cluster, change this to the cluster you want to use
cluster_name = 'dbudziwojski-aks-tsty'
# If using a cloud cluster, set this to the repository you want to push the image to. If using a local cluster, leave this empty.
repository = 'k8sagentsuserreg.azurecr.io'

live_reload = True

# Only use explicitly allowed kubeconfigs as a safety measure.
allow_k8s_contexts([cluster_name])

# Detect the active cluster's node architecture so builds target the right platform
# whether we're pointed at local minikube (arm64) or the remote EKS cluster (amd64).
node_arch = str(local("kubectl get nodes -o jsonpath='{.items[0].status.nodeInfo.architecture}'", quiet=True)).strip()
platform = 'linux/%s' % node_arch

# Building Docker image.
load('ext://restart_process', 'docker_build_with_restart')

docker_ref = project_name
if repository != '':
    docker_ref = '%s/%s' % (repository, project_name)

if live_reload:
  binary_name = '%s-linux-%s' % (project_name, node_arch)

  # Building daemon binary locally.
  local_resource(
    '%s-binary' % project_name,
    'GOOS=linux GOARCH=%s make compile' % node_arch,
    deps=[
      "./src",
      "./internal",
      "./cmd"
    ],
  )

  # Use custom Dockerfile for Tilt builds, which only takes locally built daemon binary for live reloading.
  dockerfile = '''
    FROM alpine:3.24.1
    COPY %s /usr/local/bin/%s
  ''' % (binary_name, project_name)

  docker_build_with_restart(
    ref=docker_ref,
    context='./bin',
    dockerfile_contents=dockerfile,
    entrypoint=[
      "/usr/local/bin/%s" % project_name,
    ],
    platform=platform,
    only=binary_name,
    live_update=[
      # Copy the binary so it gets restarted.
      sync('bin/%s' % binary_name, '/usr/local/bin/%s' % project_name),
    ],
  )
else:
  local(['make', 'compile', 'GOOS=linux', 'GOARCH=amd64'])
  local(['make', 'compile', 'GOOS=linux', 'GOARCH=arm64'])
  docker_build(docker_ref, '.', platform=platform)

# ns_yaml_str is wrapped as Blob so that Tiltfile will treat it as DATA and not as a filepath
ns_yaml_str = """
---
apiVersion: v1
kind: Namespace
metadata:
  name: nri-k8s-dev
  labels:
    environment: dev
    team: k8-team
  annotations:
    owner: "namespace@example.com"
    description: "Namespace for e2e workloads"
"""
k8s_yaml([blob(ns_yaml_str)])

k8s_yaml(helm(
    './charts/newrelic-infrastructure',
    name='nr',
    namespace='nri-k8s-dev',
    values=['values-dev.yaml', 'values-local.yaml'],
    set=[
        'images.integration.registry=%s' % repository,
        'images.windowsIntegration.registry=%s' % repository
    ]
))

k8s_yaml(helm(
    './charts/internal/e2e-resources',
    name='e2e-resources',
    namespace='nri-k8s-dev',
    set=[
        'demo.enabled=true'
    ]
))
