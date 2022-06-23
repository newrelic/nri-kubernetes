{{- /* Naming helpers*/ -}}
{{- define "nriKubernetes.ksm.fullname" -}}
{{- include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "nriKubernetes.naming.fullname" .) "suffix" "ksm-scraper") -}}
{{- end -}}

{{- define "nriKubernetes.ksm.fullname.agent" -}}
{{- include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "nriKubernetes.naming.fullname" .) "suffix" "ksm-agent") -}}
{{- end -}}
