{{- /*
Defaults for kubelet while keeping then overridable.
*/ -}}
{{- define "nriKubernetes.kubelet.tolerations.defaults" -}}
- operator: "Exists"
  effect: "NoSchedule"
- operator: "Exists"
  effect: "NoExecute"
{{- end -}}



{{- /*
As this chart deploys what it should be three charts to maintain the transition to v3 as smooth as possible.
This means that this chart has 3 tolerations so a helper should be done per scraper.
*/ -}}
{{- define "nriKubernetes.kubelet.tolerations" -}}
{{- if .Values.kubelet.tolerations -}}
    {{- toYaml .Values.kubelet.tolerations -}}
{{- else if include "common.tolerations" . -}}
    {{- include "common.tolerations" . -}}
{{- else -}}
    {{- include "nriKubernetes.kubelet.tolerations.defaults" . -}}
{{- end -}}
{{- end -}}
