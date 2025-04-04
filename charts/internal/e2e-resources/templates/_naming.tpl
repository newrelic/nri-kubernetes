{{- define "nriKubernetes.e2e.kubelet.fullname.windows2019" -}}
{{- include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name"  .Release.Name "suffix" "windows-server-2019" ) -}}
{{- end -}}

{{- define "nriKubernetes.e2e.kubelet.fullname.windows2022" -}}
{{- include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name"  .Release.Name "suffix" "windows-server-2022" ) -}}
{{- end -}}
