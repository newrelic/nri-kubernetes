{{- define "nriKubernetes.kubelet.securityContext.privileged" -}}
runAsUser: 0
runAsGroup: 0
allowPrivilegeEscalation: true
privileged: true
readOnlyRootFilesystem: true
{{- end -}}



{{- define "nriKubernetes.kubelet.securityContext.agentContainer" -}}
{{- $privileged := dict -}}
{{- if include "common.privileged" . -}}
{{- $privileged = fromYaml ( include "nriKubernetes.kubelet.securityContext.privileged" . ) -}}
{{- end -}}
{{- $privileged
        | mustMergeOverwrite (include "newrelic.compatibility.securityContext" . | fromYaml )
        | mustMergeOverwrite (include "common.securityContext.container" . | fromYaml )
        | toYaml
-}}
{{- end -}}
