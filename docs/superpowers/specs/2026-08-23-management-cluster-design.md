# CamundaManagementCluster: the management plane

**Status:** draft for review
**Date:** 2026-08-23
**Scope:** `CamundaManagementCluster` (types and controller), the `ManagementAuthConfig` producer,
`CamundaPlatformConfig` additions (management clients, provider type, images),
`CamundaCluster` additions (`status.gateway`, ping entries, claim annotation), a Keycloak
wrapper, a Web Modeler user on basic-auth clusters, `pkg/images`

## Summary

`CamundaManagementCluster` is the last planned kind in `core.camunda.io/v1`. Today it is a
Kubebuilder scaffold (`spec.foo`, a no-op reconciler) and `docs/crds/index.md` lists it under
"Planned kinds". This spec replaces the planned design in `docs/crds/camundamanagementcluster.md`,
which predates the facts below.

One namespaced CR deploys the Camunda 8.9 management plane: Management Identity, an identity
provider (Keycloak that the operator runs through the Keycloak Operator, a Keycloak the user
runs, or an external OIDC provider), Console, and Web Modeler. It writes the cluster-scoped
`ManagementAuthConfig` that `CamundaOptimize` already consumes, and it attaches to the
orchestration clusters it serves: it tells each cluster where Console is, and it tells Web
Modeler where each cluster is.

The work ships as one epic with parallel sub-PRs on `feat/management-cluster`. The PR breakdown
and the contracts between PRs live in the plan, not here.

## Verified facts

Every fact below comes from a `camunda-docs` MCP search on 2026-08-23. The URL next to a fact is
the page the search returned. Implementation verifies env var names, container ports, health
paths, and image names against the same pages and against the 8.9 Helm chart before it writes
them; the CRD docs are not a source. Ports in this spec that the pages above do not state
(Console and Web Modeler container ports) are the chart defaults as remembered and are checked
first.

### The management plane in 8.9

- The management plane is Management Identity, Console, and Web Modeler, plus an OIDC identity
  provider. All of them are optional relative to the orchestration cluster.
  https://docs.camunda.io/docs/self-managed/reference-architecture/kubernetes/#components
- Management Identity is required by Console, Web Modeler, and Optimize. None of them can talk
  OIDC without it. An orchestration cluster needs nothing from Management Identity.
  https://docs.camunda.io/docs/self-managed/deployment/helm/configure/enable-additional-components/
  https://docs.camunda.io/docs/self-managed/reference-architecture/#admin-vs-management-identity
- Management Identity is never the OIDC issuer. The issuer, auth, token, and JWKS URLs that
  consumers use belong to Keycloak or to the external provider. Identity's own URL is a separate
  setting (`camunda.identity.base-url`).
  https://docs.camunda.io/docs/self-managed/components/management-identity/miscellaneous/configuration-variables/

### Management Identity

- Identity provider types: `KEYCLOAK` (default), `MICROSOFT`, `GENERIC`
  (`CAMUNDA_IDENTITY_TYPE`).
- Database: PostgreSQL 14 to 17 (also Oracle and SQL Server, out of scope here). Env:
  `IDENTITY_DATABASE_HOST|PORT|NAME|USERNAME|PASSWORD`.
- Keycloak mode: `KEYCLOAK_URL`, `KEYCLOAK_REALM` (default `camunda-platform`),
  `KEYCLOAK_SETUP_USER|PASSWORD|REALM|CLIENT_ID`, `IDENTITY_CLIENT_ID|SECRET`, `IDENTITY_URL`,
  `IDENTITY_AUTH_PROVIDER_ISSUER_URL|BACKEND_URL`. Identity creates the realm, the clients, and
  the initial user on startup. Per-component presets: `KEYCLOAK_INIT_<COMPONENT>_SECRET`,
  `_ROOT_URL`, `_CLIENT_ID` for `OPERATE`, `OPTIMIZE`, `TASKLIST`, `WEBMODELER`. Extra clients:
  the `KEYCLOAK_CLIENTS_<n>_*` family (name, id, secret, root URL, redirect URIs, type,
  permissions), which the Helm value `identity.clients[]` maps to.
  https://docs.camunda.io/docs/self-managed/components/management-identity/miscellaneous/configuration-variables/#component-configuration
  https://docs.camunda.io/docs/self-managed/deployment/helm/configure/authentication-and-authorization/custom-users-and-clients/
  https://docs.camunda.io/docs/self-managed/components/management-identity/miscellaneous/starting-configuration/
- OIDC mode: `SPRING_PROFILES_ACTIVE=oidc`, `CAMUNDA_IDENTITY_TYPE`, `CAMUNDA_IDENTITY_BASE_URL`,
  `CAMUNDA_IDENTITY_ISSUER`, `CAMUNDA_IDENTITY_ISSUER_BACKEND_URL`, `CAMUNDA_IDENTITY_CLIENT_ID`,
  `CAMUNDA_IDENTITY_CLIENT_SECRET`, `CAMUNDA_IDENTITY_AUDIENCE`, `IDENTITY_INITIAL_CLAIM_NAME`
  (default `oid`), `IDENTITY_INITIAL_CLAIM_VALUE`. The initial claim cannot change after first
  start except in the database. Tokens must carry `aud`. Split-horizon URLs are not supported
  for generic OIDC.
  https://docs.camunda.io/docs/self-managed/components/management-identity/configuration/connect-to-an-oidc-provider/
  https://docs.camunda.io/docs/self-managed/deployment/helm/configure/authentication-and-authorization/generic-oidc-provider/
