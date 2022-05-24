{{- /* Naming helpers*/ -}}
{{- define "nriKubernetes.kubelet.fullname" -}}
{{- include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "nriKubernetes.naming.fullname" .) "suffix" "kubelet-node-scraper") -}}
{{- end -}}

{{- define "nriKubernetes.kubelet.fullname.agent" -}}
{{- include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "nriKubernetes.naming.fullname" .) "suffix" "kubelet-node-agent") -}}
{{- end -}}

{{- define "nriKubernetes.kubelet.fullname.integrations" -}}
{{- include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "nriKubernetes.naming.fullname" .) "suffix" "integrations-cfg") -}}
{{- end -}}
