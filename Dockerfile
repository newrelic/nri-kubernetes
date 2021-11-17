ARG BASE_IMAGE=newrelic/infrastructure-bundle:2.7.4

FROM $BASE_IMAGE

# Set by docker automatically
# If building with `docker build`, make sure to set GOOS/GOARCH explicitly when calling make:
# `make compile GOOS=something GOARCH=something`
# Otherwise the makefile will not append them to the binary name and docker build will fail.
ARG TARGETOS
ARG TARGETARCH

# ensure there is no default integration enabled
RUN rm -rf /etc/newrelic-infra/integrations.d/*

# creating the nri-agent user used only in unprivileged mode
RUN addgroup -g 2000 nri-agent && adduser -D -u 1000 -G nri-agent nri-agent

ENV NRIA_HTTP_SERVER_ENABLED true

ENTRYPOINT ["/sbin/tini", "--", "/usr/bin/newrelic-infra"]
