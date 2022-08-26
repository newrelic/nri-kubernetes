{{- /* Naming helpers*/ -}}
{{- define "nriKubernetes.deployment.fullname" -}}
{{- include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "nriKubernetes.naming.fullname" .) "suffix" "deployment") -}}
{{- end -}}

{{- define "nriKubernetes.deployment.fullname.agent" -}}
{{- include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "nriKubernetes.naming.fullname" .) "suffix" "agent-deployment") -}}
{{- end -}}

{{- define "nriKubernetes.deployment.fullname.integrations" -}}
{{- include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "nriKubernetes.naming.fullname" .) "suffix" "deployment-integrations-cfg") -}}
{{- end -}}
