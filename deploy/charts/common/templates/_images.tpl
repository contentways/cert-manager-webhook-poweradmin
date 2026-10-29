{{/*
Image reference "[registry/]repository(:tag|@digest)". global.imageRegistry replaces the
registry, a digest takes precedence over the tag and an empty tag means the appVersion.
  include "common.images.image" (dict "imageRoot" .Values.image "global" .Values.global "chart" .Chart)
*/}}
{{- define "common.images.image" -}}
{{- $image := .imageRoot -}}
{{- $parts := list -}}
{{- $registry := (.global).imageRegistry | default $image.registry -}}
{{- if $registry -}}
  {{- $parts = append $parts $registry -}}
{{- end -}}
{{- $reference := join "/" (append $parts $image.repository) -}}
{{- if $image.digest -}}
  {{- printf "%s@%s" $reference (toString $image.digest) -}}
{{- else -}}
  {{- $tag := toString ($image.tag | default "") -}}
  {{- if and (not $tag) .chart -}}
    {{- $tag = toString .chart.AppVersion -}}
  {{- end -}}
  {{- printf "%s:%s" $reference $tag -}}
{{- end -}}
{{- end -}}

{{/*
imagePullSecrets block for a pod: global.imagePullSecrets followed by the pullSecrets of each
image, without duplicates. Entries are names or {name: ...} maps and may contain templates.
  include "common.images.renderPullSecrets" (dict "images" (list .Values.image) "context" $)
*/}}
{{- define "common.images.renderPullSecrets" -}}
{{- $root := .context -}}
{{- $entries := ($root.Values.global).imagePullSecrets | default list -}}
{{- range .images -}}
  {{- $entries = concat $entries (.pullSecrets | default list) -}}
{{- end -}}
{{- $secretNames := list -}}
{{- range $entries -}}
  {{- $entry := . -}}
  {{- if kindIs "map" . -}}
    {{- $entry = .name -}}
  {{- end -}}
  {{- $secretNames = append $secretNames (include "common.tplvalues.render" (dict "value" $entry "context" $root)) -}}
{{- end -}}
{{- with uniq $secretNames }}
imagePullSecrets:
{{- range . }}
  - name: {{ . }}
{{- end }}
{{- end }}
{{- end -}}
