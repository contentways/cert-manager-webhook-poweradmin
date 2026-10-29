{{/*
Object names. Every result is cut to 63 characters, the limit of a DNS label.
*/}}

{{/* Chart name, or .Values.nameOverride. */}}
{{- define "common.names.name" -}}
{{- .Values.nameOverride | default .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* "<chart>-<version>" for the helm.sh/chart label; "+" is not allowed in label values. */}}
{{- define "common.names.chart" -}}
{{- list .Chart.Name .Chart.Version | join "-" | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Base name of the release's objects: .Values.fullnameOverride, otherwise "<release>-<name>",
or just the release name when it already contains the chart name.
*/}}
{{- define "common.names.fullname" -}}
{{- $result := .Values.fullnameOverride -}}
{{- if not $result -}}
  {{- $chartName := .Values.nameOverride | default .Chart.Name -}}
  {{- $result = .Release.Name -}}
  {{- if not (contains $chartName .Release.Name) -}}
    {{- $result = list .Release.Name $chartName | join "-" -}}
  {{- end -}}
{{- end -}}
{{- $result | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Target namespace, or .Values.namespaceOverride. */}}
{{- define "common.names.namespace" -}}
{{- .Values.namespaceOverride | default .Release.Namespace | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* "<fullname>-<namespace>", unique across namespaces, for cluster-scoped objects. */}}
{{- define "common.names.fullname.namespace" -}}
{{- list (include "common.names.fullname" .) (include "common.names.namespace" .) | join "-" | trunc 63 | trimSuffix "-" -}}
{{- end -}}
