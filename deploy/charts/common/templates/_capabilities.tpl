{{/*
API versions of the workload kinds. Only the GA versions are left, since the charts require
Kubernetes 1.25 or later; the helpers stay so templates need no change if that ever differs.
*/}}
{{- define "common.capabilities.deployment.apiVersion" -}}apps/v1{{- end -}}
{{- define "common.capabilities.ingress.apiVersion" -}}networking.k8s.io/v1{{- end -}}
{{- define "common.capabilities.networkPolicy.apiVersion" -}}networking.k8s.io/v1{{- end -}}
{{- define "common.capabilities.policy.apiVersion" -}}policy/v1{{- end -}}
{{- define "common.capabilities.hpa.apiVersion" -}}autoscaling/v2{{- end -}}
