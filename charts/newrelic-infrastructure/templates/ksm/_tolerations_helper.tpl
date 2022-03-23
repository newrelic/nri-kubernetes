{{- /*
Defaults for ksm while keeping then overridable.
*/ -}}
{{- define "nriKubernetes.ksm.tolerations.defaults" -}}
- operator: "Exists"
  effect: "NoSchedule"
- operator: "Exists"
  effect: "NoExecute"
{{- end -}}



{{- /*
As this chart deploys what it should be three charts to maintain the transition to v3 as smooth as possible.
This means that this chart has 3 tolerations so a helper should be done per scraper.
*/ -}}
{{- define "nriKubernetes.ksm.tolerations" -}}
{{- if gt (len .Values.ksm.tolerations) 0  -}}
    {{- toYaml .Values.ksm.tolerations -}}
{{- else if include "common.tolerations" . -}}
    {{- include "common.tolerations" . -}}
{{- else -}}
    {{- include "nriKubernetes.ksm.tolerations.defaults" . -}}
{{- end -}}
{{- end -}}
