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
    FROM alpine:3.16.0
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

k8s_yaml(helm('./charts/newrelic-infrastructure', name='nr', values=['values-dev.yaml', 'values-local.yaml']))

k8s_yaml(helm('./charts/internal/e2e-resources', name='e2e-resources'))
