FROM alpine:3.17.1

ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache --upgrade && apk add --no-cache tini curl bind-tools

COPY bin/nri-kubernetes-${TARGETOS}-${TARGETARCH} /bin/

RUN mv /bin/nri-kubernetes-${TARGETOS}-${TARGETARCH} /bin/nri-kubernetes && \
    chmod 755 /bin/nri-kubernetes

# creating the nri-agent user used only in unprivileged mode
RUN addgroup -g 2000 nri-agent && adduser -D -u 1000 -G nri-agent nri-agent

USER nri-agent

ENTRYPOINT ["/sbin/tini", "--", "/bin/nri-kubernetes"]
