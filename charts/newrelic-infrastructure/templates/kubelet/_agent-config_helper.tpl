{{- /*
Defaults for kubelet's agent config
*/ -}}
{{- define "nriKubernetes.kubelet.agentConfig.defaults" -}}
http_server_enabled: true
http_server_port: 8003
features:
  docker_enabled: false
{{- if not ( include "nriKubernetes.privileged" . ) }}
is_secure_forward_only: true
{{- end }}
{{- /*
`enableProcessMetrics` is commented in the values and we want to configure it when it is set to something
either `true` or `false`. So we test if the variable is a boolean and in that case simply use it.
*/}}
{{- if (get .Values "enableProcessMetrics" | kindIs "bool") }}
enable_process_metrics: {{ .Values.enableProcessMetrics }}
{{- end }}
{{- /*
`enable_elevated_process_priv` enables SeDebugPrivilege on Windows for enhanced process visibility.
Auto-enable when enableProcessMetrics is true AND enableWindows is true, since Windows HostProcess
containers are inherently privileged and partial process visibility is less useful.
Users can still override via kubelet.agentConfig.enable_elevated_process_priv if needed.
*/}}
{{- if (get .Values "enableElevatedProcessPrivilege" | kindIs "bool") }}
enable_elevated_process_priv: {{ .Values.enableElevatedProcessPrivilege }}
{{- else if and (get .Values "enableProcessMetrics") (get .Values "enableWindows") }}
enable_elevated_process_priv: true
{{- end }}
{{- end -}}



{{- define "nriKubernetes.kubelet.agentConfig" -}}
{{- $agentDefaults := fromYaml ( include "newrelic.common.agentConfig.defaults" . ) -}}
{{- $kubelet := fromYaml ( include "nriKubernetes.kubelet.agentConfig.defaults" . ) -}}
{{- $agentConfig := fromYaml ( include "newrelic.compatibility.agentConfig" . ) -}}
{{- $kubeletAgentConfig := .Values.kubelet.agentConfig -}}
{{- $customAttributes := dict "custom_attributes" (dict "clusterName" (include "newrelic.common.cluster" . )) -}}

{{- mustMergeOverwrite $agentDefaults $kubelet $agentConfig $kubeletAgentConfig $customAttributes | toYaml -}}
{{- end -}}
