{{- /*
By default the common library uses .Chart.Name for creating the name.
This chart's name is too long so we shorted to `nrk8s`
*/ -}}
{{- define "common.naming.chartnameOverride" -}}
nrk8s
{{- end -}}



{{- /* Allow to change container defaults dynamically based if we are running in privileged mode or not */ -}}
{{- define "common.securityContext.containerDefaults" -}}
runAsUser: 1000
runAsGroup: 2000
allowPrivilegeEscalation: false
readOnlyRootFilesystem: true
{{- end -}}



{{- /* Allow to change pod defaults dynamically based if we are running in privileged mode or not */ -}}
{{- define "common.securityContext.podDefaults" -}}
{{- end -}}



{{- /* Add mode to each object create */ -}}
{{- define "common.labels.overrides.addLabels" -}}
{{- if ( include "newrelic.common.privileged" . ) -}}
mode: privileged
{{- else -}}
mode: unprivileged
{{- end -}}
{{- end -}}



{{/*
This function allows easily to overwrite custom attributes to the function "common.customAttributes"
*/}}
{{- define "common.customAttributes.overrideAttributes" -}}
clusterName: {{ include "newrelic.common.cluster" . }}
{{- end }}
