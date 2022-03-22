{{/* Generate mode label */}}
{{- define "newrelic.mode" }}
{{- if .Values.privileged -}}
privileged
{{- else -}}
unprivileged
{{- end }}
{{- end -}}

{{/* Create the name of the service account to use */}}
{{- define "newrelic.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "common.naming.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}


{{/*
Returns nrStaging
*/}}
{{- define "newrelic.nrStaging" -}}
{{- if .Values.nrStaging -}}
  {{- .Values.nrStaging -}}
{{- else if .Values.global -}}
  {{- if .Values.global.nrStaging -}}
    {{- .Values.global.nrStaging -}}
  {{- end -}}
{{- end -}}
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
Returns lowDataMode
*/}}
{{- define "newrelic.lowDataMode" -}}
{{/* `get` will return "" (empty string) if value is not found, and the value otherwise, so we can type-assert with kindIs */}}
{{- if (get .Values "lowDataMode" | kindIs "bool") -}}
  {{- if .Values.lowDataMode -}}
    {{/*
        We want only to return when this is true, returning `false` here will template "false" (string) when doing
        an `(include "newrelic-logging.lowDataMode" .)`, which is not an "empty string" so it is `true` if it is used
        as an evaluation somewhere else.
    */}}
    {{- .Values.lowDataMode -}}
  {{- end -}}
{{- else -}}
{{/* This allows us to use `$global` as an empty dict directly in case `Values.global` does not exists */}}
{{- $global := index .Values "global" | default dict -}}
{{- if get $global "lowDataMode" | kindIs "bool" -}}
  {{- if $global.lowDataMode -}}
    {{- $global.lowDataMode -}}
  {{- end -}}
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
{{- if include "newrelic.lowDataMode" . -}}
interval: 30s
{{- else  -}}
interval: 15s
{{- end -}}
{{- end -}}
