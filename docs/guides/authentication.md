# Authentication

You choose the authentication method once per environment, on a `CamundaPlatformConfig`. Every `CamundaCluster` that references that platform config uses the same method. The OIDC client of one cluster and its administrators go on the `CamundaCluster`, or on a `CamundaClusterPreset` that many clusters share.

The orchestration cluster supports two methods: `basic` and `oidc`. The operator uses `basic` when the platform config has no `auth` block.

This guide covers the orchestration cluster. Management Identity, Console, Web Modeler, and Optimize sign in through the management plane. See [CamundaManagementCluster](../crds/camundamanagementcluster.md) and [CamundaOptimize](../crds/camundaoptimize.md#authentication).

For the whole OIDC setup in one place, read [A complete OIDC example](#a-complete-oidc-example).

## Basic authentication

Under basic authentication the orchestration cluster stores its users itself, and every caller sends a username and a password. The operator creates the first administrator for you: the user `admin`, a member of the `admin` role. You manage every other user in the Admin web application.

A minimal platform config for basic authentication:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaPlatformConfig
metadata:
  name: my-platform-config
spec:
  auth:
    method: basic
```

A platform config with no `auth` block has the same effect.

The credentials of `admin` are in the Secret `<name>-camunda-admin`, in the namespace of the `CamundaCluster`. Read the keys `username` and `password`. The other keys are for the operator. The operator generates the password once and keeps it. The condition `AdminSecretReady` reports that the Secret is applied, and it takes part in `Ready`.

To read the password of the cluster `my-cluster`:

```bash
kubectl get secret my-cluster-camunda-admin -n my-cluster-ns -o go-template='{{.data.password | base64decode}}'
```

The connectors runtime signs in with the same user and password. You configure nothing for it.

### Rotate the password

Set `spec.auth.basic.passwordRotation` on the `CamundaCluster` to a value that differs from the last one, for example a date:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # ... the rest of your cluster
  auth:
    basic:
      passwordRotation: "2026-08"
```

The operator generates a new password and sets it on the `admin` user through the user API of the running cluster. Then it publishes the password in the Secret, and the connectors Deployment restarts with it. The brokers, the gateway, and the web applications keep running, and every other user keeps its password.

`status.adminPassword.rotation` shows the value when the rotation is complete. The same value never rotates twice, so a GitOps tool can apply it again and again. A preset can set `passwordRotation` too. Then it rotates the password of every cluster that uses the preset. A suspended cluster serves no user API, so the rotation waits until the cluster resumes.

> **Caution:** Between the update of the user and the restart of the connectors pods, the cluster refuses a connectors call with the old password. Rotate outside of peak hours.

If the call fails, the Secret keeps the active password, and the operator tries again. `AdminSecretReady` tells you which failure it is:

| Reason | What happened | What to do |
| --- | --- | --- |
| `ConnectionFailed` | The cluster did not answer. | Nothing. It clears when the user API of the gateway answers again. Until then, the cluster is not `Ready`. |
| `InvalidCredentials` | The cluster refused the password that the Secret publishes. Somebody changed it in the Admin web application. | Set the password from the Secret on the `admin` user there. The next try succeeds. |
| `Rejected` | The cluster accepted the password and refused the call itself. | Read the condition message. It carries the answer of the cluster. A new password does not help. |

A failed change of `adminEmail` reports the same three reasons, because it is an update of the same user.

### Do not rotate by deletion

If you delete the Secret, the operator publishes a new password, and the connectors restart with it. But the `admin` user keeps the old password. The orchestration cluster creates its initial user only when no user of that name exists, so it ignores the new password. Only `passwordRotation` changes the password of a running cluster.

After a deletion, the connectors fail to sign in, and a `passwordRotation` fails with `InvalidCredentials`. To repair it:

1. Sign in to the Admin web application as `admin` with the old password.
2. Set the password from the new Secret on the `admin` user.

The old password is not published again. If you must delete the Secret, read and keep the password first. You do not need to restart the connectors after the repair.

### Set the address of the administrator

The orchestration cluster stores an email address on every user, and the Admin web application shows it. Set the address of the person or the team that owns the cluster:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # ... the rest of your cluster
  auth:
    basic:
      adminEmail: platform-team@example.com
```

Without the field, the address is `admin@example.com`. That domain is reserved for documentation, so it belongs to nobody.

The operator applies a changed address to the running cluster through the user API, and restarts nothing. The user API refuses an address with no dot in the domain. A refused address shows on `AdminSecretReady` with the answer of the cluster. Until the cluster accepts the address, the Secret key `email` shows the address you asked for, and `email-applied` shows the one the cluster holds.

## OIDC

Under OIDC an external identity provider authenticates every caller, and the orchestration cluster stores no users. Camunda describes the mode and what it needs from a provider in [Connect Camunda to any OIDC provider](https://docs.camunda.io/docs/self-managed/deployment/helm/configure/authentication-and-authorization/generic-oidc-provider/).

The provider authenticates a caller. It does not authorize one. A new OIDC cluster has no administrator until you name one on the `CamundaCluster` or on its preset.

### Configure the identity provider

Create one confidential client in your identity provider for the orchestration cluster. The operator needs these values from the provider:

| Value | Where it goes |
| --- | --- |
| The issuer URL of the realm or tenant | `spec.auth.oidc.issuerUrl` on the platform config |
| The client id | `spec.auth.oidc.clientId` on the platform config, or `spec.auth.clientId` on the preset or the cluster |
| The client secret, in a Secret | `spec.auth.oidc.clientSecretRef` on the platform config, or `spec.auth.clientSecretRef` on the preset or the cluster |

Register the redirect URI of each cluster on that client: `<externalUrl>/sso-callback`, from `spec.externalUrl` of the `CamundaCluster`. For the cluster at `https://camunda.example.com` it is `https://camunda.example.com/sso-callback`. If the cluster has no `externalUrl`, the orchestration cluster uses its own default.

Make sure that the access tokens carry these claims:

- `aud` holds the audience that the cluster validates. The audience is the client id unless you set `audience`.
- The claim that you name in `usernameClaim` on the platform config holds the username of a person. Common names are `preferred_username`, `email`, and `sub`.
- The claim that you name in `clientIdClaim` on the platform config holds the client id of a machine client. A machine client is an application that signs in with its own client id and client secret, not a person in a browser. This claim must be absent from the tokens of persons. See [How a token becomes a person or a client](#how-a-token-becomes-a-person-or-a-client).

For Keycloak, add these two mappers to the client:

- An audience mapper that puts the client id in `aud`. Keycloak issues `aud: account` by default, and the cluster refuses that token.
- A hardcoded-claim mapper that puts `client_id` in the access tokens of the client. Keycloak puts the client in `azp`, but `azp` is also in the tokens of persons, so it cannot identify a client.

### Configure the operator

Put the provider connection on the platform config:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: oidc-credentials
  namespace: camunda-system
stringData:
  client-secret: "<the client secret from the identity provider>"
---
apiVersion: core.camunda.io/v1
kind: CamundaPlatformConfig
metadata:
  name: my-platform-config
spec:
  auth:
    method: oidc
    oidc:
      # Required. The issuer URL. The cluster reads the endpoints from its discovery document.
      issuerUrl: "https://login.example.com/realms/camunda"
      # Optional. Explicit endpoints. Set them only when discovery does not give the endpoint you need.
      # jwksUrl: "https://login.example.com/realms/camunda/protocol/openid-connect/certs"
      # tokenUrl: "https://login.example.com/realms/camunda/protocol/openid-connect/token"
      # authUrl: "https://login.example.com/realms/camunda/protocol/openid-connect/auth"
      # Required. The default client id of every cluster.
      clientId: "camunda"
      # Optional, default: the clientId. The audience that access tokens must carry.
      audience: "camunda"
      # Optional, default: sub. The claim that holds the username of a person.
      usernameClaim: "preferred_username"
      # Optional, default: unset. The claim that holds the id of a machine client.
      clientIdClaim: "client_id"
      # Required. The Secret key that holds the default client secret.
      clientSecretRef:
        name: oidc-credentials
        namespace: camunda-system
        key: client-secret
```

The platform config is cluster-scoped, so this Secret can live in any namespace. If it lives outside the namespace of a cluster, the operator copies the key into that namespace as the Secret `<name>-camunda-oidc-client`. When you change the client secret, the operator updates the copy and restarts the pods that read it.

[CamundaPlatformConfig](../crds/camundaplatformconfig.md) lists every field of the block.

### How a token becomes a person or a client

`usernameClaim` and `clientIdClaim` tell the cluster who is behind a token. The cluster reads a token in this order:

1. If the token holds the client id claim, the caller is a machine client with that id.
2. If not, and the token holds the username claim, the caller is a person with that username.
3. If the token holds neither claim, the cluster refuses the request.

If `clientIdClaim` is unset, every token becomes a person. The claims only say who the caller is. They grant nothing. The `spec.auth.admin` block of the `CamundaCluster` makes a caller an administrator.

> **Caution:** Name a `clientIdClaim` that only the tokens of machine clients carry. If the tokens of persons carry it too, every person becomes a machine client, and the `users` of `spec.auth.admin` never match anybody. Keycloak puts `azp` in every token, so do not use `azp`. One client can serve both the browser login and the machine callers. Then add a claim that only machine tokens carry, and name that claim.

### Name the administrators

Nothing authorizes a caller until `spec.auth.admin` names one. The block takes three kinds of member, and all of them become members of the `admin` role:

| Field | Matched against | Needs |
| --- | --- | --- |
| `users` | The value of `usernameClaim` in the token | Nothing |
| `clients` | The value of `clientIdClaim` in the token | `clientIdClaim` set on the platform config |
| `mappingRules` | A claim name and a claim value in the token | Nothing |

A mapping rule gives the role to every token whose claim `claimName` holds `claimValue`. So one rule can cover a whole group from the identity provider.

A cluster with one administrator of each kind:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  platformConfigRef: my-platform-config
  storageRef: my-storage-config
  externalUrl: "https://camunda.example.com"
  auth:
    admin:
      users:
        - "ada@example.com"
      clients:
        - "camunda"
      mappingRules:
        - id: "platform-admins"
          claimName: "groups"
          claimValue: "camunda-admins"
```

> **Caution:** An administrator that is only a client works over the API. A person who signs in with a browser gets no access until `users` or `mappingRules` matches that person. Camunda shows the result in [Test user authentication](https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/admin/connect-external-identity-provider/#step-6-test-user-authentication). If people sign in to the cluster, list a user or a mapping rule too.

The block names members of the `admin` role only. The operator has no field for other roles, groups, or authorizations. You can manage them in two ways:

- In the Admin web application of the cluster, as an administrator.
- With the `CAMUNDA_SECURITY_INITIALIZATION_*` environment variables of the orchestration cluster, through `extraEnv` on the cluster or the preset. The cluster creates each entity once, at first start, and does not update it when the value changes. See [Identity as Code](https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/core-settings/configuration/admin-identity-as-code/). For example, to give a user the `rpa` role:

    ```yaml
    apiVersion: core.camunda.io/v1
    kind: CamundaCluster
    metadata:
      name: my-cluster
      namespace: my-cluster-ns
    spec:
      extraEnv:
        - name: CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_RPA_USERS_0
          value: "grace@example.com"
      # ... the rest of your cluster
    ```

The operator refuses an `extraEnv` entry under `CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_` that Camunda cannot read as `<role>_<type>_<n>` or `<role>_<type>`. The type must be `USERS`, `CLIENTS`, `GROUPS`, `ROLES`, or `MAPPINGRULES`. Camunda stops the whole identity initialization on any other form, and the members of `spec.auth.admin` lose their access too. Two forms are common mistakes:

- An underscore in place of the dash of a role ID, as in `..._DEFAULTROLES_READONLY_ADMIN_USERS_0`. Write `READONLY-ADMIN`.
- `MAPPINGS` as the type. Write `MAPPINGRULES`.

When the operator refuses an entry, the cluster reports `Ready: InvalidReference`. A running cluster keeps the configuration that it runs:

```yaml
status:
  conditions:
    - type: Ready
      status: "False"
      reason: InvalidReference
      message: >-
        invalid effective spec: extraEnv entry CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_READONLY_ADMIN_USERS_0
        is not a default role membership that Camunda can read. Camunda stops the identity initialization
        on this entry and creates no configured user and no role member. Write
        CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_<role>_<type>_<n>, with USERS, CLIENTS, GROUPS, ROLES,
        or MAPPINGRULES as the type. Keep the dash of a role ID, as in READONLY-ADMIN
```

The operator does not read the keys of an `extraEnvFrom` source. It cannot refuse such a name in a ConfigMap or a Secret, so write these variables in `extraEnv`.

The operator does not check the indexes. For each role and type, count `<n>` up from `0` without a gap. Camunda does not start when an index is missing. The operator itself writes the members of `spec.auth.admin` from index `0` of the `admin` role. Under OIDC, it can also write the first client of the `connectors` role. An `extraEnv` entry with the same name replaces the entry of the operator. List administrators in `spec.auth.admin`, not in `extraEnv`.

### Per-cluster client

A cluster can use a client of its own:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # ... platformConfigRef, storageRef, externalUrl, and the rest of your cluster
  auth:
    clientId: "camunda-my-cluster"
    # Optional. Defaults to the clientId.
    audience: "camunda-my-cluster"
    clientSecretRef:
      name: my-cluster-oidc
      key: client-secret
```

Each field overrides the default of the platform config on its own. The Secret of `clientSecretRef` is in the namespace of the cluster. The issuer, the endpoints, and the claim names always come from the platform config.

When the cluster or its preset sets `clientId`, the audience of the platform config no longer applies. The audience is then the `audience` of the cluster or the preset, or else the new client id. If your new client uses another audience, set `audience` next to `clientId`.

A `CamundaClusterPreset` can carry the same `spec.auth` fields as a baseline for many clusters. The cluster overrides `clientId`, `audience`, and `clientSecretRef` of the preset one by one. The `admin` block never merges: when the cluster sets `spec.auth.admin`, it replaces the whole block of the preset.

The operator uses `spec.auth.basic` only under basic authentication, and every other field of `spec.auth` only under OIDC. The API server accepts both on either method.

### Connectors

You configure nothing for the connectors runtime. Under OIDC it signs in with the OIDC client of the cluster: the resolved client id, client secret, issuer URL, and audience. A cluster with a [per-cluster client](#per-cluster-client) gives that client to the runtime.

The runtime needs the `connectors` role of the cluster. The operator gives that role to the OIDC client of the cluster when `spec.connectors.enabled` is `true` and the platform config sets `clientIdClaim`. Without a client id claim, the cluster reads the token of the runtime as a person. In that case, give the `connectors` role to that person yourself in the Admin web application. The username is the value of `usernameClaim` in the token of the client.

## A complete OIDC example

This example sets up one environment in which a team can create a cluster in a few lines. You create three resources once, and one resource per cluster.

The identity provider has one confidential client `camunda`. Its access tokens carry `aud: camunda`, `preferred_username` for persons, and `client_id` for the client.

1. The Secret with the client secret, and the platform config with the provider connection. The client of the platform config is the default client of every cluster.

    ```yaml
    apiVersion: v1
    kind: Secret
    metadata:
      name: oidc-credentials
      namespace: camunda-system
    stringData:
      client-secret: "<the client secret from the identity provider>"
    ---
    apiVersion: core.camunda.io/v1
    kind: CamundaPlatformConfig
    metadata:
      name: production
    spec:
      auth:
        method: oidc
        oidc:
          issuerUrl: "https://login.example.com/realms/camunda"
          clientId: "camunda"
          usernameClaim: "preferred_username"
          clientIdClaim: "client_id"
          clientSecretRef:
            name: oidc-credentials
            namespace: camunda-system
            key: client-secret
    ```

2. A preset with the sizing, connectors, and the administrators of every cluster. The administrators are the members of the group `camunda-admins` in the provider, and the client `camunda` for automation. The client gets the `connectors` role from the operator, because `clientIdClaim` is set.

    ```yaml
    apiVersion: core.camunda.io/v1
    kind: CamundaClusterPreset
    metadata:
      name: medium
    spec:
      cluster:
        zeebe:
          replicas: 3
          partitions: 3
          replicationFactor: 3
          storageSize: "32Gi"
        connectors:
          enabled: true
        auth:
          admin:
            clients:
              - "camunda"
            mappingRules:
              - id: "platform-admins"
                claimName: "groups"
                claimValue: "camunda-admins"
    ```

3. One cluster. It gets the client, the administrators, and connectors from the layers above, and its versions from a [CamundaRelease](../crds/camundarelease.md). It sets only what is its own: the references, the URL, and the storage.

    ```yaml
    apiVersion: core.camunda.io/v1
    kind: CamundaCluster
    metadata:
      name: orders
      namespace: orders
    spec:
      presetRef: medium
      releaseRef: camunda-8-9-4
      platformConfigRef: production
      storageRef: orders-storage
      externalUrl: "https://orders.camunda.example.com"
    ```

    Register `https://orders.camunda.example.com/sso-callback` as a redirect URI on the client `camunda`.

4. A cluster that needs its own client and its own administrators sets them. The `admin` block replaces the block of the preset, so it names every administrator again.

    ```yaml
    apiVersion: core.camunda.io/v1
    kind: CamundaCluster
    metadata:
      name: payments
      namespace: payments
    spec:
      presetRef: medium
      platformConfigRef: production
      storageRef: payments-storage
      externalUrl: "https://payments.camunda.example.com"
      auth:
        clientId: "camunda-payments"
        clientSecretRef:
          name: payments-oidc
          key: client-secret
        admin:
          users:
            - "ada@example.com"
          clients:
            - "camunda-payments"
          mappingRules:
            - id: "platform-admins"
              claimName: "groups"
              claimValue: "camunda-admins"
    ```

    The audience of this cluster is `camunda-payments`, because neither the cluster nor the preset sets `audience`. The connectors runtime of this cluster signs in as `camunda-payments` and gets the `connectors` role from the operator.

## Where settings live

| Setting | Kind and field | Which wins |
| --- | --- | --- |
| Authentication method | `CamundaPlatformConfig` `spec.auth.method` | Only the platform config sets it |
| Issuer URL and explicit endpoints | `CamundaPlatformConfig` `spec.auth.oidc.issuerUrl`, `jwksUrl`, `tokenUrl`, `authUrl` | Only the platform config sets them |
| `usernameClaim`, `clientIdClaim` | `CamundaPlatformConfig` `spec.auth.oidc` | Only the platform config sets them |
| Client id, audience, client secret | `CamundaPlatformConfig` `spec.auth.oidc`, then `CamundaClusterPreset` `spec.cluster.auth`, then `CamundaCluster` `spec.auth` | The cluster, then the preset, then the platform config, field by field |
| Administrators under OIDC | `CamundaClusterPreset` `spec.cluster.auth.admin`, then `CamundaCluster` `spec.auth.admin` | The cluster replaces the whole block of the preset |
| Redirect URI | `CamundaCluster` `spec.externalUrl` | Only the cluster sets it |
| Admin email and password rotation under basic | `CamundaClusterPreset` `spec.cluster.auth.basic`, then `CamundaCluster` `spec.auth.basic` | The cluster replaces the whole block of the preset |
| Admin password under basic | Secret `<name>-camunda-admin` | The operator generates it |
| Connectors credentials | None. The operator derives them from the rows above | - |

## Related

- [Presets](presets.md): how the platform config, the preset, and the cluster layer.
- [CamundaPlatformConfig](../crds/camundaplatformconfig.md): the authentication method and the identity provider connection.
- [CamundaCluster](../crds/camundacluster.md): the per-cluster client, the administrators, and `externalUrl`.
- [CamundaClusterPreset](../crds/camundaclusterpreset.md): the baseline between the platform config and the cluster, and the merge rules.
- [CamundaManagementCluster](../crds/camundamanagementcluster.md): sign-in for Management Identity, Console, Web Modeler, and Optimize.
