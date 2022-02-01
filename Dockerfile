FROM alpine:3.15.0

ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache --upgrade && apk add --no-cache tini=0.19.0-r0 curl bind-tools

ADD --chmod=755 bin/nri-kubernetes-${TARGETOS}-${TARGETARCH} /var/db/newrelic-infra/newrelic-integrations/bin/

RUN mv /var/db/newrelic-infra/newrelic-integrations/bin/nri-kubernetes-${TARGETOS}-${TARGETARCH} \
       /var/db/newrelic-infra/newrelic-integrations/bin/nri-kubernetes

# creating the nri-agent user used only in unprivileged mode
RUN addgroup -g 2000 nri-agent && adduser -D -u 1000 -G nri-agent nri-agent

USER nri-agent

ENTRYPOINT ["/sbin/tini", "--", "/var/db/newrelic-infra/newrelic-integrations/bin/nri-kubernetes"]
