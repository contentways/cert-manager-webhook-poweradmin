{{/*
Recommended labels (app.kubernetes.io/*, helm.sh/chart) plus custom ones.
  include "common.labels.standard" (dict "customLabels" .Values.commonLabels "context" $)
customLabels may contain templates and may override everything except the two selector labels.
*/}}
{{- define "common.labels.standard" -}}
{{- $root := .context -}}
{{- $labels := dict
      "app.kubernetes.io/instance" $root.Release.Name
      "app.kubernetes.io/managed-by" $root.Release.Service
      "app.kubernetes.io/name" (include "common.names.name" $root)
      "helm.sh/chart" (include "common.names.chart" $root) -}}
{{- if $root.Chart.AppVersion -}}
  {{- $_ := set $labels "app.kubernetes.io/version" ($root.Chart.AppVersion | replace "+" "_" | trunc 63 | trimSuffix "-") -}}
{{- end -}}
{{- $custom := include "common.tplvalues.render" (dict "value" (.customLabels | default dict) "context" $root) | fromYaml -}}
{{- range $key, $value := omit $custom "app.kubernetes.io/name" "app.kubernetes.io/instance" -}}
  {{- $_ := set $labels $key $value -}}
{{- end -}}
{{- toYaml $labels -}}
{{- end -}}

{{/*
Selector labels. They end up in immutable fields, so they never include custom labels.
  include "common.labels.matchLabels" (dict "customLabels" .Values.podLabels "context" $)
*/}}
{{- define "common.labels.matchLabels" -}}
app.kubernetes.io/name: {{ include "common.names.name" .context }}
app.kubernetes.io/instance: {{ .context.Release.Name }}
{{- end -}}
