{{/*
Helpers for pod specs: resources, security contexts, scheduling and service accounts.
*/}}

{{/*
Requests and limits of a resources preset. The presets only give a container something to start
with; real installations should size .resources themselves.
  include "common.resources.preset" (dict "type" "small")
*/}}
{{- define "common.resources.preset" -}}
{{- /* requests cpu, requests memory, limits cpu, limits memory */ -}}
{{- $sizes := dict
      "nano"    (list "100m" "128Mi"  "150m" "192Mi")
      "micro"   (list "250m" "256Mi"  "375m" "384Mi")
      "small"   (list "500m" "512Mi"  "750m" "768Mi")
      "medium"  (list "500m" "1024Mi" "750m" "1536Mi")
      "large"   (list "1.0"  "2048Mi" "1.5"  "3072Mi")
      "xlarge"  (list "1.0"  "3072Mi" "3.0"  "6144Mi")
      "2xlarge" (list "1.0"  "3072Mi" "6.0"  "12288Mi") -}}
{{- $size := get $sizes .type -}}
{{- if not $size -}}
  {{- fail (printf "unknown resources preset %q, choose one of: %s" .type (keys $sizes | sortAlpha | join ", ")) -}}
{{- end -}}
{{- $requests := dict "cpu" (index $size 0) "memory" (index $size 1) "ephemeral-storage" "50Mi" -}}
{{- $limits := dict "cpu" (index $size 2) "memory" (index $size 3) "ephemeral-storage" "2Gi" -}}
{{- toYaml (dict "requests" $requests "limits" $limits) -}}
{{- end -}}

{{/*
.resources when set, else the preset; "none" (or no preset) renders nothing.
  include "common.resources.render" (dict "resources" .Values.resources "preset" .Values.resourcesPreset)
*/}}
{{- define "common.resources.render" -}}
{{- if .resources -}}
  {{- toYaml .resources -}}
{{- else if not (has (.preset | default "none") (list "none")) -}}
  {{- include "common.resources.preset" (dict "type" .preset) -}}
{{- end -}}
{{- end -}}

{{/* "true" on OpenShift, detected by its security API. */}}
{{- define "common.compatibility.isOpenshift" -}}
{{- if .Capabilities.APIVersions.Has "security.openshift.io/v1" -}}
true
{{- end -}}
{{- end -}}

{{/*
A pod or container securityContext from values. Drops the chart's own "enabled" switch and
empty optional lists/maps. On OpenShift the restricted SCC hands out UIDs and GIDs itself, so
runAsUser, runAsGroup and fsGroup are removed there; global.compatibility.openshift.adaptSecurityContext
selects when: "auto" (detect, default), "force" (always) or anything else (never).
  include "common.compatibility.renderSecurityContext" (dict "secContext" .Values.podSecurityContext "context" $)
*/}}
{{- define "common.compatibility.renderSecurityContext" -}}
{{- $result := omit .secContext "enabled" -}}
{{- range $optional := list "seLinuxOptions" "sysctls" "supplementalGroups" -}}
  {{- if not (get $result $optional) -}}
    {{- $result = omit $result $optional -}}
  {{- end -}}
{{- end -}}
{{- $adapt := ((((.context.Values.global).compatibility).openshift).adaptSecurityContext) | default "auto" -}}
{{- $onOpenshift := include "common.compatibility.isOpenshift" .context -}}
{{- if or (eq $adapt "force") (and (eq $adapt "auto") $onOpenshift) -}}
  {{- $result = omit $result "runAsUser" "runAsGroup" "fsGroup" -}}
{{- end -}}
{{- toYaml $result -}}
{{- end -}}

{{/*
podAffinity/podAntiAffinity that prefers ("soft") or requires ("hard") spreading over nodes
relative to the pods of this release; other types render nothing.
  include "common.affinities.pods" (dict "type" .Values.podAntiAffinityPreset "customLabels" $podLabels "context" $)
*/}}
{{- define "common.affinities.pods" -}}
{{- $selector := dict "matchLabels" (include "common.labels.matchLabels" . | fromYaml) -}}
{{- $term := dict "labelSelector" $selector "topologyKey" "kubernetes.io/hostname" -}}
{{- if eq .type "soft" -}}
  {{- toYaml (dict "preferredDuringSchedulingIgnoredDuringExecution" (list (dict "weight" 1 "podAffinityTerm" $term))) -}}
{{- else if eq .type "hard" -}}
  {{- toYaml (dict "requiredDuringSchedulingIgnoredDuringExecution" (list $term)) -}}
{{- end -}}
{{- end -}}

{{/*
nodeAffinity for nodes whose label .key has one of .values, preferred ("soft") or required ("hard").
  include "common.affinities.nodes" (dict "type" "hard" "key" "kubernetes.io/arch" "values" (list "arm64"))
*/}}
{{- define "common.affinities.nodes" -}}
{{- $values := list -}}
{{- range .values -}}
  {{- $values = append $values (toString .) -}}
{{- end -}}
{{- $match := dict "matchExpressions" (list (dict "key" .key "operator" "In" "values" $values)) -}}
{{- if eq .type "soft" -}}
  {{- toYaml (dict "preferredDuringSchedulingIgnoredDuringExecution" (list (dict "weight" 1 "preference" $match))) -}}
{{- else if eq .type "hard" -}}
  {{- toYaml (dict "requiredDuringSchedulingIgnoredDuringExecution" (dict "nodeSelectorTerms" (list $match))) -}}
{{- end -}}
{{- end -}}

{{/*
ServiceAccount to run as: .name, or the fullname when the chart creates it; "default" otherwise.
  include "common.utils.serviceAccountName" (dict "serviceAccount" .Values.serviceAccount "context" $)
*/}}
{{- define "common.utils.serviceAccountName" -}}
{{- $fallback := ternary (include "common.names.fullname" .context) "default" (.serviceAccount.create | default false) -}}
{{- .serviceAccount.name | default $fallback -}}
{{- end -}}

{{/*
sha256 of another template of the chart without apiVersion, kind and metadata, as a pod
annotation value that changes, and so restarts the pods, whenever that object's content changes.
  include "common.utils.checksumTemplate" (dict "path" "/secrets.yaml" "context" $)
*/}}
{{- define "common.utils.checksumTemplate" -}}
{{- $object := include (print .context.Template.BasePath .path) .context | fromYaml -}}
{{- $content := omit $object "apiVersion" "kind" "metadata" -}}
{{- toYaml $content | sha256sum -}}
{{- end -}}
