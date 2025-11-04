# -*- mode: Python -*-

# Settings and defaults.

project_name = 'nri-kubernetes'
cluster_name = 'minikube'

live_reload = True

# Only use explicitly allowed kubeconfigs as a safety measure.
allow_k8s_contexts(cluster_name)

# Building Docker image.
load('ext://restart_process', 'docker_build_with_restart')

if live_reload:
  binary_name = '%s-linux' % project_name

  # Building daemon binary locally.
  local_resource(
    '%s-binary' % project_name,
    'GOOS=linux make compile',
    deps=[
            "./src",
            "./internal",
            "./cmd"
        ],
    )

  # Use custom Dockerfile for Tilt builds, which only takes locally built daemon binary for live reloading.
  dockerfile = '''
    FROM alpine:3.17.3
    COPY %s /usr/local/bin/%s
  ''' % (binary_name, project_name)

  docker_build_with_restart(
    ref=project_name,
    context='./bin',
    dockerfile_contents=dockerfile,
    entrypoint=[
      "/usr/local/bin/%s" % project_name,
    ],
    only=binary_name,
    live_update=[
      # Copy the binary so it gets restarted.
      sync('bin/%s' % binary_name, '/usr/local/bin/%s' % project_name),
    ],
  )
else:
  docker_build(project_name, '.')

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

k8s_yaml(helm('./charts/newrelic-infrastructure', name='nr', namespace='nri-k8s-dev', values=['values-dev.yaml', 'values-local.yaml']))

k8s_yaml(helm('./charts/internal/e2e-resources', name='e2e-resources', namespace='nri-k8s-dev'))
