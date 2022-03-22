{{/* Generate mode label */}}
{{- define "newrelic.mode" }}
{{- if .Values.privileged -}}
privileged
{{- else -}}
unprivileged
{{- end }}
{{- end -}}



{{/*
Returns fargate
*/}}
{{- define "newrelic.fargate" -}}
{{- if .Values.fargate -}}
  {{- .Values.fargate -}}
{{- else if .Values.global -}}
  {{- if .Values.global.fargate -}}
    {{- .Values.global.fargate -}}
  {{- end -}}
{{- end -}}
{{- end -}}


{{/*
Returns the list of namespaces where secrets need to be accessed by the controlPlane integration to do mTLS Auth
*/}}
{{- define "newrelic.roleBindingNamespaces" -}}
{{ $namespaceList := list }}
{{- range $components := .Values.controlPlane.config }}
  {{- if $components }}
  {{- if kindIs "map" $components -}}
  {{- if $components.staticEndpoint }}
      {{- if $components.staticEndpoint.auth }}
      {{- if $components.staticEndpoint.auth.mtls }}
      {{- if $components.staticEndpoint.auth.mtls.secretNamespace }}
      {{- $namespaceList = append $namespaceList $components.staticEndpoint.auth.mtls.secretNamespace -}}
      {{- end }}
      {{- end }}
      {{- end }}
  {{- end }}
  {{- if $components.autodiscover }}
    {{- range $autodiscover := $components.autodiscover }}
      {{- if $autodiscover }}
      {{- if $autodiscover.endpoints }}
        {{- range $endpoint := $autodiscover.endpoints }}
            {{- if $endpoint.auth }}
            {{- if $endpoint.auth.mtls }}
            {{- if $endpoint.auth.mtls.secretNamespace }}
            {{- $namespaceList = append $namespaceList $endpoint.auth.mtls.secretNamespace -}}
            {{- end }}
            {{- end }}
            {{- end }}
        {{- end }}
      {{- end }}
      {{- end }}
    {{- end }}
  {{- end }}
  {{- end }}
  {{- end }}
{{- end }}
roleBindingNamespaces: {{- uniq $namespaceList | toYaml | nindent 0 }}
{{- end -}}

{{/*
Returns Custom Attributes even if formatted as a json string
*/}}
{{- define "newrelic.customAttributesWithoutClusterName" -}}
{{- if kindOf .Values.customAttributes | eq "string" -}}
{{  .Values.customAttributes }}
{{- else -}}
{{ .Values.customAttributes | toJson }}
{{- end -}}
{{- end -}}

{{- define "newrelic.customAttributes" -}}
{{- merge (include "newrelic.customAttributesWithoutClusterName" . | fromJson) (dict "clusterName" (include "common.cluster" .)) | toJson }}
{{- end -}}

{{- define "newrelic.integrationConfigDefaults" -}}
{{- if include "common.lowDataMode" . -}}
interval: 30s
{{- else  -}}
interval: 15s
{{- end -}}
{{- end -}}
