{{/*
Checks and hints shown at install time.
*/}}

{{/*
Fails with all non-empty messages at once, so a user sees every problem in one run.
  include "common.validations.render" (dict "messages" $messages)
*/}}
{{- define "common.validations.render" -}}
{{- $problems := without .messages "" -}}
{{- if $problems -}}
  {{- fail (printf "\nThe values have %d problem(s):\n%s" (len $problems) (join "\n" $problems)) -}}
{{- end -}}
{{- end -}}

{{/*
NOTES hint for an image tag that points to different builds over time.
  include "common.warnings.rollingTag" .Values.image
*/}}
{{- define "common.warnings.rollingTag" -}}
{{- $tag := toString (.tag | default "") -}}
{{- if and (not .digest) (has $tag (list "latest" "stable" "dev" "lts" "main" "master")) }}
WARNING: "{{ $tag }}" is a moving tag, so pods may run different builds after a restart. Use a version tag or a digest.
{{- end }}
{{- end -}}

{{/*
NOTES hint for containers that fall back to a resources preset. "" stands for the top-level
resources/resourcesPreset, any other entry for a values section of that name.
  include "common.warnings.resources" (dict "sections" (list "" "metrics") "context" $)
*/}}
{{- define "common.warnings.resources" -}}
{{- $preset := list -}}
{{- range $section := .sections -}}
  {{- $values := ternary $.context.Values (index $.context.Values $section) (eq $section "") -}}
  {{- if and (not $values.resources) (ne ($values.resourcesPreset | default "none") "none") -}}
    {{- $preset = append $preset (ternary "resources" (printf "%s.resources" $section) (eq $section "")) -}}
  {{- end -}}
{{- end -}}
{{- if $preset }}

WARNING: these resources come from "resourcesPreset" ({{ join ", " $preset }}). Presets are a
rough starting point; set the resources to what the workload needs before production use.
{{- end -}}
{{- end -}}
