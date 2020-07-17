# Agent v1.1.14 (2018-12-20)
FROM newrelic/infrastructure:0.0.62
ADD nr-kubernetes-definition.yml /var/db/newrelic-infra/newrelic-integrations/
ADD bin/nr-kubernetes /var/db/newrelic-infra/newrelic-integrations/bin/
# Warning: First, Edit sample file to suit your needs and rename it to
# `nr-kubernetes-config.yml`
ADD nr-kubernetes-config.yml.sample /etc/newrelic-infra/integrations.d/nr-kubernetes-config.yml
