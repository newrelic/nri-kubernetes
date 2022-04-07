{{- /*
Defaults for controlPlane's agent config
*/ -}}
{{- define "nriKubernetes.controlPlane.agentConfig.defaults" -}}
is_forward_only: true
overide_host_root: ""  # Typo from here: https://github.com/newrelic/infrastructure-agent/blob/master/pkg/config/config.go#L267
http_server_enabled: true
http_server_port: 8001
{{- end -}}



{{- define "nriKubernetes.controlPlane.agentConfig" -}}
{{- $agentDefaults := fromYaml ( include "common.agentConfig.defaults" . ) -}}
{{- $controlPlane := fromYaml ( include "nriKubernetes.controlPlane.agentConfig.defaults" . ) -}}
{{- $agentConfig := fromYaml ( include "newrelic.compatibility.agentConfig" . ) -}}

{{- mustMergeOverwrite $agentDefaults $controlPlane $agentConfig | toYaml -}}
{{- end -}}
