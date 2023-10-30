{{/*
Create ControlPlane Agent ConfigMap
*/}}
{{- define "nriKubernetes.controlPlane.agentConfigMap" -}}
{{- if and (.Values.controlPlane.enabled) (not (include "newrelic.common.gkeAutopilot" .)) -}}
true
{{- end -}}
{{- end -}}

{{/*
Create ControlPlane ClusterRole
*/}}
{{- define "nriKubernetes.controlPlane.clusterRole" -}}
{{- if and (.Values.controlPlane.enabled) (.Values.rbac.create) (not (include "newrelic.common.gkeAutopilot" .)) -}}
true
{{- end -}}
{{- end -}}

{{/*
Create ControlPlane ClusterRoleBinding
*/}}
{{- define "nriKubernetes.controlPlane.clusterRoleBinding" -}}
{{- if and (.Values.controlPlane.enabled) (.Values.rbac.create) (not (include "newrelic.common.gkeAutopilot" .)) -}}
true
{{- end -}}
{{- end -}}

{{/*
Create ControlPlane DaemonSet
*/}}
{{- define "nriKubernetes.controlPlane.daemonSet" -}}
{{- if and (.Values.controlPlane.enabled) (not (include "newrelic.common.gkeAutopilot" .)) (not (include "newrelic.fargate" .)) -}}
true
{{- end -}}
{{- end -}}

{{/*
Create ControlPlane RoleBinding
*/}}
{{- define "nriKubernetes.controlPlane.roleBinding" -}}
{{- if and (.Values.rbac.create) (not (include "newrelic.common.gkeAutopilot" .)) -}}
true
{{- end -}}
{{- end -}}

{{/*
Create ControlPlane Scraper ConfigMap
*/}}
{{- define "nriKubernetes.controlPlane.scraperConfigMap" -}}
{{- if and (.Values.controlPlane.enabled) (not (include "newrelic.common.gkeAutopilot" .)) -}}
true
{{- end -}}
{{- end -}}

{{/*
Create ControlPlane Service Account
*/}}
{{- define "nriKubernetes.controlPlane.serviceAccount" -}}
{{- if and (include "newrelic.common.serviceAccount.create" .) (not (include "newrelic.common.gkeAutopilot" .)) -}}
true
{{- end -}}
{{- end -}}
