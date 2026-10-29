{{/*
"backend" of an Ingress rule; .servicePort is a port name or number.
  include "common.ingress.backend" (dict "serviceName" $name "servicePort" "http" "context" $)
*/}}
{{- define "common.ingress.backend" -}}
{{- $port := dict -}}
{{- if kindIs "string" .servicePort -}}
  {{- $_ := set $port "name" .servicePort -}}
{{- else if or (kindIs "int" .servicePort) (kindIs "int64" .servicePort) (kindIs "float64" .servicePort) -}}
  {{- $_ := set $port "number" (int .servicePort) -}}
{{- end -}}
{{- toYaml (dict "service" (dict "name" .serviceName "port" $port)) -}}
{{- end -}}

{{/*
"true" when the annotations make cert-manager's ingress-shim issue a certificate.
  include "common.ingress.certManagerRequest" (dict "annotations" .Values.ingress.annotations)
*/}}
{{- define "common.ingress.certManagerRequest" -}}
{{- $annotations := .annotations | default dict -}}
{{- $requested := false -}}
{{- range list "cert-manager.io/issuer" "cert-manager.io/cluster-issuer" "kubernetes.io/tls-acme" -}}
  {{- if hasKey $annotations . -}}
    {{- $requested = true -}}
  {{- end -}}
{{- end -}}
{{- if $requested -}}
true
{{- end -}}
{{- end -}}
