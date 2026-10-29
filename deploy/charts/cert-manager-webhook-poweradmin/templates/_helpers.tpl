{{/* vim: set filetype=mustache: */}}

{{/*
Webhook image reference.
*/}}
{{- define "cmwp.image" -}}
{{- include "common.images.image" (dict "imageRoot" .Values.image "global" .Values.global "chart" .Chart) -}}
{{- end -}}

{{/*
imagePullSecrets of the pod.
*/}}
{{- define "cmwp.imagePullSecrets" -}}
{{- include "common.images.renderPullSecrets" (dict "images" (list .Values.image) "context" $) -}}
{{- end -}}

{{/*
ServiceAccount name.
*/}}
{{- define "cmwp.serviceAccountName" -}}
{{- include "common.utils.serviceAccountName" (dict "serviceAccount" .Values.serviceAccount "context" $) -}}
{{- end -}}

{{/*
Name of cluster-scoped objects; includes the namespace so releases in different namespaces do not collide.
*/}}
{{- define "cmwp.clusterName" -}}
{{- include "common.names.fullname.namespace" . -}}
{{- end -}}

{{/*
Names of the PKI objects.
*/}}
{{- define "cmwp.selfSignedIssuer" -}}
{{- printf "%s-selfsign" (include "common.names.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "cmwp.caCertificate" -}}
{{- printf "%s-ca" (include "common.names.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "cmwp.caIssuer" -}}
{{- printf "%s-ca" (include "common.names.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "cmwp.servingCertificate" -}}
{{- printf "%s-tls" (include "common.names.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Validate values.
*/}}
{{- define "cmwp.validateValues" -}}
{{- $messages := list -}}
{{- if not (regexMatch "^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)+$" .Values.groupName) -}}
{{- $messages = append $messages (printf "groupName %q must be a DNS subdomain with at least one dot, e.g. acme.example.com" .Values.groupName) -}}
{{- end -}}
{{- if and (not .Values.automountServiceAccountToken) (not .Values.serviceAccount.automountServiceAccountToken) -}}
{{- $messages = append $messages "automountServiceAccountToken: the webhook needs the service account token to read Secrets and to delegate authentication" -}}
{{- end -}}
{{- if $messages -}}
{{- printf "\nVALUES VALIDATION:\n%s" (join "\n" $messages) | fail -}}
{{- end -}}
{{- end -}}
