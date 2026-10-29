{{/*
storageClassName line of a PVC: the persistence block's storageClass, otherwise
global.defaultStorageClass or global.storageClass. "-" renders an empty class (no dynamic
provisioning); no class at all renders nothing, leaving the cluster default.
  include "common.storage.class" (dict "persistence" .Values.persistence "global" .Values.global)
*/}}
{{- define "common.storage.class" -}}
{{- $class := .persistence.storageClass | default (.global).defaultStorageClass | default (.global).storageClass | default "" | toString -}}
{{- if eq $class "-" -}}
storageClassName: ""
{{- else if $class -}}
storageClassName: {{ quote $class }}
{{- end -}}
{{- end -}}
