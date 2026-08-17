# OIDC admin bootstrap for CamundaCluster, proven against Keycloak in kind

**Status:** approved in dialogue (2026-08-17)
**Date:** 2026-08-17
**Scope:** `CamundaPlatformConfig` (`spec.auth.oidc.usernameClaim`, `spec.auth.oidc.clientIdClaim`),
`CamundaCluster` and `CamundaClusterPreset` (`spec.auth.admin`), the OIDC render path of
`pkg/components/camundacluster`, new keys in `pkg/camundaconfig`, Keycloak in the kind e2e suite

## Summary

Today the operator renders authentication under `method: oidc` and nothing else. A cluster comes up
where every token is authenticated and no token is authorized. The basic-auth path does not have
this hole, because it seeds an admin user and gives that user the admin role
(`adminUserEnv` in `render.go`).

This design closes the hole. `CamundaPlatformConfig` gains the two claim names that identify a
principal in a token. `CamundaCluster` gains a `spec.auth.admin` block that lists which
identities from the identity provider hold the admin role of that cluster. The operator renders
both into the `camunda.security.*` configuration of the workloads.

The design also adds the end-to-end proof that issue #61 asks for. The kind suite gains a
Keycloak realm, and a third `CamundaCluster` flow runs against it with `method: oidc`.

## What Camunda 8.9 does

These facts come from the camunda/camunda checkout at tag 8.9.9. They decide the shape of the
API.

**A token names its principal through a claim.** `TokenClaimsConverter.convert` reads the
username claim and the client id claim from the token
(`authentication/.../converter/TokenClaimsConverter.java:38-62`). If the client id claim is
present, the caller is a client. If only the username claim is present, the caller is a user. If
neither is present, the call fails with an `INVALID_CLIENT` error. The username claim defaults
to `sub` and the client id claim defaults to unset
(`security-core/.../OidcAuthenticationConfiguration.java`).

**Role membership comes from static configuration.** `camunda.security.initialization.default-roles`
is a map of role id to member type to member ids
(`security-core/.../InitializationConfiguration.java:26`). `PlatformDefaultEntities.getEntityType`
accepts the member types `users`, `clients`, `mappingrules` (also spelled `mapping-rules`),
`groups`, and `roles`, and throws on any other key
(`zeebe/engine/.../PlatformDefaultEntities.java:269-282`). The role ids are `admin`, `rpa`,
`connectors`, `app-integrations`, and `task-worker` (`DefaultRole.java`).

**A mapping rule matches a claim.** `ConfiguredMappingRule` holds `mappingRuleId`, `claimName`,
and `claimValue`, and rejects an empty value for each
(`security-core/.../ConfiguredMappingRule.java`). The rules live under
`camunda.security.initialization.mapping-rules`. A rule becomes an admin through the
`mapping-rules` member type of the `admin` role.

**Authorizations are on by default.** `AuthorizationsConfiguration` defaults `enabled` to true
(`security-core/.../AuthorizationsConfiguration.java:12`). Two endpoints hide the hole today.
`TopologyServices.getTopology` runs no authorization check
(`service/.../TopologyServices.java:74`), so `GET /v2/topology` answers any authenticated caller.
The `AdminUserCheckFilter` only redirects to `/admin/setup` when no admin **user** is configured
(`authentication/.../filters/AdminUserCheckFilter.java:57-64`), so an admin client alone still
sends a first-time operator to the setup page.

## Goals

- A `CamundaCluster` with `method: oidc` can be administered through the CRD, with no manual step
  inside the running cluster.
- The claim names that describe a provider live on the platform config, because they describe the
  tokens of that provider and not one cluster.
- The admin members live on the cluster and therefore also in a preset baseline, because they
  answer "who administers this cluster".
- The connectors runtime works under OIDC without boilerplate in the spec.
- The kind suite proves the OIDC path against a real identity provider, to the same depth as the
  two basic-auth flows.

## Non-goals

- The operator never calls the identity provider. It creates no client, reads no discovery
  document, and holds no provider credentials. Provisioning identity-provider objects belongs to
  whoever owns the provider, in the same way that cloud infrastructure belongs to
  `camunda-cloud-operator`.
- No management of roles, groups, tenants, or authorizations beyond the admin role and the
  connectors role. Camunda serves an API and a user interface for the rest.
- No `groupsClaim`, no `preferUsernameClaim`, and no `scope` override. Add them when a user asks.
- No change to the basic-auth path.

## API surface

### CamundaPlatformConfig

`OIDCSpec` gains two optional fields. Both are empty by default, and an empty field renders
nothing, so the Camunda default applies.

