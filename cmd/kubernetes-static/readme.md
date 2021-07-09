# Kubernetes Static

Kubernetes Static is a project used to run the Kubernetes Integration locally on your machine, without needing a running kubernetes cluster with KSM.

## How it works

The files in the `./data` folder are saved outputs from KSM and various kubelet endpoints, which will be embedded at build-time and then served from a temporary HTTP server.
The groupers are configured to use these endpoints instead of discovering them.

## Running kubernetes-static

From within root of this repository, run the following command in your terminal
```shell script
go run cmd/kubernetes-static/main.go cmd/kubernetes-static/basic_http_client.go
```

This is not sending any data to an agent, but outputs the JSON to stdout.
