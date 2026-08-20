# Authentication

You configure authentication once per environment, on a `CamundaPlatformConfig`. Every `CamundaCluster` that references that platform config uses the same authentication method. The OIDC client credentials of one cluster and its administrators go on the `CamundaCluster`, or on a `CamundaClusterPreset` that many clusters share.

The orchestration cluster supports two methods: `basic` and `oidc`. The operator uses `basic` when the platform config has no `auth` block.

If you want the whole picture in one place, read [A complete OIDC example](#a-complete-oidc-example) first. The sections before it explain each part.

## Basic authentication

Under basic authentication the orchestration cluster stores its users itself, and every caller sends a username and a password. The operator creates the first administrator for you. The user is named `admin` and is a member of the `admin` role. You manage every other user in the Admin web application.

The credentials live in the Secret `<name>-camunda-admin`, in the namespace of the `CamundaCluster`. Read `username` (`admin`) and `password`. The operator generates the password once and keeps it. The Secret also holds the bookkeeping of a rotation: `password-rotation` names the request that the current password answers, and `password-pending` with `password-pending-rotation` appear only while a rotation is in flight. Do not read those keys; they are for the operator. The condition `AdminSecretReady` reports that the Secret is applied, and it takes part in `Ready`.

The connectors runtime authenticates against the cluster with the same user and password. You configure nothing for it.

To read the password of the cluster `my-cluster`:

```bash
kubectl get secret my-cluster-camunda-admin -n my-cluster-ns -o go-template='{{.data.password | base64decode}}'
```

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

### Rotate the password

Set `spec.auth.basic.passwordRotation` on the `CamundaCluster` to a value that differs from the last one, for example a date:

```yaml
spec:
  auth:
    basic:
      passwordRotation: "2026-08"
```

The operator generates a new password, sets it on the `admin` user through the user API of the running cluster, and publishes it in the Secret. The connectors Deployment restarts with the new password; the brokers, the gateway, and the web applications keep running. Every other user keeps its password. `status.adminPassword.rotation` shows the value when the rotation is complete. The same value never rotates twice, so a GitOps tool can apply it repeatedly. A suspended cluster serves no user API, so a requested rotation waits and applies after the cluster resumes.

If the call fails, the Secret keeps the active password and the operator retries until the call succeeds. `AdminSecretReady` reports which of the three failures it is, and each one asks for something different from you:

| Reason | What happened | What to do |
| --- | --- | --- |
| `ConnectionFailed` | The cluster did not answer. | Nothing. It clears on its own when the cluster is `Ready` again. |
| `InvalidCredentials` | The cluster refused the password that the Secret publishes. Somebody changed it in the Admin web application. | Set the password from the Secret on the `admin` user there. The next retry succeeds. |
| `Rejected` | The cluster accepted the password and refused the call itself. | Read the condition message, which carries the answer of the cluster and names the reason. A new password does not help. |

> **Caution:** Between the update of the user and the restart of the connectors pods, a connectors call with the old password is rejected. Plan a rotation outside of peak hours.

Do not rotate by deletion. A deleted Secret gets a new password, but the `admin` user keeps the old one. The old password is not published again, so read and keep it before you delete the Secret. You then sign in to the Admin web application with the old password, set the new password from the new Secret on the `admin` user, and run `kubectl rollout restart deployment/<name>-connectors`. A `passwordRotation` requested after the deletion fails with `InvalidCredentials`, because the operator no longer holds a password that the cluster accepts.

The extra steps of a deletion come from the orchestration cluster, not from the operator. The operator passes the user and the password as the initial user of the cluster. The cluster creates that user once, at first start. After that it checks only that the username exists, and it ignores the password in the configuration. `passwordRotation` exists because of that: it sets the password through the user API of the running cluster, which is the only path that changes it.

## OIDC

Under OIDC an external identity provider authenticates every caller, and the orchestration cluster stores no users. A person signs in through the browser and gets a token. A machine client gets a token with its client credentials. The cluster reads the token and decides who the caller is. The provider authenticates the caller. It does not authorize the caller. A new OIDC cluster has no administrator until you name one on the `CamundaCluster` or on its preset.

### Configure the identity provider

Create one confidential client in your identity provider for the orchestration cluster. The operator needs these values from the provider:

| Value | Where it goes |
| --- | --- |
| The issuer URL of the realm or tenant | `spec.auth.oidc.issuerUrl` on the platform config |
| The client id | `spec.auth.oidc.clientId` on the platform config, or `spec.auth.clientId` on the preset or the cluster |
| The client secret, in a Secret | `spec.auth.oidc.clientSecretRef` on the platform config, or `spec.auth.clientSecretRef` on the preset or the cluster |

Register the redirect URI of the cluster on that client. The operator derives it from `spec.externalUrl` of the `CamundaCluster`: the redirect URI is `<externalUrl>/sso-callback`. For the cluster at `https://camunda.example.com` it is `https://camunda.example.com/sso-callback`. If the cluster has no `externalUrl`, the operator sets no redirect URI, and the orchestration cluster uses its own default.

Make sure that the access tokens carry these claims:

- `aud` holds the audience that the cluster validates. The audience is the client id unless you set `audience`. Some providers need a mapper for this. Keycloak, for example, issues `aud: account` by default, and the cluster refuses that token.
- The claim that you name in `usernameClaim` holds the username of a person. Common names are `preferred_username`, `email`, and `sub`.
- The claim that you name in `clientIdClaim` holds the client id of a machine client. This claim must be absent from the tokens of persons. See [How a token becomes a person or a client](#how-a-token-becomes-a-person-or-a-client).

The end-to-end tests of the operator run this flow against Keycloak. The realm has one confidential client with service accounts enabled, an audience mapper that puts the client id in `aud`, and a hardcoded-claim mapper that puts `client_id` in the access tokens of the client. Keycloak names the client in `azp`, and `azp` is also present in the tokens of persons. That is why the tests add a separate claim.

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

The Secret can live in any namespace. If it is not in the namespace of a cluster, the operator copies the key into that namespace as the Secret `<name>-camunda-oidc-client`. The operator watches the source Secret. When you change the client secret there, the operator updates the copy and rolls the pods that read it. You do not restart anything.

### How a token becomes a person or a client

`usernameClaim` and `clientIdClaim` tell the cluster who is behind a token. The cluster reads a token in this order:

1. If the token holds the client id claim, the caller is a machine client with that id.
2. If not, and the token holds the username claim, the caller is a person with that username.
3. If the token holds neither claim, the cluster refuses the request.

If `clientIdClaim` is unset, every token becomes a person. The claims only say who the caller is. They grant nothing. The `spec.auth.admin` block of the `CamundaCluster` makes a caller an administrator.

> **Caution:** The claim that you name in `clientIdClaim` must be present in the tokens of machine clients only. If the tokens of persons also carry that claim, every person becomes a machine client, and the user list of `spec.auth.admin` never matches anybody. Keycloak, for example, puts `azp` in every token, so `azp` is a bad choice. If one client serves both the browser login and the machine callers, add a claim that the provider puts in machine tokens only, and name that claim. The Camunda documentation gives the same advice.

### Name the administrators

Nothing authorizes a caller until `spec.auth.admin` names one. A cluster without this block, on the cluster or on its preset, has no administrator. The block takes three kinds of member, and all of them become members of the `admin` role:

| Field | Matched against | Needs |
| --- | --- | --- |
| `users` | The value of `usernameClaim` in the token | Nothing |
| `clients` | The value of `clientIdClaim` in the token | `clientIdClaim` set on the platform config |
| `mappingRules` | A claim name and a claim value in the token | Nothing |

A mapping rule is the general form. It gives the role to every token whose claim `claimName` holds `claimValue`, so one rule can cover a whole group from the identity provider.

The block names members of the `admin` role only. The orchestration cluster has other default roles, for example `readonly-admin`, and it lets you create roles, groups, and authorizations of your own. The operator has no field for those. You have two ways to manage them:

- In the Admin web application of the cluster, as an administrator. This is the usual way.
- With the `CAMUNDA_SECURITY_INITIALIZATION_*` environment variables of the orchestration cluster, through `extraEnv` on the cluster or the preset. The cluster reads them at first start. It creates an entity once and does not update it when the value changes. See [Identity as Code](https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/core-settings/configuration/admin-identity-as-code/) in the Camunda documentation. For example, to give a user the `readonly-admin` role:

    ```yaml
    spec:
      extraEnv:
        - name: CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_READONLYADMIN_USERS_0
          value: "grace@example.com"
    ```

> **Caution:** The web applications show the first-run setup page at `/admin/setup` while the `admin` role has no user. A cluster whose only administrator is a client works over the API, but a browser still lands on that page. List a user too when people sign in to the cluster.

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
      namespace: my-cluster-ns
      key: client-secret
```

Each field overrides the default of the platform config on its own. The issuer, the endpoints, and the claim names always come from the platform config.

When you set `clientId` and not `audience`, the audience becomes the new client id, not the audience of the platform config. This is a rule of the operator. The audience belongs to a client: the provider puts the id of the client that a token was issued for in `aud`, so a new client id most often means a new audience. If your new client uses a different audience, set `audience` next to `clientId`. The orchestration cluster itself has no default audience. The operator always sets one.

A `CamundaClusterPreset` can carry the same `spec.auth` fields as a baseline for many clusters. The cluster overrides the client fields of the preset one by one. The `admin` block never merges: when the cluster sets `spec.auth.admin`, it replaces the whole block of the preset.

Under basic authentication the cluster ignores `spec.auth`. It does not reject it.

### Connectors

You configure nothing for the connectors runtime. The operator gives it the credentials of the cluster:

- Under basic authentication, the `admin` user and its password from the Secret `<name>-camunda-admin`.
- Under OIDC, the OIDC client of the cluster: the resolved client id, client secret, issuer URL, and audience.

The runtime needs the `connectors` role of the cluster. The operator gives that role to the OIDC client when both conditions hold: `spec.connectors.enabled` is `true`, and the platform config sets `clientIdClaim`. Without a client id claim, the cluster reads the token of the runtime as a person, and a client member never matches it. In that case, give the `connectors` role to that person yourself in the Admin web application. The username is the value of `usernameClaim` in the token of the client.

The only authentication choice for connectors is therefore which OIDC client the cluster uses. A cluster that shares the client of the platform config shares it with the runtime too. A cluster with a [per-cluster client](#per-cluster-client) gives that client to the runtime.

## A complete OIDC example

This example sets up one environment in which a team can create a cluster in a few lines. It has four resources that you create once, and one resource per cluster.

The identity provider has one confidential client `camunda` with the secret in the Secret `oidc-credentials`. Its access tokens carry `aud: camunda`, `preferred_username` for persons, and `client_id` for the client.

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

2. A preset with the sizing, connectors, and the administrators that every cluster gets: the members of the group `camunda-admins` in the provider, and the client `camunda` for automation. The `connectors` role of the client comes from the operator, because `clientIdClaim` is set.

    ```yaml
    apiVersion: core.camunda.io/v1
    kind: CamundaClusterPreset
    metadata:
      name: medium
    spec:
      cluster:
        version: "8.9.9"
        zeebe:
          replicas: 3
          partitions: 3
          replicationFactor: 3
          storageSize: "32Gi"
        connectors:
          enabled: true
          version: "8.9.7"
        auth:
          admin:
            clients:
              - "camunda"
            mappingRules:
              - id: "platform-admins"
                claimName: "groups"
                claimValue: "camunda-admins"
    ```

3. One cluster. It inherits the client, the administrators, and connectors from the layers above. It sets only what is its own: the names of the references, the URL, and the storage.

    ```yaml
    apiVersion: core.camunda.io/v1
    kind: CamundaCluster
    metadata:
      name: orders
      namespace: orders
    spec:
      presetRef: medium
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
          namespace: payments
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

    The audience of this cluster is `camunda-payments`, because `audience` is unset. The connectors runtime of this cluster authenticates as `camunda-payments` and gets the `connectors` role from the operator.

## Where settings live

| Setting | Kind and field | Which wins |
| --- | --- | --- |
| Authentication method | `CamundaPlatformConfig` `spec.auth.method` | Only the platform config sets it |
| Issuer URL and explicit endpoints | `CamundaPlatformConfig` `spec.auth.oidc.issuerUrl`, `jwksUrl`, `tokenUrl`, `authUrl` | Only the platform config sets them |
| `usernameClaim`, `clientIdClaim` | `CamundaPlatformConfig` `spec.auth.oidc` | Only the platform config sets them |
| Client id, audience, client secret | `CamundaPlatformConfig` `spec.auth.oidc`, then `CamundaClusterPreset` `spec.cluster.auth`, then `CamundaCluster` `spec.auth` | The cluster, then the preset, then the platform config, field by field |
| Administrators | `CamundaClusterPreset` `spec.cluster.auth.admin`, then `CamundaCluster` `spec.auth.admin` | The cluster replaces the whole block of the preset |
| Redirect URI | `CamundaCluster` `spec.externalUrl` | Only the cluster sets it |
| Connectors credentials | None. The operator derives them from the rows above | - |
| Basic-auth admin password | Secret `<name>-camunda-admin` | The operator generates it |

## Related

- [Presets](presets.md): how the platform config, the preset, and the cluster layer, and why a cluster stays small.
- [CamundaPlatformConfig](../crds/camundaplatformconfig.md): the authentication method and the identity provider connection.
- [CamundaCluster](../crds/camundacluster.md): the per-cluster client credentials, the administrators, and `externalUrl`.
- [CamundaClusterPreset](../crds/camundaclusterpreset.md): the baseline between the platform config and the cluster, and the merge rules.
