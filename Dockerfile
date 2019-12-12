ARG IMAGE_NAME=newrelic/infrastructure
ARG IMAGE_TAG=1.8.23
ARG MODE=normal

FROM $IMAGE_NAME:$IMAGE_TAG AS base
ADD nr-kubernetes-definition.yml /var/db/newrelic-infra/newrelic-integrations/
ADD bin/nr-kubernetes /var/db/newrelic-infra/newrelic-integrations/bin/
# Warning: First, Edit sample file to suit your needs and rename it to
# `nr-kubernetes-config.yml`
ADD nr-kubernetes-config.yml.sample /etc/newrelic-infra/integrations.d/nr-kubernetes-config.yml

FROM base AS branch-normal
USER root

FROM base AS branch-unprivileged

RUN addgroup -g 2000 nri-agent && adduser -D -u 1000 -G nri-agent nri-agent
USER nri-agent

ENV NRIA_OVERRIDE_HOST_ROOT ""
ENV NRIA_IS_SECURE_FORWARD_ONLY true

FROM branch-${MODE}
ENTRYPOINT ["/usr/bin/newrelic-infra"]
