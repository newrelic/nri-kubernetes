{{- /*
Defaults for ksm while keeping then overridable.
*/ -}}
{{- define "nriKubernetes.ksm.affinity.defaults" -}}
podAffinity:
  preferredDuringSchedulingIgnoredDuringExecution:
    - podAffinityTerm:
        topologyKey: kubernetes.io/hostname
        labelSelector:
          matchLabels:
            app.kubernetes.io/name: kube-state-metrics
      weight: 100
nodeAffinity: {}
{{- end -}}



{{- /*
As this chart deploys what it should be three charts to maintain the transition to v3 as smooth as possible.
This means that this chart has 3 affinity so a helper should be done per scraper.
*/ -}}
{{- define "nriKubernetes.ksm.affinity" -}}
{{- if .Values.ksm.affinity -}}
    {{- toYaml .Values.ksm.affinity -}}
{{- else if include "newrelic.compatibility.nodeAaffinity" . -}}
    {{- include "newrelic.compatibility.nodeAaffinity" . -}}
{{- else if include "common.affinity" . -}}
    {{- include "common.affinity" . -}}
{{- else -}}
    {{- include "nriKubernetes.ksm.affinity.defaults" . -}}
{{- end -}}
{{- end -}}
