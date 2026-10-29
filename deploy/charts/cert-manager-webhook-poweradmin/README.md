# cert-manager webhook for Poweradmin Helm chart

[cert-manager-webhook-poweradmin](https://github.com/contentways/cert-manager-webhook-poweradmin)
is a cert-manager DNS01 webhook that solves ACME challenges, wildcards included, by writing the
TXT records through the [Poweradmin](https://www.poweradmin.org) API.

The chart runs the webhook unprivileged with a read-only root filesystem, registers it as an
aggregated API with the kube-apiserver and lets cert-manager issue its serving certificate. It
builds on the [`common`](../common) library chart of this repository.

## TL;DR

```console
helm install cert-manager-webhook-poweradmin \
  oci://ghcr.io/contentways/charts/cert-manager-webhook-poweradmin --namespace cert-manager
```

## Prerequisites

- Kubernetes 1.25+
- Helm 3.8+ or Helm 4
- cert-manager 1.x, installed before this chart (its CRDs and cainjector are needed)
- Poweradmin 4.5.0 or later with the API enabled and an API key

## Installing the chart

```console
helm install my-release oci://ghcr.io/contentways/charts/cert-manager-webhook-poweradmin \
  --namespace cert-manager
```

Install the chart into cert-manager's namespace or any other one; `certManager.namespace` and
`certManager.serviceAccountName` must point at the cert-manager controller, which is
`cert-manager/cert-manager` for the official cert-manager chart.

## Configuration and installation details

### Using the webhook in an issuer

Store the Poweradmin API key in a Secret next to the issuer and reference the solver
`poweradmin` with the chart's `groupName`:

```yaml
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: letsencrypt
  namespace: customer-a
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: hostmaster@customer-a.example
    privateKeySecretRef:
      name: letsencrypt-account
    solvers:
      - dns01:
          webhook:
            groupName: acme.contentways.org
            solverName: poweradmin
            config:
              baseURL: https://dns.example.com
              apiKeySecretRef:
                name: poweradmin-api-key
                key: api-key
```

All solver options are described in the
[webhook's README](https://github.com/contentways/cert-manager-webhook-poweradmin#options).

### Access to the API key Secrets

The webhook reads the API key from a Secret in the issuer's namespace, or from cert-manager's
cluster resource namespace (`cert-manager` by default) for a `ClusterIssuer`. By default it may
read Secrets in every namespace. To narrow that down, list the namespaces and optionally the
Secret names:

```yaml
secretAccess:
  namespaces: [customer-a, customer-b, cert-manager]
  resourceNames: [poweradmin-api-key]
```

The chart then creates a Role and RoleBinding in each listed namespace instead of a ClusterRole.
An issuer in a namespace that is not listed fails with a `forbidden` error until the namespace is
added.

### Several customers

Give every customer namespace its own `Issuer` and its own Poweradmin API key that is limited to
that customer's zones. One webhook release serves all of them; the API key, and with it the zones
an issuer may change, always comes from the issuer's own namespace.

### API group

`groupName` is the API group the webhook registers with the kube-apiserver, and every issuer
names it in `solvers[].dns01.webhook.groupName`. The default `acme.contentways.org` only needs to
change when two installations of this webhook run in one cluster; each release needs its own
group.

### Certificates

The chart creates a self-signed CA with cert-manager and issues the serving certificate of the
webhook from it; cert-manager's cainjector puts the CA into the APIService. The lifetimes are set
with `pki.caDuration` and `pki.servingCertificateDuration`.

### Security

The pod runs as the distroless nonroot user (65532) with a read-only root filesystem, no
capabilities and the `RuntimeDefault` seccomp profile. On OpenShift the fixed user and group are
dropped automatically (`global.compatibility.openshift.adaptSecurityContext`).

## Parameters

### Global parameters

| Name                                                  | Description                                                                                                                      | Value  |
| ----------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------ |
| `global.imageRegistry`                                | Global container image registry                                                                                                  | `""`   |
| `global.imagePullSecrets`                             | Global registry secret names as an array                                                                                         | `[]`   |
| `global.compatibility.openshift.adaptSecurityContext` | Drop fixed runAsUser, runAsGroup and fsGroup so the restricted SCC can assign its own (auto: only on OpenShift, force, disabled) | `auto` |

### Common parameters

| Name                | Description                                                                 | Value |
| ------------------- | --------------------------------------------------------------------------- | ----- |
| `kubeVersion`       | Override the Kubernetes version detected from the cluster                   | `""`  |
| `apiVersions`       | Override the API versions detected from the cluster (used by helm template) | `[]`  |
| `nameOverride`      | Replaces the chart name in the object names                                 | `""`  |
| `fullnameOverride`  | Replaces the generated base name of all objects                             | `""`  |
| `namespaceOverride` | Namespace for all objects instead of the release namespace                  | `""`  |
| `commonLabels`      | Labels set on every object of the release                                   | `{}`  |
| `commonAnnotations` | Annotations set on every object of the release                              | `{}`  |
| `extraDeploy`       | Additional manifests installed with the release; may contain templates      | `[]`  |

### Webhook parameters

| Name                             | Description                                                                               | Value                  |
| -------------------------------- | ----------------------------------------------------------------------------------------- | ---------------------- |
| `groupName`                      | API group of the webhook; issuers reference it as solvers[].dns01.webhook.groupName       | `acme.contentways.org` |
| `certManager.namespace`          | Namespace of cert-manager                                                                 | `cert-manager`         |
| `certManager.serviceAccountName` | ServiceAccount of the cert-manager controller                                             | `cert-manager`         |
| `secretAccess.namespaces`        | Namespaces whose Secrets the webhook may read; empty grants access in all namespaces      | `[]`                   |
| `secretAccess.resourceNames`     | Names of the Secrets the webhook may read; empty allows every Secret in those namespaces  | `[]`                   |
| `pki.caDuration`                 | Lifetime of the CA certificate                                                            | `43800h`               |
| `pki.servingCertificateDuration` | Lifetime of the serving certificate                                                       | `8760h`                |
| `logLevel`                       | klog verbosity of the webhook; presented and cleaned up records are logged at every level | `2`                    |

### Image parameters

| Name                | Description                                                      | Value                                         |
| ------------------- | ---------------------------------------------------------------- | --------------------------------------------- |
| `image.registry`    | Webhook image registry                                           | `ghcr.io`                                     |
| `image.repository`  | Webhook image repository                                         | `contentways/cert-manager-webhook-poweradmin` |
| `image.digest`      | Webhook image digest in the form sha256:aa.... Wins over the tag | `""`                                          |
| `image.pullPolicy`  | Webhook image pull policy                                        | `IfNotPresent`                                |
| `image.pullSecrets` | Webhook image pull secrets                                       | `[]`                                          |

### Deployment parameters

| Name                                                | Description                                                                                                | Value            |
| --------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- | ---------------- |
| `replicaCount`                                      | Number of webhook replicas                                                                                 | `1`              |
| `revisionHistoryLimit`                              | Number of old ReplicaSets to retain                                                                        | `10`             |
| `updateStrategy.type`                               | Deployment strategy type                                                                                   | `RollingUpdate`  |
| `containerPorts.https`                              | HTTPS port of the webhook's API server                                                                     | `8443`           |
| `command`                                           | Override the default container command                                                                     | `[]`             |
| `args`                                              | Override the default container args                                                                        | `[]`             |
| `extraArgs`                                         | Extra arguments for the webhook, e.g. --tls-min-version=VersionTLS13                                       | `[]`             |
| `extraEnvVars`                                      | Extra environment variables for the webhook container                                                      | `[]`             |
| `extraEnvVarsCM`                                    | Name of a ConfigMap whose keys become environment variables                                                | `""`             |
| `extraEnvVarsSecret`                                | Name of a Secret whose keys become environment variables                                                   | `""`             |
| `podSecurityContext.enabled`                        | Enable the pod security context                                                                            | `true`           |
| `podSecurityContext.fsGroupChangePolicy`            | When to change volume ownership                                                                            | `Always`         |
| `podSecurityContext.sysctls`                        | Kernel settings via sysctl                                                                                 | `[]`             |
| `podSecurityContext.supplementalGroups`             | Additional groups for the first process                                                                    | `[]`             |
| `podSecurityContext.fsGroup`                        | Group owning the volumes                                                                                   | `65532`          |
| `podSecurityContext.seccompProfile.type`            | Seccomp profile                                                                                            | `RuntimeDefault` |
| `containerSecurityContext.enabled`                  | Enable the container security context                                                                      | `true`           |
| `containerSecurityContext.seLinuxOptions`           | SELinux options                                                                                            | `{}`             |
| `containerSecurityContext.runAsUser`                | User ID                                                                                                    | `65532`          |
| `containerSecurityContext.runAsGroup`               | Group ID                                                                                                   | `65532`          |
| `containerSecurityContext.runAsNonRoot`             | Require a non-root user                                                                                    | `true`           |
| `containerSecurityContext.privileged`               | Run privileged                                                                                             | `false`          |
| `containerSecurityContext.readOnlyRootFilesystem`   | Mount the root filesystem read-only                                                                        | `true`           |
| `containerSecurityContext.allowPrivilegeEscalation` | Allow privilege escalation                                                                                 | `false`          |
| `containerSecurityContext.capabilities.drop`        | Capabilities to drop                                                                                       | `["ALL"]`        |
| `containerSecurityContext.seccompProfile.type`      | Seccomp profile                                                                                            | `RuntimeDefault` |
| `resourcesPreset`                                   | Resources preset (none, nano, micro, small, medium, large, xlarge, 2xlarge); ignored when resources is set | `nano`           |
| `resources`                                         | Container requests and limits, e.g. requests.cpu: 50m, limits.memory: 128Mi                                | `{}`             |
| `livenessProbe.enabled`                             | Run a liveness probe                                                                                       | `true`           |
| `livenessProbe.initialDelaySeconds`                 | Seconds after container start before the first liveness check                                              | `5`              |
| `livenessProbe.periodSeconds`                       | Seconds between liveness checks                                                                            | `10`             |
| `livenessProbe.timeoutSeconds`                      | Seconds after which a liveness check times out                                                             | `5`              |
| `livenessProbe.failureThreshold`                    | Failed liveness checks in a row before the probe fails                                                     | `6`              |
| `livenessProbe.successThreshold`                    | Successful liveness checks in a row before the probe passes again                                          | `1`              |
| `readinessProbe.enabled`                            | Run a readiness probe                                                                                      | `true`           |
| `readinessProbe.initialDelaySeconds`                | Seconds after container start before the first readiness check                                             | `5`              |
| `readinessProbe.periodSeconds`                      | Seconds between readiness checks                                                                           | `10`             |
| `readinessProbe.timeoutSeconds`                     | Seconds after which a readiness check times out                                                            | `5`              |
| `readinessProbe.failureThreshold`                   | Failed readiness checks in a row before the probe fails                                                    | `3`              |
| `readinessProbe.successThreshold`                   | Successful readiness checks in a row before the probe passes again                                         | `1`              |
| `customLivenessProbe`                               | Complete liveness probe that replaces the built-in one                                                     | `{}`             |
| `customReadinessProbe`                              | Complete readiness probe that replaces the built-in one                                                    | `{}`             |
| `lifecycleHooks`                                    | Lifecycle hooks for the webhook container                                                                  | `{}`             |
| `automountServiceAccountToken`                      | Mount the service account token in the pod (the webhook needs it to read Secrets)                          | `true`           |
| `hostAliases`                                       | Webhook pod host aliases                                                                                   | `[]`             |
| `podLabels`                                         | Extra labels for webhook pods                                                                              | `{}`             |
| `podAnnotations`                                    | Annotations for webhook pods                                                                               | `{}`             |
| `podAffinityPreset`                                 | Pod affinity preset (soft or hard). Ignored if affinity is set                                             | `""`             |
| `podAntiAffinityPreset`                             | Pod anti-affinity preset (soft or hard). Ignored if affinity is set                                        | `soft`           |
| `nodeAffinityPreset.type`                           | Node affinity preset type (soft or hard). Ignored if affinity is set                                       | `""`             |
| `nodeAffinityPreset.key`                            | Node label key to match                                                                                    | `""`             |
| `nodeAffinityPreset.values`                         | Node label values to match                                                                                 | `[]`             |
| `affinity`                                          | Affinity for webhook pods (evaluated as a template)                                                        | `{}`             |
| `nodeSelector`                                      | Node labels for webhook pods (evaluated as a template)                                                     | `{}`             |
| `tolerations`                                       | Tolerations for webhook pods (evaluated as a template)                                                     | `[]`             |
| `topologySpreadConstraints`                         | Topology spread constraints for webhook pods (evaluated as a template)                                     | `[]`             |
| `priorityClassName`                                 | Webhook pods' priorityClassName                                                                            | `""`             |
| `schedulerName`                                     | Scheduler for the pods; empty means the default scheduler                                                  | `""`             |
| `terminationGracePeriodSeconds`                     | Seconds webhook pods need to terminate gracefully                                                          | `""`             |
| `extraVolumes`                                      | Optionally specify extra list of additional volumes for the webhook pods                                   | `[]`             |
| `extraVolumeMounts`                                 | Optionally specify extra list of additional volumeMounts for the webhook container                         | `[]`             |
| `sidecars`                                          | Add additional sidecar containers to the webhook pods                                                      | `[]`             |
| `initContainers`                                    | Add additional init containers to the webhook pods                                                         | `[]`             |
| `pdb.create`                                        | Create a PodDisruptionBudget                                                                               | `true`           |
| `pdb.minAvailable`                                  | Pods (number or percentage) that must stay available during voluntary disruptions                          | `""`             |
| `pdb.maxUnavailable`                                | Maximum number/percentage of unavailable pods; defaults to 1 when both are empty                           | `""`             |

### Service parameters

| Name                     | Description                                   | Value       |
| ------------------------ | --------------------------------------------- | ----------- |
| `service.type`           | Type of the Service                           | `ClusterIP` |
| `service.ports.https`    | HTTPS port of the Service                     | `443`       |
| `service.clusterIP`      | Service Cluster IP                            | `""`        |
| `service.annotations`    | Additional custom annotations for the Service | `{}`        |
| `service.ipFamilyPolicy` | IP family policy of the Service               | `""`        |
| `service.ipFamilies`     | IP families of the Service                    | `[]`        |

### Other parameters

| Name                                          | Description                                                        | Value   |
| --------------------------------------------- | ------------------------------------------------------------------ | ------- |
| `serviceAccount.create`                       | Create a ServiceAccount                                            | `true`  |
| `serviceAccount.name`                         | Name of the ServiceAccount; generated from the fullname when empty | `""`    |
| `serviceAccount.automountServiceAccountToken` | Automount the token of the created ServiceAccount                  | `false` |
| `serviceAccount.annotations`                  | Annotations for the ServiceAccount                                 | `{}`    |

## License

The chart is licensed under the Apache License 2.0. The webhook itself is licensed under the MIT
license.
