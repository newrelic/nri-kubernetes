{{- /*
Defaults for kubelet while keeping then overridable.
*/ -}}
{{- define "nriKubernetes.kubelet.affinity.defaults" -}}
{{- end -}}



{{- /*
Patch to add affinity in case we are running in fargate mode
*/ -}}
{{- define "nriKubernetes.kubelet.affinity.fargateDefaults" -}}
nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
      - matchExpressions:
          - key: eks.amazonaws.com/compute-type
            operator: NotIn
            values:
              - fargate
{{- end -}}



{{- /*
As this chart deploys what it should be three charts to maintain the transition to v3 as smooth as possible.
This means that this chart has 3 affinity so a helper should be done per scraper.
*/ -}}
{{- define "nriKubernetes.kubelet.affinity" -}}
{{- if .Values.kubelet.affinity -}}
    {{- toYaml .Values.kubelet.affinity -}}
{{- else if include "newrelic.compatibility.nodeAaffinity" . -}}
    {{- include "newrelic.compatibility.nodeAaffinity" . -}}
{{- else if include "common.affinity" . -}}
    {{- include "common.affinity" . -}}
{{- else if include "newrelic.fargate" . -}}
    {{- include "nriKubernetes.kubelet.affinity.fargateDefaults" . -}}
{{- else -}}
    {{- include "nriKubernetes.kubelet.affinity.defaults" . -}}
{{- end -}}
{{- end -}}
