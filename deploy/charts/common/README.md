# common library chart

Shared template helpers of the application charts in this repository. As a
[library chart](https://helm.sh/docs/topics/library_charts/) it renders no objects itself.

## Usage

```yaml
# Chart.yaml
dependencies:
  - name: common
    version: 1.x.x
    repository: oci://ghcr.io/contentways/charts
```

The charts in this repository refer to it as `file://../common`. `helm package` bundles the
library into each chart archive, so a published chart has no external dependency.

```yaml
metadata:
  name: {{ include "common.names.fullname" . }}
  labels: {{- include "common.labels.standard" (dict "customLabels" .Values.commonLabels "context" $) | nindent 4 }}
```

Helpers that need the chart's root context take it as `"context" $`. The comment above each
`define` in `templates/` describes its arguments.

## Helpers

| File | Templates |
|------|-----------|
| `_names.tpl` | `common.names.name`, `common.names.chart`, `common.names.fullname`, `common.names.namespace`, `common.names.fullname.namespace` |
| `_labels.tpl` | `common.labels.standard`, `common.labels.matchLabels` |
| `_tplvalues.tpl` | `common.tplvalues.render`, `common.tplvalues.merge` |
| `_images.tpl` | `common.images.image`, `common.images.renderPullSecrets` |
| `_workload.tpl` | `common.resources.preset`, `common.resources.render`, `common.compatibility.isOpenshift`, `common.compatibility.renderSecurityContext`, `common.affinities.pods`, `common.affinities.nodes`, `common.utils.serviceAccountName`, `common.utils.checksumTemplate` |
| `_capabilities.tpl` | `common.capabilities.{deployment,ingress,networkPolicy,policy,hpa}.apiVersion` |
| `_secrets.tpl` | `common.secrets.name`, `common.secrets.passwords.manage` |
| `_ingress.tpl` | `common.ingress.backend`, `common.ingress.certManagerRequest` |
| `_storage.tpl` | `common.storage.class` |
| `_notes.tpl` | `common.validations.render`, `common.warnings.rollingTag`, `common.warnings.resources` |

### Generated secrets

`common.secrets.passwords.manage` keeps a generated value stable across upgrades. A value set in
the chart values comes first, then the value already stored in the cluster secret, and only then
a new random one. `"hex" true` generates `length` random bytes written as hex digits (session
keys). `"strong" true` guarantees lower case, upper case, a digit and a symbol. `"failOnNew" true`
fails instead of generating.

Under `helm template` and `--dry-run=client` there is no cluster to read from, so every run shows
new random values. Installs and upgrades keep the stored ones.

### Resource presets

| Preset | Requests (CPU / memory) | Limits (CPU / memory) |
|--------|-------------------------|-----------------------|
| nano | 100m / 128Mi | 150m / 192Mi |
| micro | 250m / 256Mi | 375m / 384Mi |
| small | 500m / 512Mi | 750m / 768Mi |
| medium | 500m / 1024Mi | 750m / 1536Mi |
| large | 1 / 2048Mi | 1.5 / 3072Mi |
| xlarge | 1 / 3072Mi | 3 / 6144Mi |
| 2xlarge | 1 / 3072Mi | 6 / 12288Mi |

Every preset also requests 50Mi and limits 2Gi of ephemeral storage. The presets give a
container something to start with. Production installs should size `resources` explicitly.

## License

Apache License 2.0, see [LICENSE](../LICENSE).
