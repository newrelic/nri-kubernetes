{{- /*
Defaults for controlPlane while keeping then overridable.
*/ -}}
{{- define "nriKubernetes.controlPlane.affinity.defaults" -}}
nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
      - matchExpressions:
          - key: node-role.kubernetes.io/control-plane
            operator: Exists
      - matchExpressions:
          - key: node-role.kubernetes.io/controlplane
            operator: Exists
      - matchExpressions:
          - key: node-role.kubernetes.io/etcd
            operator: Exists
      - matchExpressions:
          - key: node-role.kubernetes.io/master
            operator: Exists
{{- end -}}



{{- /*
As this chart deploys what it should be three charts to maintain the transition to v3 as smooth as possible.
This means that this chart has 3 affinity so a helper should be done per scraper.
*/ -}}
{{- define "nriKubernetes.controlPlane.affinity" -}}
{{- if .Values.controlPlane.affinity -}}
    {{- toYaml .Values.controlPlane.affinity -}}
{{- else if include "common.affinity" . -}}
    {{- include "common.affinity" . -}}
{{- else -}}
    {{- include "nriKubernetes.controlPlane.affinity.defaults" . -}}
{{- end -}}
{{- end -}}
