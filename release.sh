#! /bin/bash
if [[ $# -eq 0 ]] ; then
    echo 'Please specify the new release tag'
    exit 0
fi

sed -i -r 's/(## Unreleased)/\1\n\n## '$1'/g' CHANGELOG.md
sed -i -r 's/(integrationVersion = \").*$/\1'$1'"/' src/kubernetes.go
sed -i -r 's/(image\: newrelic\/infrastructure-k8s\:).*$/\1'$1'/' deploy/newrelic-infra.yaml
sed -i -r 's/(image\: newrelic\/infrastructure-k8s\:).*$/\1'$1'/' deploy/newrelic-infra-unprivileged.yaml
