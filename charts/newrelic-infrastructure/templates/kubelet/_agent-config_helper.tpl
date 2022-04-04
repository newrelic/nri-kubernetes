{{- /*
Defaults for kubelet's agent config
*/ -}}
{{- define "nriKubernetes.kubelet.agentConfig.defaults" -}}
http_server_enabled: true
http_server_port: 8003
features:
  docker_enabled: false
{{- if include "common.privileged" . }}
is_secure_forward_only: true
overide_host_root: ""  # Typo from here: https://github.com/newrelic/infrastructure-agent/blob/master/pkg/config/config.go#L267
{{- end }}
{{- if .Values.enableProcessMetrics }}
enable_process_metrics: {{ .Values.enableProcessMetrics }}
{{- end }}
{{- end -}}



{{- define "nriKubernetes.kubelet.agentConfig" -}}
{{- $agentDefaults := fromYaml ( include "common.agentConfig.defaults" . ) -}}
{{- $kubelet := fromYaml ( include "nriKubernetes.kubelet.agentConfig.defaults" . ) -}}
{{- $agentConfig := fromYaml ( include "newrelic.compatibility.agentConfig" . ) -}}

{{- mustMergeOverwrite $agentDefaults $kubelet $agentConfig | toYaml -}}
{{- end -}}
