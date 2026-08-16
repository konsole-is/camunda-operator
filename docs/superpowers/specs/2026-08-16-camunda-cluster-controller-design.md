# CamundaCluster controller (Batch C)

**Status:** draft
**Date:** 2026-08-16
**Scope:** CamundaCluster, CamundaClusterPreset (types only), consumption of CamundaPlatformConfig

## Summary

Implement the `CamundaCluster` controller: the third controller batch from the
[implementation order](../../crds/index.md#implementation-order). The controller turns one
`CamundaCluster` into the workloads of a Camunda 8.9 orchestration cluster (Zeebe brokers,
gateway, web applications, connectors), configured through the unified configuration of the
`camunda/camunda` binary, with secondary storage from a `SecondaryStorageConfig`, shared
settings from a `CamundaPlatformConfig`, and a baseline from an optional
`CamundaClusterPreset`. `CamundaClusterPreset` ships as a passive data CRD (types and schema
validation, no controller), like `ElasticsearchClusterPreset` in Batch B.

The batch is built the Batch B way: ocf components, `pkg/components/<crd>` pure builders,
`conditions.Aggregate` for `Ready`, `pkg/labels`, envtest through `internal/testenv`, and the
kind e2e suite.

The CRD docs (`docs/crds/camundacluster.md`, `camundaclusterpreset.md`,
`camundaplatformconfig.md`) are the starting point, not the contract. Where this spec or the
implementation finds a better shape, the doc changes in the same PR (see "Doc deviations").

## The source-of-truth rule for configuration

Every configuration property, environment variable name, image name, port, path, and health
endpoint that the operator renders is verified against the Camunda source before it is written:

- **`~/Documents/camunda/camunda` at the 8.9 line** (tag `8.9.9` at the time of writing;
  worktree at that tag while implementing). Primary files:
  `dist/src/main/config/defaults.yaml` (generated from the `camunda.*` configuration classes;
  it does **not** list `camunda.security.*`, `camunda.license.*`, `camunda.webapps.enabled`,
  `camunda.rest.enabled`), the classes under `configuration/src/main/java/io/camunda/configuration/`,
  `dist/src/main/java/io/camunda/application/ModesAndProfilesProcessor.java`,
  `dist/src/main/java/io/camunda/application/initializers/*.java`,
  `security/security-core/src/main/java/io/camunda/security/configuration/*.java`,
  `dist/src/main/resources/application.properties`, and `camunda.Dockerfile`.
- **`~/Documents/camunda/camunda-operator`** (the upstream operator) is reference only. Its
  gateway is unified and its broker is legacy; some of its keys are wrong for 8.9
  (`camunda.api.grpc.host` does not exist, the property is `address`; `camunda.identity.*`
  has no binding in the orchestration cluster). Nothing is copied from it without the source
  check above.
- **The CRD docs are not a source.** The `camundacluster.md` env examples are corrected to the
  verified names.

The rule is enforced in code: `pkg/camundaconfig` declares every key the operator sets, each
with a comment that names the source file (and line at the time of writing). The renderer
emits only declared keys, and a unit test asserts that. A key without a source pointer does
not exist.

The keys verified for this spec are listed under "Verified configuration". Anything marked
*verify in implementation* is a known open point with the two possible answers, and the
implementer settles it against the source before the golden that uses it is committed.

## Goals

- Real `api/v1` types for `CamundaCluster` and `CamundaClusterPreset`, faithful to the docs
  except where "Doc deviations" says otherwise, with schema and CEL validation.
- A `CamundaCluster` controller that renders and converges the full documented topology
  surface: Zeebe StatefulSet, gateway `Standalone | Embedded`, operate/tasklist/identity
  `Standalone | Embedded`, connectors, Services, optional ServiceMonitors.
- Configuration through unified-configuration environment variables on the single
  `camunda/camunda` image (Spring relaxed binding), layered from cluster identity, secondary
  storage (Elasticsearch and RDBMS), auth from the platform config (basic and OIDC), preset,
  and per-component overrides.
- Watches on every referenced CR and Secret, so a change to a platform config, preset,
  binding, or Secret rolls the cluster without touching the `CamundaCluster`.
- Field-level preset merge exactly as the preset doc specifies.
- `Ready` and per-process conditions with the Batch B condition model; suspend (scale to zero,
  data kept) and pause (no reconcile).
- Zeebe storage growth in place; retention policy for broker volumes on deletion.
- Tests at three levels: pure unit tests and goldens, envtest, and kind e2e against the Batch B
  Elasticsearch and PostgreSQL backends. e2e proves basic auth; OIDC is proven by goldens.

## Non-goals

- No backup wiring in this batch: no Zeebe primary-storage backup store, no snapshot
  repository registration, no continuous RDBMS backup (doc steps 8 and 9). `backupStorageRef`
  and `documentStorageRef` are accepted and existence-checked (`InvalidReference`) only. Batch
  D lands the wiring together with the `Backup` kinds that exercise it.
- No OIDC end-to-end proof (no identity provider in the kind suite).
- No Ingress, HTTPRoute, or certificate management (the doc already says so). `externalUrl` is
  used only where the binary needs an absolute URL.
- No Optimize, management cluster, PVC auto-resize, or `CamundaPlatformConfig` controller
  changes beyond what consumption needs.
- No multi-tenancy, authorizations tuning, or MCP gateway configuration; the operator sets
  the keys the topology and storage need and leaves the rest to `extraEnv`.
- No OpenSearch (`SecondaryStorageConfig` has no such type).

## Design

### Architecture and layout

The controller follows the Batch B layout:

- `internal/controller/camundacluster/controller.go` — Reconcile: fetch; `spec.pause` early
  return; pre-checks; component build and reconcile; `conditions.Aggregate`; one status write
  through `FlushStatus`. Watches: owned StatefulSets, Deployments, Services, ServiceMonitors,
  Secrets (metadata only), PersistentVolumeClaims of the brokers, and `refindex`-driven watches
  on `CamundaPlatformConfig`, `CamundaClusterPreset`, `SecondaryStorageConfig`,
  `DatabaseConfig`, `DatabaseServerConfig`, `ObjectStorageConfig`, and every referenced Secret.
  `SetupWithManager` nil-guards `Recorder` and the component client; the recorder is named
  `camundacluster`.
- `pkg/components/camundacluster/` — pure: `MergePreset`, `ValidateMerged`, the topology
  resolver, the configuration renderer, one component builder per process, goldens.
- `pkg/camundaconfig/` — the declared vocabulary of unified-configuration keys with source
  pointers, the Spring relaxed-binding conversion (`camunda.data.secondary-storage.type` →
  `CAMUNDA_DATA_SECONDARYSTORAGE_TYPE`; a dash is dropped, a dot becomes an underscore, list
  indexes `[0]` become `_0_`), and typed helpers for the few list-valued keys.
- Workloads use the ocf StatefulSet, Deployment, and Service primitives; ServiceMonitors the
  existing `pkg/wrappers/servicemonitor` behind `IncludeWhen` (CRD served) and `GatedBy`
  (`monitoring.serviceMonitor.enabled`). Labels come from `pkg/labels`: `camunda.io/cluster`
  (the owner key reserved for this kind), `camunda.io/component`, `app.kubernetes.io/managed-by`.
  Component values: `zeebe`, `gateway`, `operate`, `tasklist`, `identity`, `connectors`.
- Managed resources are applied with SSA under ocf's field manager `CamundaCluster/<process>`.

### Topology model

`Resolve(merged) → []Process`. A process is one workload: name, kind (StatefulSet for
`zeebe`, Deployment otherwise), replicas, and the applications it hosts. The unified binary
selects its role through `camunda.mode` (`all-in-one | broker | gateway`,
`ModesAndProfilesProcessor.java:27`) and the web applications it serves through
`camunda.webapps.<app>.enabled` (`Webapps.java`, `defaults.yaml:1415-1435`).
`camunda.webapps.enabled` is derived by the binary and is never set by the operator
(`WebappsConfigurationInitializer.java:54-55`, `ModesAndProfilesProcessor.java:74`).

| Spec shape | Processes and roles |
| --- | --- |
| gateway `Embedded`, all web apps `Embedded` | `zeebe`: `camunda.mode=all-in-one`, all `webapps.<app>.enabled=true` |
| gateway `Standalone`, all web apps `Embedded` (8.9 default) | `zeebe`: `camunda.mode=broker`; `gateway`: `camunda.mode=gateway`, all web apps enabled |
| a web app `Standalone` | plus a process named after the app: `camunda.mode=gateway`, only that app enabled; the host process (gateway, or zeebe when the gateway is embedded) sets that app's `webapps.<app>.enabled=false` |
| `connectors.enabled` | plus `connectors`: the connectors runtime pointed at the cluster's gRPC and REST Services |

A standalone web application is therefore a gateway process that serves one application
(`ModesAndProfilesProcessor.java:142-164`, `WebappsConfigurationInitializer.java:74-78`); it
runs the gateway code path but is not a member of the gateway Service and gets no gRPC Service
of its own. `camunda.mode=broker` sets `zeebe.broker.gateway.enable=false` and `all-in-one`
sets it `true` inside the binary (`ModesAndProfilesProcessor.java:73,81`); the operator does
not set that legacy key.

*Identity/admin profile.* `camunda.mode` derives the profile `identity` for the Identity
application, and the binary treats `identity` and `admin` alike for serving the UI
(`WebappsConfigurationInitializer.java:112-118`). The operator relies on `camunda.mode` and
does not set `spring.profiles.active`. **Verify in implementation** (e2e: the Identity UI
answers on the gateway); if it does not, the fallback is `SPRING_PROFILES_ACTIVE` per process
with the profile list the resolver computes, and the spec is amended.

*Broker node id.* Each broker needs `camunda.cluster.node-id` equal to its ordinal
(`defaults.yaml:186`). **Verify in implementation**: either `camunda.cluster.node-id-provider.type`
(`defaults.yaml:187-190`, default `fixed`) offers a StatefulSet-ordinal provider in 8.9, or the
container command wraps the entrypoint (`sh -c 'export CAMUNDA_CLUSTER_NODEID=${HOSTNAME##*-};
exec /usr/local/camunda/bin/camunda'`), the way the Camunda Helm chart does. The renderer
isolates this in one place.

### Configuration rendering

The renderer produces environment variables in layers; a later layer wins by name:

1. **Cluster identity** — `camunda.cluster.name` (the CR name), `size` (`zeebe.replicas`),
   `partition-count`, `replication-factor`, `node-id` (brokers), `initial-contact-points`
   (`<name>-zeebe-<ordinal>.<name>-zeebe.<ns>.svc:26502` for every broker),
   `camunda.cluster.gateway-id` (`POD_NAME` field ref, standalone gateway processes),
   `camunda.api.grpc.address=0.0.0.0` (default, set for clarity), ports left at their defaults
   (gRPC 26500 `camunda.api.grpc.port`, REST 8080 `server.port`, management 9600
   `management.server.port`, internal 26502, command 26501).
2. **Secondary storage** from the resolved `SecondaryStorageConfig`:
   - Elasticsearch: `camunda.data.secondary-storage.type=elasticsearch`,
     `...elasticsearch.url`, `...username` and `...password` (`secretKeyRef` into the binding's
     credentials Secret), and for an HTTPS endpoint with `caSecretRef`:
     `...elasticsearch.security.enabled=true`, `...security.certificate-path=/etc/camunda/es-ca/<key>`
     (the CA Secret mounted read-only), `...security.verify-hostname=true`,
     `...security.self-signed=false` (`SecondaryStorageSecurity.java`). Index prefix stays
     empty. Exporters are auto-configured by the binary
     (`camunda.data.secondary-storage.autoconfigure-camunda-exporter=true`,
     `BrokerBasedPropertiesOverride.java:126-131`); the operator sets no `zeebe.broker.exporters.*`.
   - RDBMS: `type=rdbms`, `...rdbms.url=jdbc:postgresql://<host>:<port>/<database>` built from
     the binding's `DatabaseConfig` → `DatabaseServerConfig`, `...rdbms.username`/`password`
     (`secretKeyRef` into the `DatabaseConfig` credentials Secret),
     `...rdbms.database-vendor-id=postgresql` (`MyBatisConfiguration.java:119-122`,
     `RdbmsDatabaseIdProvider.java:18-25`). The rdbms `url` default in the binary is the
     Elasticsearch URL, so the operator always sets it. **Verify in implementation**: the
     PostgreSQL JDBC driver ships in the image (`dist/src/main/resources/application-rdbmsPostgres.yaml`
     suggests it) or must be provided through the `/driver-lib` volume.
3. **Authentication and platform settings** from the effective source (platform config →
   preset `auth` → cluster `auth`):
   - `camunda.security.authentication.method=basic|oidc`
     (`AuthenticationMethod.java`, default `BASIC`).
   - OIDC: `camunda.security.authentication.oidc.issuer-uri`, `client-id`, `client-secret`
     (`secretKeyRef`), `audiences` (the `audience`, default the client id), `jwk-set-uri`,
     `token-uri`, `authorization-uri` when the platform config overrides discovery, and
     `redirect-uri` computed from `spec.externalUrl` (`OidcAuthenticationConfiguration.java`).
     The redirect path is **verified in implementation** against the 8.9 login flow (the
     Camunda docs and `authentication/`), and the platform config's `issuerBackendUrl` maps
     onto the property that the source uses for backend token validation, if one exists in
     8.9 — otherwise the doc drops the field. There is no unified "external base URL"
     property; `externalUrl` is used for the redirect URI only.
   - Basic: the binary seeds no user (`InitializationConfiguration.java:23`; the `demo/demo`
     constants are used by test code only), so the operator creates the initial admin:
     `camunda.security.initialization.users[0].{username,password,name,email}` and
     `camunda.security.initialization.default-roles.admin.users[0]`
     (`InitializationConfiguration.java`, `PlatformDefaultEntities.java:269-276`). The
     credentials live in a Secret the operator owns, `<name>-camunda-admin` (keys `username`,
     `password`), generated with `pkg/credentials` and stable across reconciles; the password
     reaches the container as a `secretKeyRef`. This is a doc addition (see "Doc deviations").
   - License: `camunda.license.key` as a `secretKeyRef` into the platform config's
     `licenseSecretRef` (`ManagementServicesConfiguration.java:39-40`).
   - Image registry: the platform config registry prefixes `camunda/camunda` and the
     connectors image.
4. **Process role** from the topology (`camunda.mode`, `camunda.webapps.<app>.enabled`).
5. **User overrides**: global `extraEnv`/`extraEnvFrom`, then per-component; a component entry
   wins over a global one by name; preset entries were merged in step "Preset merge" already.

*JVM.* Every unified process gets `JAVA_TOOL_OPTIONS=-XX:MaxRAMPercentage=… -XX:+ExitOnOutOfMemoryError`
derived from the memory limit when one is set (`JAVA_TOOL_OPTIONS` is read by the JVM itself;
the launcher's handling of `JAVA_OPTS` is not verifiable in the source). Users override through
`extraEnv`.

*Rollout on configuration change.* The pod template carries `camunda.io/config-hash`: a hash of
the rendered environment (references, not secret values) plus the resource versions of the
referenced Secrets and the generations of the referenced CRs. A change to any of them restarts
the pods through the ordinary rolling update.

*Images.* `camunda/camunda:<version>` (`camunda.Dockerfile`, build invocations in the repo's
workflows) for every unified process, entrypoint `/usr/local/camunda/bin/camunda`, user
`1001:1001`, data volume `/usr/local/camunda/data`, ports 8080, 26500, 26501, 26502, 9600. The
connectors image is **verified in implementation** against the 8.9 Helm chart
(`~/Documents/camunda/helm-charts-mirror`): the repo shows both `camunda/connectors-bundle`
and `camunda/connectors`; the operator pins the chart's default and exposes it through the
platform config registry prefix.

*Connectors.* The runtime is a separate repository (`camunda/connectors`); its properties are
the Camunda Spring Boot starter's `camunda.client.*` (`CamundaClientProperties.java`):
`camunda.client.mode=self-managed`, `grpc-address=http://<gateway>:26500`,
`rest-address=http://<gateway>:8080`, and `camunda.client.auth.method` with `username`/`password`
(basic, the operator's admin user) or `client-id`/`client-secret`/`issuer-url`/`audience`
(OIDC). The gateway address is the gateway Service, or the zeebe Service in all-in-one.

### Health, conditions, suspend, pause

Probes on the management port 9600 (`application.properties`,
`HealthConfigurationInitializer.java`): liveness `/actuator/health/liveness`, readiness
`/actuator/health/readiness`, startup `/actuator/health/startup`; connectors use the
runtime's own actuator health (verified with the image). ocf's workload primitives turn pod
readiness into the component condition: `ZeebeReady`, `GatewayReady`, `OperateReady`,
`TasklistReady`, `IdentityReady` (only for standalone processes), `ConnectorsReady`.
`Ready` is `conditions.Aggregate` over every process component; connectors is part of it (a
user-enabled workload, unlike the ES metrics exporter). Pre-check failures use
`InvalidReference` (preset, platform config, storageRef binding and its DatabaseConfig and
DatabaseServerConfig chain, backup/document ObjectStorageConfig, invalid merged spec with the
fields named) and `MissingSecret` (auth client secret, license, storage credentials, CA,
DatabaseConfig credentials). `Suspended` is `Ready=True`. There is no `Progressing` reason and
no separate `Suspended` condition.

`spec.suspend`: every process component is suspended (ocf scales the workloads to zero;
broker PVCs stay). `spec.pause`: reconcile returns before the pre-checks and records one
`Paused` event; nothing is written, including status.

`ObjectStorageConfig` references are resolved for existence only in this batch.

### Zeebe storage and StatefulSet lifecycle

- `spec.zeebe.storageSize` grows in place: the operator patches the broker
  PersistentVolumeClaims (`camunda.io/cluster` + `camunda.io/component=zeebe` selector) to the
  new size; the storage class must allow expansion, and a rejected patch surfaces on
  `ZeebeReady` through the PVC event message. Shrink: CEL rejects an inline decrease; a decrease
  through the preset is clamped to the largest bound claim with a `StorageShrinkIgnored`
  Warning event (Batch B rule). `status.storageSize` reports the smallest bound broker claim.
- The volume claim template of the StatefulSet is immutable. When the rendered template
  differs from the applied one (size growth for future replicas), the operator deletes the
  StatefulSet with `orphan` propagation and re-applies it; pods and claims stay and the new
  StatefulSet adopts them (event `StatefulSetRecreated`).
- `spec.zeebe.storageClassName` and a `partitions` decrease are rejected by CEL.
- `podManagementPolicy: Parallel` and `updateStrategy: RollingUpdate` (the shape the Camunda
  Helm chart and the upstream operator use); the StatefulSet's own
  `persistentVolumeClaimRetentionPolicy` maps from
  `spec.zeebe.persistentVolumeClaimRetentionPolicy` (`whenDeleted: Delete` default,
  consistent with `ElasticsearchCluster`; `whenScaled: Retain` fixed, so a scale-down never
  erases a broker's data). `securityContext.fsGroup: 1001` matches the image user.

### Preset merge

`MergePreset(spec, preset)` implements the preset doc rules field by field: scalars and
pointers override individually (`version`, `auth` fields, per-component `mode`, `replicas`,
`partitions`, `replicationFactor`, `storageClassName`, `storageSize`, `connectors.enabled`,
`persistentVolumeClaimRetentionPolicy`); `resources` per request/limit entry; `extraEnv` by
variable name with the cluster winning; `extraEnvFrom` concatenated preset first; `podLabels`
and `podAnnotations` by key; `scheduling` replaced entirely at the level where the cluster sets
it. Instance-bound fields (`presetRef`, `platformConfigRef`, `storageRef`, `backupStorageRef`,
`documentStorageRef`, `externalUrl`, `suspend`, `pause`) are cluster-only and rejected in a
preset by CEL. `ValidateMerged` checks: version present and 8.9 or later (three-segment),
`replicationFactor <= replicas`, `partitions >= 1`, connectors sizing only when enabled.

### Watches and indexes

`pkg/refindex` field indexes on `spec.presetRef`, `spec.platformConfigRef`, `spec.storageRef`
(namespaced), the DatabaseConfig behind an rdbms binding, the ObjectStorageConfig refs, and
every Secret the effective configuration references (auth, license, storage credentials, CA,
DatabaseConfig credentials). Metadata-only Secret watches, as in Batch A and B.

### Testing

- **Unit (pure package):** `MergePreset` per rule, `ValidateMerged`, `Resolve` per topology row,
  the renderer's layer order and win-by-name, `pkg/camundaconfig` name conversion, and the
  "declared keys only" test. Goldens per fixture: `minimal`, `default` (8.9 default topology,
  ES, basic), `all-in-one`, `separated` (every web app standalone, connectors), `rdbms`,
  `oidc`, `preset`, `suspended`; one YAML per process component.
- **envtest** (`internal/controller/camundacluster`, real CRDs including prometheus-operator's
  ServiceMonitor): wiring and owner refs, labels, per-process conditions and `Ready` mirroring
  (workload status stamped by the specs), each pre-check reason with the reference named,
  watch-driven rollout (config hash changes on platform config, preset, binding, Secret
  edits), suspend and resume, pause writes nothing, storage growth patches PVCs and recreates
  the StatefulSet with orphaned pods, CEL rejections, schema specs for both kinds.
- **e2e (kind, extends the Batch B suite):** an 8.9 default-topology cluster (1 broker,
  1 gateway, connectors) with basic auth on the Batch B `ElasticsearchCluster`: `Ready: Healthy`;
  `GET /v2/topology` on the gateway (REST port 8080, `TopologyController.java`) reports the
  broker and partitions; Operate, Tasklist, and Identity answer on the gateway; a process is
  deployed and an instance started through the REST API with the admin credentials and shows
  up in Operate's API (export to secondary storage works); connectors ready; suspend to zero and
  resume with the deployed process still present; deletion garbage-collects the workloads and
  the broker PVC follows the default retention. A second flow runs the same cluster on the Batch
  B `Database` (RDBMS). If the runner cannot host both backends and Camunda, the RDBMS flow
  runs as a separate job.

## Risks

- **Configuration drift.** The main risk is a key that 8.9 does not read. Mitigation is the
  source-of-truth rule, `pkg/camundaconfig` with pointers, the declared-keys test, and e2e
  that proves effect (topology, export) rather than shape.
- **Open verification points** (identity profile, node id, JDBC driver, redirect path,
  connectors image): each has a fallback stated above; the implementer resolves them against
  the source before the goldens are committed and records the answer in the doc.
- **StatefulSet recreate-with-orphan** is the delicate mechanic; envtest covers it, e2e once.
- **e2e resources** on the runner (ES + Camunda + Postgres); fallback is a split job.

## Doc deviations (applied in this batch)

- Status table: no `Progressing`, no separate `Suspended` condition; `Ready` mirrors the
  highest-priority process condition; `Suspended` is `Ready=True`.
- `spec.zeebe.persistentVolumeClaimRetentionPolicy` added (`whenDeleted: Delete` default);
  `status.storageSize` added; storage-shrink handling described as above.
- Field manager is ocf's `CamundaCluster/<process>`, not `camunda-operator/camundacluster`.
- Basic auth: the operator-created admin credentials Secret `<name>-camunda-admin` documented.
- Env examples corrected to verified names; `camunda.webapps.enabled` removed from the doc.
- Steps 8-9 (backup wiring) marked "lands with Batch D".
- Platform config: `issuerBackendUrl` kept only if 8.9 has a property for it (verified in
  implementation), otherwise removed with a note.

## Verified configuration (8.9.9)

Keys the operator renders, with the source that declares them. `pkg/camundaconfig` carries
these pointers in code.

| Key | Source |
| --- | --- |
| `camunda.mode` (`all-in-one`, `broker`, `gateway`) | `Camunda.java:35-42`, `ModesAndProfilesProcessor.java:27` |
| `camunda.webapps.{operate,tasklist,identity}.enabled` | `Webapps.java`, `defaults.yaml:1415-1435` |
| `camunda.cluster.{name,size,partition-count,replication-factor,node-id,initial-contact-points,gateway-id}` | `Cluster.java`, `defaults.yaml:73-290` |
| `camunda.api.grpc.{address,port}` | `Grpc.java:39-42`, `defaults.yaml:14-25` |
| `server.port` (8080), `management.server.port` (9600) | `dist/src/main/resources/application.properties` |
| `camunda.data.secondary-storage.type` (`elasticsearch`, `rdbms`) | `SecondaryStorage.java:118-123` |
| `camunda.data.secondary-storage.elasticsearch.{url,username,password}` | `defaults.yaml:607-642` |
| `camunda.data.secondary-storage.elasticsearch.security.{enabled,certificate-path,verify-hostname,self-signed}` | `SecondaryStorageSecurity.java` |
| `camunda.data.secondary-storage.rdbms.{url,username,password}` | `defaults.yaml:797-819` |
| `camunda.data.secondary-storage.rdbms.database-vendor-id` | `MyBatisConfiguration.java:119-122` |
| `camunda.security.authentication.method` | `AuthenticationProperties.java:14`, `AuthenticationMethod.java` |
| `camunda.security.authentication.oidc.{issuer-uri,client-id,client-secret,audiences,jwk-set-uri,token-uri,authorization-uri,redirect-uri}` | `OidcAuthenticationConfiguration.java:33-61` |
| `camunda.security.initialization.users[N].{username,password,name,email}`, `default-roles.admin.users[N]` | `InitializationConfiguration.java`, `ConfiguredUser.java`, `PlatformDefaultEntities.java` |
| `camunda.license.key` | `ManagementServicesConfiguration.java:39-40` |
| `camunda.data.primary-storage.directory` (default `data` → `/usr/local/camunda/data`) | `PrimaryStorage.java:24`, `camunda.Dockerfile:122-131` |
| Health: `/actuator/health/{liveness,readiness,startup}` on 9600 | `HealthConfigurationInitializer.java:59-61` |
| Topology: `GET /v2/topology` on 8080 | `TopologyController.java:19-29` |
| Connectors: `camunda.client.{mode,grpc-address,rest-address,auth.*}` | `CamundaClientProperties.java`, `CamundaClientAuthProperties.java` |

Not set by the operator, by decision: `camunda.webapps.enabled` (derived),
`zeebe.broker.gateway.enable` (set by `camunda.mode`), `zeebe.broker.exporters.*`
(auto-configured), `spring.profiles.active` (unless the identity-profile verification fails).

## Alternatives considered

- **One component for all workloads** — rejected: loses the per-process conditions and makes a
  broken connectors runtime indistinguishable from a broken broker.
- **`application.yaml` ConfigMap instead of env vars** (the upstream operator's gateway shape)
  — rejected for this operator: env keeps CamundaOptimize's SSA env injection trivial and
  matches the doc; the config-hash annotation gives the same rollout behaviour.
- **`spring.profiles.active` instead of `camunda.mode`** — the upstream operator's choice;
  kept as the fallback only, because `camunda.mode` is the documented 8.9 control surface and
  derives the profiles itself.
- **Wholesale preset merge like Batch B** — rejected: the component blocks are large and a
  cluster that sets one field must not lose the rest of the baseline.
- **Backup wiring in this batch** — rejected: nothing exercises it before the `Backup` kinds.
