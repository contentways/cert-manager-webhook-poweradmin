{{/*
Renders a value from values.yaml as YAML, evaluating templates in it.
  include "common.tplvalues.render" (dict "value" .Values.extraEnv "context" $)
*/}}
{{- define "common.tplvalues.render" -}}
{{- $text := .value -}}
{{- if not (kindIs "string" $text) -}}
  {{- $text = toYaml $text -}}
{{- end -}}
{{- if contains "{{" $text -}}
  {{- tpl $text .context -}}
{{- else -}}
  {{- $text -}}
{{- end -}}
{{- end -}}

{{/*
Renders several maps and combines them; for a key set in more than one, the earlier map wins.
  include "common.tplvalues.merge" (dict "values" (list .Values.podLabels .Values.commonLabels) "context" $)
*/}}
{{- define "common.tplvalues.merge" -}}
{{- $combined := dict -}}
{{- range $item := .values -}}
  {{- $rendered := include "common.tplvalues.render" (dict "value" $item "context" $.context) | fromYaml -}}
  {{- $combined = merge $combined $rendered -}}
{{- end -}}
{{- toYaml $combined -}}
{{- end -}}