```go
// UsernameClaim is the token claim that names a user...
UsernameClaim string `json:"usernameClaim,omitempty"`
// ClientIDClaim is the token claim that names a machine client...
ClientIDClaim string `json:"clientIdClaim,omitempty"`
```

### CamundaCluster and CamundaClusterPreset

`ClusterAuthSpec` gains an optional `admin` block. `CamundaClusterPresetSpec.Cluster` is a
`CamundaClusterSpec` (`camundaclusterpreset_types.go:36`), and `auth` is not an instance-bound
field, so a preset can carry the block with no further change to the preset type.

```go
// Admin grants the admin role of this cluster to identities from the
// identity provider. It applies under OIDC only.
Admin *ClusterAdminSpec `json:"admin,omitempty"`

type ClusterAdminSpec struct {
    Users        []string           `json:"users,omitempty"`
    Clients      []string           `json:"clients,omitempty"`
    MappingRules []AdminMappingRule `json:"mappingRules,omitempty"`
}

type AdminMappingRule struct {
    ID         string `json:"id"`
    ClaimName  string `json:"claimName"`
    ClaimValue string `json:"claimValue"`
}
```

The three member kinds match the three member types that `getEntityType` accepts for a user, a
client, and a rule. Each `AdminMappingRule` field is required and has `MinLength=1`, because
`ConfiguredMappingRule` rejects an empty value at broker startup.

### Preset merge

`mergeAuth` (`presetmerge.go:112`) gains one clause. A cluster that sets `admin` replaces the
`admin` block of the preset in full. A cluster that leaves it unset inherits the block of the
preset. This is the rule that `Scheduling` already uses (`presetmerge.go:87`).

Replacement, and not a union, is the choice here for two reasons. A reader of one manifest learns
who administers the cluster without also reading the preset. A cluster can drop an administrator
that the preset grants. A union can be added later without taking access away from anybody. The
reverse change would take access away silently.

## Rendering

`EffectiveAuth` gains an `Admin *v1.ClusterAdminSpec` field next to `OIDC`. `ResolveAuth`
(`input.go:93`) fills it from the effective cluster auth, and only when the method is OIDC.

`oidcEnv` (`render.go:234`) renders the claim names when they are set, then the admin bootstrap.
`adminUserEnv`, the basic-auth branch, does not change. The new keys in `pkg/camundaconfig`:

| Key | Purpose |
| --- | --- |
| `camunda.security.authentication.oidc.username-claim` | Claim that names a user |
| `camunda.security.authentication.oidc.client-id-claim` | Claim that names a client |
| `camunda.security.initialization.mapping-rules[N].mapping-rule-id` | Rule definition |
| `camunda.security.initialization.mapping-rules[N].claim-name` | Rule definition |
| `camunda.security.initialization.mapping-rules[N].claim-value` | Rule definition |
| `camunda.security.initialization.default-roles.admin.clients[N]` | Admin client member |
| `camunda.security.initialization.default-roles.admin.mapping-rules[N]` | Admin rule member |
| `camunda.security.initialization.default-roles.connectors.clients[N]` | Connectors client member |

`camunda.security.initialization.default-roles.admin.users` already exists and carries the users
of the block. Each new key gets a class-name GoDoc and an entry in the `notInDefaultsYAML` table
of `keys_source_test.go`, with the reason that the existing security keys use.

### The connectors grant

The connectors runtime authenticates with the OIDC client of the cluster (`render.go:313-323`). It
needs the `connectors` role to stream jobs. The operator renders
`default-roles.connectors.clients[0]` with the effective client id when three conditions hold: the
method is OIDC, connectors are enabled, and `clientIdClaim` is set.

The `clientIdClaim` condition is not decoration. Without that claim name the runtime authenticates
as a user, and a `clients` entry never matches it. The operator writes an entry that works, or it
writes nothing.

## Validation

`spec.auth.admin` under `method: basic` is ignored, not rejected. A `CamundaCluster` cannot read
the method of its `CamundaPlatformConfig` at admission time, because the platform config is a
separate cluster-scoped resource. The render path already resolves auth on the controller side,
and it simply emits nothing. The CRD docs state this.

The change adds no condition and no status field. The new variables travel on the existing
workloads. A change to them moves `camunda.io/config-hash` and rolls the pods, and `Ready` already
covers the result.

## The end-to-end proof

`test/e2e/testdata/keycloak.yaml` holds a Deployment, a Service, and a Secret with a realm
export. Keycloak runs `start-dev --import-realm`. The export is a Secret and not a ConfigMap
because it carries the client secret and the password of the user, which is how the PostgreSQL
fixture of the suite already holds its credentials. The realm `camunda` holds one confidential
client `camunda` with the standard flow and service accounts turned on, and one static user.

