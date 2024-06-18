{{- /* Naming helpers*/ -}}
{{- define "nriKubernetes.kubelet.fullname" -}}
{{- if (.Values.kubelet.agentOnly) }}
{{- include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "nriKubernetes.naming.fullname" .) "suffix" "agent") -}}
{{- else }}
{{- include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "nriKubernetes.naming.fullname" .) "suffix" "kubelet") -}}
{{- end }}
{{- end -}}

{{- define "nriKubernetes.kubelet.fullname.agent" -}}
{{- include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "nriKubernetes.naming.fullname" .) "suffix" "agent-kubelet") -}}
{{- end -}}

{{- define "nriKubernetes.kubelet.fullname.integrations" -}}
{{- include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "nriKubernetes.naming.fullname" .) "suffix" "integrations-cfg") -}}
{{- end -}}