- Identity refuses to start without `CAMUNDA_IDENTITY_AUDIENCE` when Optimize is off.
  https://docs.camunda.io/docs/self-managed/deployment/helm/configure/authentication-and-authorization/microsoft-entra/
- Identity pods must reach the public Keycloak URL (since 8.5.3).
  https://docs.camunda.io/docs/self-managed/components/management-identity/configuration/connect-to-an-existing-keycloak/
- Health: `/actuator/health` on 8082, metrics `/actuator/prometheus`. Chart image
  `camunda/identity:8.9.x`, service port 80.

### Keycloak

- 8.9 supports Keycloak 26.x only.
  https://docs.camunda.io/docs/reference/announcements-release-notes/890/890-announcements/#supported-environments
- Camunda documents the Keycloak Operator: `apiVersion: k8s.keycloak.org/v2alpha1`,
  `kind: Keycloak`, image `camunda/keycloak:quay-optimized-<version>`, `db.url` plus
  `usernameSecret`/`passwordSecret`, HTTP enabled, `additionalOptions`
  `http-relative-path=/auth` and `proxy-headers=xforwarded`, `ingress.enabled: false`,
  `hostname.hostname` with the `/auth` path. Keycloak needs its own PostgreSQL.
  https://docs.camunda.io/docs/self-managed/deployment/helm/configure/operator-based-infrastructure/
- The Bitnami Keycloak subchart is marked deprecated in the 8.9 chart README.

### Console

- No database. Env: `KEYCLOAK_BASE_URL`, `KEYCLOAK_INTERNAL_BASE_URL`, `KEYCLOAK_REALM`
  (Keycloak mode), `CAMUNDA_IDENTITY_CLIENT_ID`, `CAMUNDA_IDENTITY_AUDIENCE`,
  `CAMUNDA_IDENTITY_TYPE`, `CAMUNDA_IDENTITY_BASE_URL`, `CAMUNDA_IDENTITY_ISSUER`,
  `CAMUNDA_IDENTITY_ISSUER_BACKEND_URL`, `SPRING_PROFILES_ACTIVE=oidc` (OIDC mode),
  `CAMUNDA_CONSOLE_CONTEXT_PATH`, `CAMUNDA_LICENSE_KEY`. Console is a public client.
  https://docs.camunda.io/docs/self-managed/components/console/configuration/
- Discovery: `CAMUNDA_CONSOLE_EXPERIMENTAL_DISCOVERY_MODE=true` on Console (experimental in 8.9)
  and `camunda.console.ping.{enabled,endpoint,clusterName,pingPeriod}` on the orchestration
  cluster (env `CAMUNDA_CONSOLE_PING_*`). The ping needs no credentials in 8.9. 8.10 renames it
  to `camunda.hub.ping` and adds credentials.
  https://docs.camunda.io/docs/self-managed/components/console/configuration/#experimental-features
  https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/zeebe/configuration/broker-config/#console-ping-configuration
- Health: `/health/readiness` on 9100. Chart image `camunda/console:8.9.x`.

### Web Modeler

- Two containers in 8.9: `restapi` and `websockets`. `webapp` was removed.
  https://docs.camunda.io/docs/reference/announcements-release-notes/890/whats-new-in-89/#web-modeler
- PostgreSQL through `SPRING_DATASOURCE_URL|USERNAME|PASSWORD`. SMTP is mandatory:
  `RESTAPI_MAIL_HOST|PORT|USER|PASSWORD|ENABLE_TLS|FROM_ADDRESS|FROM_NAME`.
  https://docs.camunda.io/docs/self-managed/components/modeler/web-modeler/configuration/
- OIDC on `restapi`: `OAUTH2_CLIENT_ID`, `CAMUNDA_IDENTITY_BASEURL` (internal Identity URL),
  `CAMUNDA_IDENTITY_TYPE`, `CAMUNDA_MODELER_SECURITY_JWT_AUDIENCE_INTERNAL_API`,
  `CAMUNDA_MODELER_SECURITY_JWT_AUDIENCE_PUBLIC_API`,
  `SPRING_SECURITY_OAUTH2_RESOURCESERVER_JWT_ISSUER_URI`, `..._JWK_SET_URI`,
  `RESTAPI_OAUTH2_TOKEN_ISSUER_BACKEND_URL`, `CAMUNDA_MODELER_OAUTH2_TOKEN_USERNAMECLAIM`.
  Two clients: public UI, confidential API.
- External URL: `RESTAPI_SERVER_URL`, `SERVER_SERVLET_CONTEXTPATH`. WebSocket pairing:
  `RESTAPI_PUSHER_HOST|PORT|APP_ID|KEY|SECRET` on restapi, `PUSHER_APP_ID|KEY|SECRET` on
  websockets, `CLIENT_PUSHER_HOST|PORT|PATH|FORCE_TLS` for the browser.
- Clusters: `CAMUNDA_MODELER_CLUSTERS_<n>_{ID,NAME,VERSION,AUTHENTICATION,URL_GRPC,URL_REST,URL_WEBAPP,AUTHORIZATIONS_ENABLED}`.
  Authentication `BEARER_TOKEN`, `BASIC`, or `NONE`. Without clusters, no deploy.
  https://docs.camunda.io/docs/self-managed/components/modeler/web-modeler/configuration/#clusters
