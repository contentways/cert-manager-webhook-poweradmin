{{/*
Name of the secret to use: .existingSecret (a name or a map with "name", may contain templates)
or the fullname.
  include "common.secrets.name" (dict "existingSecret" .Values.auth.existingSecret "context" $)
*/}}
{{- define "common.secrets.name" -}}
{{- $chosen := .existingSecret -}}
{{- if kindIs "map" $chosen -}}
  {{- $chosen = $chosen.name -}}
{{- end -}}
{{- $chosen = $chosen | default (include "common.names.fullname" .context) -}}
{{- include "common.tplvalues.render" (dict "value" $chosen "context" .context) -}}
{{- end -}}

{{/*
Base64 value of a chart-managed secret key that survives upgrades. In this order:
  1. the first non-empty of .providedValues (dotted paths into .Values),
  2. the value stored in the cluster under .secret/.key,
  3. a new random value of .length characters (default 10): letters and digits, with "strong"
     also at least one lower case, upper case, digit and symbol each, with "hex" .length random
     bytes as hex digits.
"failOnNew" fails instead of generating, for values that have to match another system.
  include "common.secrets.passwords.manage" (dict "secret" $name "key" "password" "providedValues" (list "auth.password") "context" $)
Under "helm template" there is no cluster to read from, so step 2 never applies there.
*/}}
{{- define "common.secrets.passwords.manage" -}}
{{- $root := .context -}}
{{- $provided := "" -}}
{{- range $path := .providedValues -}}
  {{- if not $provided -}}
    {{- $node := $root.Values -}}
    {{- range splitList "." $path -}}
      {{- if kindIs "map" $node -}}
        {{- $node = get $node . -}}
      {{- else -}}
        {{- $node = "" -}}
      {{- end -}}
    {{- end -}}
    {{- if $node -}}
      {{- $provided = toString $node -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- $stored := (lookup "v1" "Secret" (include "common.names.namespace" $root) .secret).data | default dict -}}
{{- $size := .length | default 10 | int -}}
{{- if $provided -}}
  {{- b64enc $provided -}}
{{- else if hasKey $stored .key -}}
  {{- get $stored .key -}}
{{- else if .failOnNew -}}
  {{- fail (printf "%s: no value for %q; set %s or point the chart at an existing secret" .secret .key (join " or " .providedValues)) -}}
{{- else if .hex -}}
  {{- randBytes $size | b64dec | printf "%x" | b64enc -}}
{{- else if .strong -}}
  {{- $symbol := index (list "@" "_" "." "-") (mod (randNumeric 1 | atoi) 4) -}}
  {{- $mandatory := list (randAlpha 1 | lower) (randAlpha 1 | upper) (randNumeric 1) $symbol | join "" -}}
  {{- print $mandatory (randAlphaNum (max 0 (sub $size 4) | int)) | shuffle | b64enc -}}
{{- else -}}
  {{- randAlphaNum $size | b64enc -}}
{{- end -}}
{{- end -}}
