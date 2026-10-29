# cert-manager-webhook-poweradmin

A [cert-manager](https://cert-manager.io) DNS01 webhook solver for
[Poweradmin](https://www.poweradmin.org). It writes the ACME challenge TXT
records through the Poweradmin API v2, so certificates, including wildcards,
can be issued for zones that are managed with Poweradmin and served by PowerDNS.

- Uses the Poweradmin API with an API key, no direct PowerDNS access needed
- One API key per issuer: namespaced `Issuer`s give each tenant its own key
  and zones
- Apex and wildcard challenges share one TXT RRset safely; clean-up removes
  only its own value
- Passes the cert-manager DNS01 conformance suite against Poweradmin and
  PowerDNS

Tested with Poweradmin 4.5.0 and PowerDNS 4.8.

## Installation

Install cert-manager first, then the webhook with its Helm chart. The chart lives
in [deploy/charts](deploy/charts/cert-manager-webhook-poweradmin) and is
published with every release, in the same version as the image:

```sh
helm install cert-manager-webhook-poweradmin \
  oci://ghcr.io/contentways/charts/cert-manager-webhook-poweradmin \
  --namespace cert-manager
```

The webhook registers the API group `acme.contentways.org` by default. The
issuers below use that group; change both together if you set another
`groupName`.

## Configuration

### API key

Create an API key in Poweradmin under **API Keys** and limit it to the zones the
issuer may change. The key needs to read zones and to change and delete RRsets.
Store it in a Secret:

```sh
kubectl create secret generic poweradmin-api-key \
  --namespace customer-a \
  --from-literal=api-key='<key>'
```

### Issuer

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

A Certificate in the same namespace then works as usual:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: www
  namespace: customer-a
spec:
  secretName: www-tls
  dnsNames:
    - customer-a.example
    - "*.customer-a.example"
  issuerRef:
    name: letsencrypt
    kind: Issuer
```

### Options

| Field | Required | Description |
| --- | --- | --- |
| `baseURL` | yes | Poweradmin URL without `/api`, e.g. `https://dns.example.com` |
| `apiKeySecretRef.name` | yes | Secret with the API key |
| `apiKeySecretRef.key` | yes | Key in the Secret |
| `ttl` | no | TTL of the TXT record in seconds, default `120` |
| `zone` | no | Poweradmin zone to write to; by default the zone cert-manager found, then its parent zones |
| `caBundle` | no | PEM CA certificates for a Poweradmin behind an internal CA |
| `insecureSkipVerify` | no | Skip TLS verification, for testing only |

## Several customers or namespaces

The Secret is read from the namespace of the issuer:

- **`Issuer`** (namespaced): the Secret comes from the issuer's namespace. Give
  every customer namespace its own `Issuer` and its own API key that is limited
  to that customer's zones in Poweradmin. A customer can then neither use nor
  see another customer's key, and Poweradmin refuses changes outside its zones.
- **`ClusterIssuer`**: the Secret comes from cert-manager's cluster resource
  namespace (`cert-manager` by default). Use it when one team manages all zones.

Customers on different Poweradmin instances simply set a different `baseURL`
in their `Issuer`.

The webhook needs read access to these Secrets. The Helm chart grants it either
cluster-wide or only in a list of namespaces; see the chart's
`secretAccess` values.

Clean-up does not block deleting a namespace. When the webhook has no access
to the zone, because the Secret is gone or unreadable, the API key is rejected
or the zone is not shared with the key, there is nothing it could remove: it
logs `skipping clean-up without access to the zone` with the record name and
lets cert-manager delete the Challenge. If the record had been presented
before the access was lost, remove it in Poweradmin by hand. Transient errors,
such as an unavailable Poweradmin, are still retried.

### CNAME delegation

If the API key should not be able to change a customer's main zone, delegate
the challenge to a separate zone:

```text
_acme-challenge.customer-a.example. CNAME _acme-challenge.customer-a.acme.example.com.
```

and limit the key to `acme.example.com`. cert-manager follows the CNAME when the
issuer's solver sets `cnameStrategy: Follow`:

```yaml
    solvers:
      - dns01:
          cnameStrategy: Follow
          webhook:
            groupName: acme.contentways.org
            solverName: poweradmin
            config:
              baseURL: https://dns.example.com
              apiKeySecretRef:
                name: poweradmin-api-key
                key: api-key
```

## Development

```sh
make test               # unit tests
make lint               # golangci-lint
make test-conformance   # cert-manager conformance suite, see below
make image              # container image
make chart-lint         # helm lint of the chart
make chart-readme       # regenerate the chart's parameter table and values schema (needs npx)
```

`make test-conformance` starts Poweradmin and PowerDNS on a shared SQLite
database with `hack/e2e/up.sh` and runs the cert-manager conformance suite
against them. It needs git, PHP with the `pdo_sqlite`, `intl` and `gettext`
extensions, python3, openssl, `dig` and PowerDNS with the SQLite backend
(`pdns-server` and `pdns-backend-sqlite3` on Debian and Ubuntu). The envtest
binaries are downloaded with `setup-envtest`; set `ENVTEST_ASSETS` to use a
directory that already has them. `POWERADMIN_VERSION` picks the Poweradmin
release, `v4.5.0` by default.

To run the suite against another Poweradmin, set `POWERADMIN_URL`,
`POWERADMIN_API_KEY`, `POWERADMIN_ZONE` and optionally `POWERADMIN_DNS_SERVER`
and the `TEST_ASSET_*` variables, then run
`go test -tags conformance -run TestConformance .`. Without a DNS server the
test answers the TXT queries from the Poweradmin API.

## License

[MIT](LICENSE.md)
