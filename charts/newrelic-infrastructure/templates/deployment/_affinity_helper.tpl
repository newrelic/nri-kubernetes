{{- /*
Patch to add affinity in case we are running in fargate mode
*/ -}}
{{- define "nriKubernetes.deployment.affinity.fargateDefaults" -}}
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
{{- define "nriKubernetes.deployment.affinity" -}}

{{- if or .Values.deployment.affinity .Values.nodeAffinity  -}}
    {{- $legacyNodeAffinity := fromYaml ( include "newrelic.compatibility.nodeAffinity" . ) | default dict -}}
    {{- $valuesAffinity := .Values.deployment.affinity | default dict -}}
    {{- $affinity := mustMergeOverwrite $legacyNodeAffinity $valuesAffinity -}}
    {{- toYaml $affinity -}}
{{- else if include "newrelic.common.affinity" . -}}
    {{- include "newrelic.common.affinity" . -}}
{{- else if include "newrelic.fargate" . -}}
    {{- include "nriKubernetes.deployment.affinity.fargateDefaults" . -}}
{{- end -}}
{{- end -}}
