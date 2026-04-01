#!/bin/sh
# Startup script for bundled sidecar: runs both infrastructure agent and nri-kubernetes
# The agent listens on HTTP port for metrics, nri-kubernetes sends metrics to it

set -e

# Map v2-style env vars to v3-style for compatibility with operator injection
export NRI_KUBERNETES_NODENAME="${NRI_KUBERNETES_NODENAME:-$NRK8S_NODE_NAME}"
export NRI_KUBERNETES_NODEIP="${NRI_KUBERNETES_NODEIP:-$NRIA_HOST}"
export NRI_KUBERNETES_CLUSTERNAME="${NRI_KUBERNETES_CLUSTERNAME:-$CLUSTER_NAME}"

# Start the infrastructure agent in the background
# It will listen on HTTP_SERVER_PORT (default 8001) for metrics from nri-kubernetes
/usr/bin/newrelic-infra &
AGENT_PID=$!

# Wait for agent to be ready (HTTP server to be up)
echo "Waiting for infrastructure agent to start..."
RETRIES=30
while [ $RETRIES -gt 0 ]; do
    if curl -sf http://localhost:${NRI_KUBERNETES_SINK_HTTP_PORT:-8001}/healthz > /dev/null 2>&1; then
        echo "Infrastructure agent is ready"
        break
    fi
    RETRIES=$((RETRIES - 1))
    sleep 1
done

if [ $RETRIES -eq 0 ]; then
    echo "Warning: Could not verify agent health, proceeding anyway..."
fi

# Start nri-kubernetes in the foreground
# It will send metrics to the agent's HTTP server
exec /var/db/newrelic-infra/newrelic-integrations/bin/nri-kubernetes
