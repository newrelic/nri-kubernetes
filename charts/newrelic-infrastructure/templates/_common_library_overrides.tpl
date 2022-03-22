{{- /*
By default the common library uses .Chart.Name for creating the name.
This chart's name is too long so we shorted to `nrk8s`
*/ -}}
{{- define "common.naming.chartnameOverride" -}}
nrk8s
{{- end -}}
