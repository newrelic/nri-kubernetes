{{- /*
Defaults for deployment's agent config
*/ -}}
{{- define "nriKubernetes.deployment.agentConfig.defaults" -}}
http_server_enabled: true
http_server_port: 8003
features:
  docker_enabled: false
{{- if not ( include "newrelic.common.privileged" . ) }}
is_secure_forward_only: true
{{- end }}
{{- /*
`enableProcessMetrics` is commented in the values and we want to configure it when it is set to something
either `true` or `false`. So we test if the variable is a boolean and in that case simply use it.
*/}}
{{- if (get .Values "enableProcessMetrics" | kindIs "bool") }}
enable_process_metrics: {{ .Values.enableProcessMetrics }}
{{- end }}
{{- end -}}



{{- define "nriKubernetes.deployment.agentConfig" -}}
{{- $agentDefaults := fromYaml ( include "newrelic.common.agentConfig.defaults" . ) -}}
{{- $deployment := fromYaml ( include "nriKubernetes.deployment.agentConfig.defaults" . ) -}}
{{- $agentConfig := fromYaml ( include "newrelic.compatibility.agentConfig" . ) -}}
{{- $deploymentAgentConfig := .Values.deployment.agentConfig -}}
{{- $customAttributes := dict "custom_attributes" (dict "clusterName" (include "newrelic.common.cluster" . )) -}}

{{- mustMergeOverwrite $agentDefaults $deployment $agentConfig $deploymentAgentConfig $customAttributes | toYaml -}}
{{- end -}}
