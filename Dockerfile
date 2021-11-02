ARG MODE=normal
ARG BASE_IMAGE=newrelic/infrastructure-bundle:2.7.4

FROM $BASE_IMAGE AS base

# Set by docker automatically
# If building with `docker build`, make sure to set GOOS/GOARCH explicitly when calling make:
# `make compile GOOS=something GOARCH=something`
# Otherwise the makefile will not append them to the binary name and docker build will fail.
ARG TARGETOS
ARG TARGETARCH

# ensure there is no default integration enabled
RUN rm -rf /etc/newrelic-infra/integrations.d/*
ADD nri-kubernetes-definition.yml /var/db/newrelic-infra/newrelic-integrations/
ADD --chmod=755 bin/nri-kubernetes-${TARGETOS}-${TARGETARCH} /var/db/newrelic-infra/newrelic-integrations/bin/
RUN mv /var/db/newrelic-infra/newrelic-integrations/bin/nri-kubernetes-${TARGETOS}-${TARGETARCH} \
       /var/db/newrelic-infra/newrelic-integrations/bin/nri-kubernetes

# Warning: First, Edit sample file to suit your needs and rename it to
# `nri-kubernetes-config.yml`
ADD nri-kubernetes-config.yml.sample /var/db/newrelic-infra/integrations.d/nri-kubernetes-config.yml

FROM base AS branch-normal
USER root

FROM base AS branch-unprivileged
RUN addgroup -g 2000 nri-agent && adduser -D -u 1000 -G nri-agent nri-agent
USER nri-agent

ENV NRIA_OVERRIDE_HOST_ROOT ""
ENV NRIA_IS_SECURE_FORWARD_ONLY true

FROM branch-${MODE}
ENTRYPOINT ["/sbin/tini", "--", "/usr/bin/newrelic-infra"]