- Without a license, Web Modeler allows five users. Health: `/health/readiness` on 8091
  (restapi), `/up` on 8060 (websockets).

### Optimize

- Optimize still authenticates through Management Identity in 8.9. It reads the contract fields
  `issuerUrl`, `issuerBackendUrl`, `baseUrl`, `clientId`, `audience`, `clientSecretRef`
  (`pkg/components/camundaoptimize/render.go:194-218`). Its redirect URI is
  `<OPTIMIZE_URL>/api/authentication/callback`.

## Goals

- A user runs the whole 8.9 management plane from one CR, with the same conventions as every
  other kind: ocf components, `pkg/components/<crd>` pure builders, `conditions.Aggregate`,
  `pkg/labels`, SSA, status once per reconcile.
- The three identity-provider modes Camunda documents: Keycloak run by the operator, Keycloak
  run by the user, external OIDC.
- The operator produces `ManagementAuthConfig`. Today a user writes it by hand.
- Console and Web Modeler see the orchestration clusters without a user editing a list.
- Every image the operator pulls can be renamed, not only re-registered.

## Non-goals

- Ingress. Every component takes an external URL; the route is the user's.
- Running PostgreSQL. The three databases come from `DatabaseConfig`s; a user brings the server
  today and `DatabaseServer` (#127) brings it later. #127 is not a prerequisite.
- A generated `console.configuration` block. Discovery mode is the mechanism; generation is the
  fallback if e2e proves discovery insufficient, and that is a plan-level change, not a design
  change.
- Oracle and SQL Server for Identity, non-PostgreSQL databases for Web Modeler.
- Managing Keycloak clients through the Keycloak admin API. Identity bootstraps them.
- Digest-pinned or tag-overriding images. Repository overrides only; a follow-up can add more.
- Identity's application, user, group, and role management beyond the initial admin and the
  clients that Identity bootstraps. A user does that in the Identity UI or API.
- Multi-tenancy settings.

## Decisions

1. **One kind, four components.** `CamundaManagementCluster` holds `identityProvider`,
   `identity`, `console`, and `webModeler`, the way `CamundaCluster` holds its processes and the
   optional connectors. A core-plus-attachments split (`CamundaConsole`, `CamundaWebModeler`)
   was rejected: in the Keycloak modes Identity bootstraps the clients of every component at
   startup and needs their root URLs then, so the CR that renders Identity must know every
   component. One CR knows.
2. **Namespaced.** Every `databaseConfigRef` and every Secret resolves in the CR's namespace.
   No `targetNamespace`, no Namespace creation, no Namespace RBAC. This matches the direction of
   #127 (namespace-local chains). A namespaced CR that writes a cluster-scoped contract uses
   owner labels and a finalizer, because a namespaced owner cannot hold a cluster-scoped
   dependent.
3. **Three identity-provider modes, exactly one set.** `keycloak` (operator-managed through the
   Keycloak Operator), `externalKeycloak` (a Keycloak the user runs; Identity still bootstraps
   the realm), `oidc` (external provider).
4. **The platform config owns OIDC.** In `oidc` mode the issuer, the provider type, and every
   client, including the management clients, live on `CamundaPlatformConfig.spec.auth.oidc`.
   The management CR carries no client fields. The management plane is one per platform, so its
   clients are platform facts, and a user who created six app registrations in one sitting
   writes them in one place. The first draft put the clients on the management CR to mirror
   `ClusterAuthSpec`; that mirror is wrong because the platform config already carries a client
   and the cluster only overrides it for the many-clusters case, which does not exist here.
5. **The initial admin stays on the management CR.** It names who administers the management
   plane, the `ClusterAuthSpec.admin` analog, and it is immutable after first start.
6. **Cluster discovery by label selector.** `spec.clusterSelector` over `CamundaCluster` in all
   namespaces, with the Kubernetes LabelSelector convention: an unset selector selects no
   cluster, `{}` selects every cluster. The management controller claims each selected cluster
   with an annotation, pushes the Console ping entries through SSA under its own field manager,
   and reads each cluster's endpoints for Web Modeler. `CamundaCluster` does not know the
   management cluster exists. Because the kind reaches clusters in other namespaces, creating a
   `CamundaManagementCluster` is a platform-administrator action; the CRD doc and the kind GoDoc
   say so, and the generated editor ClusterRole is not for tenants. (Revised 2026-08-23 after a
   review comment on #193; the first draft said "empty selects all".)
7. **A dedicated Web Modeler user on basic-auth clusters.** Web Modeler deploys to an OIDC
   cluster with `BEARER_TOKEN`. For a basic-auth cluster the management controller creates a
   `web-modeler` user on the cluster through the cluster's admin credential, with a generated
   password in the management namespace, and a role that holds the permissions Web Modeler
   needs. The admin credential never reaches Web Modeler.
8. **`spec.images` on the platform config.** One optional repository per image the operator
   pulls; the tag always comes from the `version` field of the owning CR. One resolver,
   `pkg/images`, replaces the two `Image()` functions that exist today.
9. **Keycloak only through the Keycloak Operator.** No Deployment of Keycloak by this operator.
   The wrapper follows the ECK shape, including the startup probe for the CRD.
10. **Each app has its own version.** Identity, Console, Web Modeler, and Keycloak each carry a
    full semantic version; floors `8.9.0` and `26.0.0`.

## Design

### API

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaManagementCluster
metadata:
  name: my-management
  namespace: my-management-ns
spec:
  platformConfigRef: my-platform        # required
  suspend: false
  clusterSelector:                      # CamundaClusters served by Console and Web Modeler
    matchLabels: {camunda.io/platform: prod}   # unset = no cluster, {} = every cluster
  managementAuthConfigName: my-management      # cluster-scoped output; default = metadata.name
  identityProvider:                     # exactly one of keycloak, externalKeycloak, oidc
    keycloak:
      version: 26.4.0                   # camunda/keycloak:quay-optimized-<version>
      externalUrl: https://auth.example.com/auth
      databaseConfigRef: keycloak-db
      replicas: 1
      resources: {}
    # externalKeycloak:
    #   url: https://kc.example.com/auth
    #   realm: camunda-platform
    #   adminCredentialsSecretRef: {name: kc-admin, usernameKey: username, passwordKey: password}
    # oidc: {}
  identity:
    version: 8.9.8
    externalUrl: https://identity.example.com
    databaseConfigRef: identity-db
    admin:
      claimName: oid                    # oidc mode
      claimValue: "11111111-2222-3333-4444-555555555555"
      # username: admin                 # keycloak modes
      # email: admin@example.com        # keycloak modes
      # passwordSecretRef: {name: identity-admin, key: password}
  optimize:                             # required in the keycloak modes
    externalUrl: https://optimize.example.com
    # WorkloadSpec: replicas, resources, extraEnv, extraEnvFrom, podLabels, podAnnotations, scheduling
  console:
    enabled: true
    version: 8.9.88
    externalUrl: https://console.example.com
    # WorkloadSpec
  webModeler:
    enabled: true
    version: 8.9.12
    externalUrl: https://modeler.example.com
    websocketsExternalUrl: https://modeler.example.com/modeler-ws
    databaseConfigRef: modeler-db
    mail:
      smtpHost: smtp.example.com
      smtpPort: 587
      fromAddress: noreply@example.com
      fromName: Camunda
      tls: true
      credentialsSecretRef: {name: smtp, usernameKey: username, passwordKey: password}
    restapi: {}                         # WorkloadSpec
    websockets: {}                      # WorkloadSpec
status:
  observedGeneration: 3
  managementAuthConfig: my-management
  clusters:
    - name: my-cluster
      namespace: my-cluster-ns
      attached: true
    - name: other
      namespace: other-ns
      attached: false
      reason: ClaimedElsewhere
      message: "claimed by CamundaManagementCluster staging-ns/staging"
  conditions:
    - Ready, KeycloakReady, IdentityReady, ConsoleReady, WebModelerReady,
      ManagementAuthReady, SecretsReady
```

Validation (CEL and controller):

- Exactly one of `identityProvider.keycloak`, `externalKeycloak`, `oidc` (CEL).
- `identity.externalUrl`, `console.externalUrl`, `webModeler.externalUrl`,
  `webModeler.websocketsExternalUrl` are `http(s)://` URLs; required when the component is
  enabled (CEL).
- `webModeler.mail.smtpHost` and `fromAddress` required when Web Modeler is enabled (CEL).
- `identity.admin` carries `claimName`+`claimValue` in `oidc` mode and `username` (+ optional
  `passwordSecretRef`, generated when absent) in the Keycloak modes (CEL on the pair, controller
  on the mode).
- Versions match `^\d+\.\d+\.\d+$` (CEL); floors in the controller (`UnsupportedVersion`).
- `keycloak.databaseConfigRef`, `identity.databaseConfigRef`, `webModeler.databaseConfigRef`
  name three distinct `DatabaseConfig`s (controller, `InvalidReference`).
- In `oidc` mode the platform config has `method: oidc` and carries the management clients the
  enabled components need (controller, `InvalidReference` naming the missing one).

Labels: owner key `camunda.io/management-cluster` (new in `pkg/labels`), component values
`keycloak`, `management-identity`, `console`, `web-modeler-restapi`, `web-modeler-websockets`.
Keycloak pods, run by the Keycloak Operator from our CR, carry owner and component through the
CR's pod template but not `managed-by`, the ECK rule.

### Identity provider modes

**`keycloak`.** `pkg/wrappers/keycloak` wraps `k8s.keycloak.org/v2alpha1 Keycloak` as an ocf
generic workload resource: plain Go types in the wrapper (no upstream Go module; the api
module does not see them), `builder.go`, `mutator.go`, `resource.go`, `health.go` on the CR's
`Ready` condition, `applyclient.go` that strips fields the CRD does not declare, a suspend
handler that sets `instances: 0`. `SetupWithManager` probes the RESTMapper once and registers
`Owns(&Keycloak{})` only when the kind is served; otherwise the pre-check reports
`Ready=False/KeycloakOperatorNotInstalled`, the `ECKNotInstalled` shape. The CR:
`instances`, `image` (through `pkg/images`), `db` from the `DatabaseConfig` (the whole
`jdbc:aws-wrapper:postgresql://` URL, `schema: public`, `usernameSecret` and `passwordSecret`
pointing at the credentials Secret keys), HTTP on, `additionalOptions` `http-relative-path=/auth`
and `proxy-headers=xforwarded`, `hostname.hostname` = `externalUrl`, Ingress off, pod template
labels. The Keycloak Operator writes `<name>-initial-admin`; Identity's
`KEYCLOAK_SETUP_USER|PASSWORD` read it, and the Identity component waits for `KeycloakReady`
through an ocf prerequisite. `KEYCLOAK_URL` is the in-cluster Service URL the operator creates
(`<name>-service`), the issuer for consumers is `<externalUrl>/realms/camunda-platform`, and the
backend issuer is the in-cluster URL. The authorization endpoint follows the front-channel
issuer; the token and JWKS endpoints are server-to-server, so they follow the backend issuer.
`externalUrl` must carry the `/auth` path, because the rendered Keycloak serves there.

**`externalKeycloak`.** Same Identity rendering. `KEYCLOAK_URL` = `spec.url`, setup credentials
from `adminCredentialsSecretRef`, realm from `spec.realm`. Issuer and backend issuer are both
`<url>/realms/<realm>`. The Keycloak component is built in every mode and gated on the `keycloak`
mode, so a move to `externalKeycloak` or `oidc` deletes the custom resource the operator wrote. A
gated-off component reports `KeycloakReady=True/Disabled` and stays out of `Ready`. A Kubernetes
cluster that serves no Keycloak kind gets no component at all.

**Both Keycloak modes.** Identity bootstraps the realm and the clients. The operator generates
(`pkg/credentials`, rotatable by deleting the Secret) `IDENTITY_CLIENT_SECRET`,
`KEYCLOAK_INIT_OPTIMIZE_SECRET`, and the Web Modeler pusher credentials. Every client comes from
a component preset, Console included: the 8.9 chart carries a `console` preset
(`charts/camunda-platform-8.9/templates/identity/configmap.yaml`, `keycloak.init.console.root-url`
and the `component-presets.console` block), so the operator sets
`KEYCLOAK_INIT_CONSOLE_ROOT_URL` = `console.externalUrl`,
`KEYCLOAK_INIT_OPTIMIZE_ROOT_URL` = `optimize.externalUrl`, and
`KEYCLOAK_INIT_WEBMODELER_ROOT_URL` = `webModeler.externalUrl`. A preset also creates the
resource server that the audience of the component names, which a hand-written
`KEYCLOAK_CLIENTS_<n>_*` entry would not. `KEYCLOAK_SETUP_REALM` (`master`) and
`KEYCLOAK_SETUP_CLIENT_ID` (`admin-cli`) are rendered at their documented defaults, so the
configuration says where the Keycloak administrator comes from. The initial admin is
`identity.admin.username` with the password from `passwordSecretRef` or a generated one, and an
optional `identity.admin.email`.

**`oidc`.** `SPRING_PROFILES_ACTIVE=oidc`, `CAMUNDA_IDENTITY_TYPE` from
`platform.auth.oidc.providerType`, `CAMUNDA_IDENTITY_ISSUER` and `_ISSUER_BACKEND_URL` from the
platform issuer (backend = issuer; split horizon is unsupported for generic OIDC),
`CAMUNDA_IDENTITY_CLIENT_ID|SECRET|AUDIENCE` from `platform.auth.oidc.management.clients.identity`,
`IDENTITY_INITIAL_CLAIM_NAME|VALUE` from `identity.admin`. A change to the claim after the first
successful start is reported on `IdentityReady` with the reason `ImmutableAfterStart` and the
rendered env keeps the first value; the docs say the database is the place to change it.

**Identity in every mode.** `IDENTITY_URL` = `identity.externalUrl`, database env from its
`DatabaseConfig`, `CAMUNDA_LICENSE_KEY` from the platform, `CAMUNDA_IDENTITY_AUDIENCE` always
set. One Deployment, one Service (80 -> 8080, 82 -> 8082), readiness `/actuator/health` on
8082 (the chart's probe; `/actuator/health/readiness` needs a Spring setting the chart does not
set — recorded at the wave-2 checkpoint). Referenced Secrets that live in another namespace
(platform license and client secrets) are copied into the management namespace; the copies
report `MirroredSecretsReady`, included in `Ready` only when a copy exists.

### The `ManagementAuthConfig` output

The controller applies the contract through SSA under its own field manager once the inputs
resolve, and keeps it converged:

| Field | Keycloak modes | `oidc` mode |
| --- | --- | --- |
| `baseUrl` | `identity.externalUrl` | `identity.externalUrl` |
| `issuerUrl` | `<keycloak external>/realms/<realm>` | platform `issuerUrl` |
| `issuerBackendUrl` | in-cluster realm URL (`keycloak`), same as issuer (`externalKeycloak`) | platform `issuerUrl` |
| `authUrl`, `tokenUrl`, `jwksUrl` | the realm's `protocol/openid-connect/{auth,token,certs}` | platform values or discovery defaults |
| `clientId`, `audience` | `optimize`, `optimize-api` | platform `management.clients.optimize` |
| `clientSecretRef` | the generated Secret, explicit namespace | platform `clients.optimize.clientSecretRef` |

The contract carries `camunda.io/management-cluster` and `camunda.io/management-cluster-namespace`
labels. A contract that exists with different owner labels, or with no labels and a spec the
controller did not write, makes the management cluster `Ready=False/Conflict` and leaves the
contract alone. The finalizer deletes the contract. The existing validation controller of
`ManagementAuthConfig` keeps running on it unchanged. `ManagementAuthReady` reports the write.

### Console

One Deployment and Service (`camunda/console:<version>`, port 80 -> 8080, readiness
`/health/readiness:9100`). Env: `CAMUNDA_IDENTITY_TYPE`, `CAMUNDA_IDENTITY_BASE_URL` (internal
Identity Service URL), `CAMUNDA_IDENTITY_ISSUER`, `CAMUNDA_IDENTITY_ISSUER_BACKEND_URL`,
`CAMUNDA_IDENTITY_CLIENT_ID`, `CAMUNDA_IDENTITY_AUDIENCE` (`console` in Keycloak modes,
`management.clients.console` in oidc mode), `KEYCLOAK_BASE_URL`, `KEYCLOAK_INTERNAL_BASE_URL`,
`KEYCLOAK_REALM` (Keycloak modes), `SPRING_PROFILES_ACTIVE=oidc` (oidc mode),
`CAMUNDA_CONSOLE_CONTEXT_PATH` from `externalUrl`'s path, `CAMUNDA_LICENSE_KEY`,
`CAMUNDA_CONSOLE_EXPERIMENTAL_DISCOVERY_MODE=true`. The docs name the flag as experimental.

### The cluster attachment

The controller watches `CamundaCluster` in every namespace and selects by `clusterSelector`.

**Claim.** Each selected cluster gets the annotation
`camunda.io/management-cluster: <namespace>/<name>`, applied under the controller's field
manager. A cluster already annotated by another management cluster is not touched; it appears
in `status.clusters` with `attached: false`, `reason: ClaimedElsewhere`. The claim is withdrawn
on deselect and on deletion.

**Ping.** For each claimed cluster the controller applies top-level `spec.extraEnv` entries
under a ping field manager of its own (distinct from the claim's, because two applies under one
manager strip each other's fields) with the cluster UID as precondition, the `patchExporter`
shape:
`CAMUNDA_CONSOLE_PING_ENABLED=true`, `_ENDPOINT=<console.externalUrl>`,
`_CLUSTERNAME=<cluster name>`, `_PINGPERIOD=1h`. The key set is selected from the cluster's
`status.management.version`: `CAMUNDA_CONSOLE_PING_*` for 8.9, `CAMUNDA_HUB_PING_*` for 8.10 and
later. Only when Console is enabled; withdrawn when Console is disabled, the cluster is
deselected, or the management cluster is deleted.

**Endpoints.** `CamundaCluster.status.gateway {grpcEndpoint, restEndpoint}` is added: the
in-cluster URLs of the gateway, unset while suspended, the `status.management` rule. Web Modeler
reads them, and `spec.externalUrl` for the browser link.

### Web Modeler

Two Deployments and Services: `web-modeler-restapi` (80 -> 8081, readiness
`/health/readiness:8091`) and `web-modeler-websockets` (80 -> 8060, readiness `/up:8060`).

`restapi` env: `SPRING_DATASOURCE_URL|USERNAME|PASSWORD` from its `DatabaseConfig`,
`RESTAPI_MAIL_*` from `spec.webModeler.mail`, `RESTAPI_SERVER_URL`,
`SERVER_SERVLET_CONTEXTPATH` from `externalUrl`, `OAUTH2_CLIENT_ID` (`web-modeler` or
`management.clients.webModeler`), `CAMUNDA_IDENTITY_BASEURL` (internal), `CAMUNDA_IDENTITY_TYPE`,
`CAMUNDA_MODELER_SECURITY_JWT_AUDIENCE_INTERNAL_API`, `..._PUBLIC_API`,
`SPRING_SECURITY_OAUTH2_RESOURCESERVER_JWT_ISSUER_URI`, `..._JWK_SET_URI`,
`RESTAPI_OAUTH2_TOKEN_ISSUER_BACKEND_URL`, `CAMUNDA_MODELER_OAUTH2_TOKEN_USERNAMECLAIM` from the
platform's `usernameClaim`, `RESTAPI_PUSHER_*` with generated app id, key, secret,
`CLIENT_PUSHER_HOST|PORT|PATH|FORCE_TLS` from `websocketsExternalUrl`, `CAMUNDA_LICENSE_KEY`, and
the cluster list. `websockets` env: `PUSHER_APP_ID|KEY|SECRET`.

Cluster list: one `CAMUNDA_MODELER_CLUSTERS_<n>_*` block per attached cluster: `ID` = the
cluster UID, `NAME` = `<namespace>/<name>`, `VERSION` = `status.management.version`, `URL_GRPC`
and `URL_REST` from `status.gateway`, `URL_WEBAPP` = `spec.externalUrl`,
`AUTHORIZATIONS_ENABLED=true`, `AUTHENTICATION=BEARER_TOKEN` for an OIDC cluster and `BASIC`
with the dedicated user for a basic-auth cluster. A cluster without `status.gateway` (suspended,
not ready) is listed in `status.clusters` with `reason: NotReady` and left out of the env.

**The Web Modeler user.** For a basic-auth attached cluster the controller creates the user
`web-modeler` through `pkg/camundaadmin` with the cluster's admin credential
(`status.adminPassword`), stores the generated password in the Secret
`<name>-web-modeler-cluster-<cluster uid prefix>` in the management namespace, and assigns a
role. `pkg/camundaadmin` gains assign-role and create-authorization calls
(`/v2/roles/{id}/users/{username}`, `/v2/authorizations`). The plan verifies against the Camunda
docs which authorizations Web Modeler needs to deploy resources and start instances; if the docs
do not pin a set, the user gets the `admin` role and the CRD doc says so. The password rotates
when the Secret is deleted, the admin-password shape. On deletion of the management cluster the
user is removed, best effort, with an event when it fails.

### `CamundaPlatformConfig` additions

```yaml
spec:
  auth:
    method: oidc
    oidc:
      issuerUrl: https://login.example.com/tenant/v2.0
      providerType: microsoft           # generic (default) | microsoft; read by Identity only
      clientId: orchestration           # unchanged: the orchestration default client
      management:
        clients:
          identity:     {clientId: camunda-identity, audience: camunda-identity-resource-server, clientSecretRef: {...}}
          optimize:     {clientId: optimize, audience: optimize-api, clientSecretRef: {...}}
          webModeler:   {clientId: web-modeler}
          webModelerApi: {clientId: web-modeler-api, audience: web-modeler-api, publicApiAudience: web-modeler-public-api, clientSecretRef: {...}}
          console:      {clientId: console, audience: console}
  imageRegistry: registry.example.com
  images:
    camunda: registry.example.com/mirror/camunda
    connectors: ...
    optimize: ...
    identity: ...
    console: ...
    webModelerRestapi: ...
    webModelerWebsockets: ...
    keycloak: ...
```

`management` is optional and only read in `oidc` mode. The platform validation controller
checks every new `clientSecretRef` (`MissingSecret`). `images.<name>` is a repository; the tag
comes from the owning CR's `version`. `pkg/images.Resolve(platform, image, version)` applies
the override, else `imageRegistry` in front of the default repository, else the default.
`pkg/components/camundacluster.Image` and `pkg/components/camundaoptimize.Image` are replaced by
it, and so is every Job image the backup and restore controllers render.

### Status, suspend, versions, deletion

- `Ready` is `conditions.Aggregate` over the built components, or a pre-check failure:
  `InvalidReference`, `MissingSecret`, `KeycloakOperatorNotInstalled`, `Conflict`,
  `UnsupportedVersion`. Per-component conditions: `KeycloakReady` (only in `keycloak` mode),
  `IdentityReady`, `ConsoleReady`, `WebModelerReady`, `ManagementAuthReady`, `SecretsReady`
  (only when the controller generated something). New reasons live in
  `api/v1/camundamanagementcluster_types.go`; shared ones in `api/v1/conditions.go`.
- `status.clusters[]` lists every selected cluster with `attached`, `reason`, `message`.
- `spec.suspend` scales Identity, Console, and Web Modeler to zero and sets the Keycloak CR to
  `instances: 0` through the wrapper's suspend handler. The contract, the claim, and the ping
  entries stay. `Ready=True/Suspended`.
- Versions: `identity.version`, `console.version`, `webModeler.version`,
  `identityProvider.keycloak.version`, each a full semantic version with its own patch line.
  Floors `8.9.0` and `26.0.0` in `pkg/components/camundamanagementcluster.ValidateSpec`.
- Deletion: the finalizer deletes the contract, withdraws the ping entries and the claims, and
  removes the Web Modeler users it created. Deployments, Services, Secrets, and the Keycloak CR
  go by owner reference. Databases are never dropped.

### Security

- Generated secrets (Identity client secret, Optimize client secret, pusher credentials,
  Keycloak initial admin from the Keycloak Operator, Web Modeler cluster users, Identity admin
  password in Keycloak modes) live in the management namespace, owned by the CR, and rotate by
  deletion.
- The only credential that crosses a namespace is the Optimize client secret reference in the
  cluster-scoped contract, which `CamundaOptimize` already mirrors into its own namespace.
- The cluster admin credential is read by the management controller to create the Web Modeler
  user and is never rendered into any pod.
- RBAC: the controller needs `camundaclusters` get/list/watch/patch, `keycloaks` full,
  `managementauthconfigs` full, `databaseconfigs` get/list/watch, `secrets` in the usual shape.

## Testing

- Pure unit tests (testify) in `pkg/components/camundamanagementcluster`: env per mode, contract
  derivation, ping entries by cluster minor, the cluster list, version floors, selector and claim
  rules, the immutable-claim rule.
- Golden snapshots (ocf `pkg/testing/golden`): fixtures `managed-keycloak`, `external-keycloak`,
  `oidc`, each minimal and realistic. Builder tests for `pkg/wrappers/keycloak`. `pkg/images`
  tests and refreshed goldens for `CamundaCluster` and `CamundaOptimize`. `pkg/camundaadmin`
  role and authorization calls against `camundaadmintest`.
- envtest (Ginkgo) in `internal/controller/camundamanagementcluster`: pre-check reasons, the
  RESTMapper probe for the Keycloak CRD, contract write and delete, claim and ping apply and
  withdraw on select, deselect, and delete, `Conflict` on a taken contract name and on a cluster
  claimed elsewhere, suspend, finalizer. The Keycloak CRD is vendored into `internal/testenv` as
  the ECK CRDs are.
- e2e (kind): the suite installs the Keycloak Operator when absent
  (`KEYCLOAK_OPERATOR_INSTALL_SKIP`, `KEYCLOAK_OPERATOR_VERSION`), the ECK hooks. Flow A:
  platform config + three `Database`s on the suite's PostgreSQL, a management cluster in
  `keycloak` mode with Console and Web Modeler, `Ready`, the contract exists, a `CamundaCluster`
  on OIDC against the realm is attached (ping env, cluster env), a `CamundaOptimize` attaches
  through the contract. Flow B: `oidc` mode against the suite's existing Keycloak with a
  basic-auth cluster, the `web-modeler` user exists on it. Management images join
  `test/e2e/matrix/<minor>.env`.
- Docs: `mkdocs build --strict` and the fresh-reader pass of `writing-operator-docs`.

## Docs

- `docs/crds/camundamanagementcluster.md` rewritten from `TEMPLATE.md`: Identity provider,
  Management Identity, Console, Web Modeler, Clusters, Suspension, Deletion, Status, Spec
  reference. The "Not implemented yet" banner and the "How it works" section go.
- `docs/crds/managementauthconfig.md`: the management cluster as producer.
- `docs/crds/camundaplatformconfig.md`: provider type, management clients, images.
- `docs/crds/camundacluster.md`: `status.gateway`, the ping entries a user sees in
  `spec.extraEnv`, the claim annotation.
- `docs/crds/camundaoptimize.md`: the redirect URI in Keycloak mode.
- `docs/crds/index.md`: the kind moves out of "Planned kinds"; the section disappears once
  #178 merges.
- `docs/architecture.md`: contract table and diagram.
- New `docs/guides/management-plane.md`: databases, platform config, management cluster,
  clusters, Optimize, in that order, with CR fragments and status blocks.
- `docs/installation.md`: the Keycloak Operator as an optional prerequisite next to ECK.

## Risks and open items

- **Optimize redirect URIs in Keycloak modes.** *Answered in implementation.* Identity
  re-applies the whole client representation on every start
  (https://github.com/camunda/camunda/issues/59963), so a changed root URL reaches Keycloak on
  the next roll of Management Identity. One preset carries one root URL, and `CamundaOptimize`
  carries no `externalUrl` of its own, so `spec.optimize.externalUrl` is required in both
  Keycloak modes and names the single Optimize this management plane bootstraps. A second
  Optimize needs its callback URL added to the `optimize` client by hand. A follow-up,
  "one Keycloak client per CamundaOptimize", holds the multi-Optimize case.
- **Console discovery is experimental in 8.9.** If e2e shows a cluster does not appear, the
  fallback is a generated `console.configuration` block from the same attached-cluster data.
- **8.10 renames.** Console becomes Hub, `camunda.console.ping` becomes `camunda.hub.ping`, the
  Web Modeler restapi image becomes `camunda/hub`. The ping key set and the image defaults
  select by version; the CRD field names stay.
- **Identity bootstrap against an existing realm.** A forum thread reports Identity always
  attempts the realm bootstrap and can fail with a conflict against a realm created by hand. The
  `externalKeycloak` doc states the realm requirements Camunda documents (realm name equals
  realm id, the `camunda-identity` client with its roles) and links them.
- **Web Modeler permissions.** Which authorizations the dedicated user needs is verified in the
  plan; `admin` is the documented fallback.
- **Keycloak Operator Go types.** Hand-written plain Go types must track the v2alpha1 schema.
  The wrapper strips undeclared fields on apply, so a schema drift fails loudly at apply time.

## Alternatives considered

- **Core + attachments** (`CamundaConsole`, `CamundaWebModeler` as kinds): cleaner boundaries,
  three more CRDs, and a Keycloak-mode bootstrap that needs every component URL before the
  components exist. Rejected (Decision 1).
- **Cluster-scoped kind with `targetNamespace`** (the planned design): needs Namespace creation
  and RBAC the operator does not have, and conflicts with #127's direction. Rejected
  (Decision 2).
- **Clients on the management CR** (`ClusterAuthSpec` mirror): splits one IdP across two
  objects. Rejected (Decision 4).
- **Self-contained OIDC block on the management CR**: duplicates the platform issuer. Rejected.
- **Web Modeler deploys as the cluster admin**: no admin credential leaves the cluster.
  Rejected (Decision 7).
- **Explicit cluster list** instead of a selector: every new cluster edits the management CR.
  Rejected (Decision 6).
- **Generated `console.configuration`** instead of discovery: kept as fallback.

## Doc deviations

`docs/crds/camundamanagementcluster.md` is rewritten; the deviations from it are the decisions
above: namespaced, three provider modes, the platform config owns OIDC, discovery by selector,
a Web Modeler user on basic clusters, Identity is not the issuer, Web Modeler has two
containers, Console has no database, no Namespace creation, ocf status vocabulary instead of
`Progressing`, `## Spec reference` instead of `## API reference`, topic sections instead of
`## How it works`.

## Implementation breakdown

One epic, `feat/management-cluster`. A contract PR first (api types, platform config fields,
`pkg/images`, `CamundaCluster.status.gateway`, condition vocabulary, the Keycloak wrapper
types), then four component PRs in parallel (Identity with `oidc` mode and the contract output;
the Keycloak wrapper with both Keycloak modes; Console with the ping attachment; Web Modeler
with the cluster list and the basic-cluster user), then e2e and docs. The plan holds the
contracts and conventions.