The client carries two protocol mappers. An audience mapper puts `camunda` in `aud`, because
Keycloak otherwise issues `account` and the `audiences` check fails. A hardcoded-claim mapper puts
`client_id: camunda` in the access token, so `clientIdClaim: client_id` resolves a machine token to
a client.

The issuer is the in-cluster Service URL of Keycloak. Nothing needs external DNS, because the
suite never completes a browser login.

`Describe("CamundaCluster with OIDC", Ordered)` runs in the namespace `camunda-oidc-e2e` on its
own `ElasticsearchCluster`. The client secret is applied into `camunda-operator-system`, the
manager namespace, so the flow also exercises the cross-namespace mirror
`<name>-camunda-oidc-client`. It is applied and not created, because that namespace outlives the
namespace of the flow and a run that ends before its cleanup must not fail the next one on
"already exists". The platform config sets
`clientIdClaim: client_id`, and the cluster sets `admin.clients: ["camunda"]` and an
`externalUrl`.

The specs of the flow:

1. `Ready` is `Healthy` and `MirroredSecretsReady` is `Healthy`.
2. A client-credentials token from Keycloak is accepted by `GET /v2/topology`.
3. A call to `GET /v2/topology` with no credentials is refused with 401.
4. A call to `/operate/` ends at the authorization endpoint of Keycloak, and the final URL carries
   `redirect_uri` with the value `<externalUrl>/sso-callback`.
5. `ConnectorsReady` is `Healthy`.
6. The token deploys a process, starts an instance, and finds it in secondary storage. This spec
   fails without the admin grant, so it is the proof that the grant works.

`utils.CamundaREST` sends basic auth today (`test/utils/camunda.go:89`). It gains two more modes.
One mode reads a client-credentials token and sends it as a bearer token. Both requests happen in
the one helper pod, so the token never reaches the test output. The other mode sends no
credentials, for spec 3.

## Design decisions

**The claim names live on the platform config, the members live on the cluster.** A claim name is
a property of the tokens of one identity provider. Every cluster that trusts that provider reads
the same claim. The members answer a different question, and the answer differs per cluster. This
split also means the members reach a preset for free, and the claim names cannot drift between two
clusters that share a provider.

**Three member kinds, not one.** Mapping rules alone would be the smallest surface, and Camunda
documentation prefers them. They also force a rule definition for "this one client is an admin",
which is the common case and the case that the e2e needs. Users and clients alone would leave
claim-based grants, such as an admin group, to a later CRD change. Three kinds cost about twenty
lines of render code and remove both problems.

**The connectors grant is automatic.** The operator knows the client id, knows that connectors are
enabled, and knows the claim name. The Camunda Helm guide tells a user to write this same entry by
hand. An automatic entry removes boilerplate from every OIDC cluster that runs connectors.

**Alternative that was rejected: the operator provisions the client in the provider.** This would
remove the manual step in Keycloak. It also puts provider credentials in the operator, ties the
operator to one provider API, and crosses the boundary in `CLAUDE.md` that keeps infrastructure
out of this operator.

## Risks

**One client serves both the browser and the machine.** With `clientIdClaim: client_id` and a
hardcoded claim on that client, a user token from the same client also carries `client_id`. The
converter then treats a person as a client. Camunda documents this trap and recommends that the
first claim in the order is absent from tokens of the other principal type. The operator cannot
prevent it, because the shape of the token belongs to the provider. The CRD doc states the
recommendation next to `clientIdClaim`. The e2e never completes a browser login, so the flow is
not affected.

**An admin client does not silence the setup page.** `AdminUserCheckFilter` reads the `users`
member type only. A cluster with an admin client and no admin user still redirects a browser to
`/admin/setup`. This is Camunda behavior, not operator behavior. The CRD doc states it, so that a
user who wants a clean web login lists a user as well.

**Keycloak adds a JVM to the kind node.** The suite already runs Elasticsearch, PostgreSQL, and
two Camunda clusters. The OIDC flow brings its own Elasticsearch and Keycloak. The e2e timeout
covers image pulls of about 2 GB today. The plan measures the flow and splits it into its own job
if the runner budget needs it, which is the fallback that issue #61 names.

## Documentation

The code change and the documentation change land together, as `CLAUDE.md` requires.

| Document | Change |
| --- | --- |
| `docs/crds/camundaplatformconfig.md` | The two claim fields, the token-shape trap, the example |
| `docs/crds/camundacluster.md` | The `spec.auth.admin` block, the connectors grant, the setup-page note |
| `docs/crds/camundaclusterpreset.md` | The merge rule of the `admin` block |

## Deferred

- `groupsClaim` and group-based membership.
- Membership of the roles other than `admin` and `connectors`.
- A `CamundaCluster` condition that reports the resolved admin members.
