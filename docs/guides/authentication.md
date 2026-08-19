# Authentication

You configure authentication once per environment, on a `CamundaPlatformConfig`. Every `CamundaCluster` that references that platform config uses the same authentication method. The OIDC client credentials of one cluster and its administrators go on the `CamundaCluster`.

The orchestration cluster supports two methods: `basic` and `oidc`. The operator uses `basic` when the platform config has no `auth` block.

## Basic authentication

Under basic authentication the orchestration cluster stores its users itself, and every caller sends a username and a password. The operator creates the first administrator for you. The user is named `admin` and is a member of the `admin` role. You manage every other user in the Admin web application.

The credentials live in the Secret `<name>-camunda-admin`, in the namespace of the `CamundaCluster`. The Secret has two keys: `username` (`admin`) and `password`. The operator generates the password once and keeps it. The condition `AdminSecretReady` reports that the Secret is applied, and it takes part in `Ready`.

The connectors runtime authenticates against the cluster with the same user and password.

To read the password of the cluster `my-cluster`:

```bash
kubectl get secret my-cluster-camunda-admin -n my-cluster-ns -o go-template='{{.data.password | base64decode}}'
```

**Rotation.** To get a new password, delete the Secret. The operator creates a new Secret with a new password on the next reconcile.

> **Caution:** The orchestration cluster creates the `admin` user once, at first start, and does not change its password from the configuration after that. After you delete the Secret, the new password works only after you set it on the `admin` user in the Admin web application. Then restart the connectors Deployment `<name>-connectors`, so that it reads the new password.

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

## OIDC

Under OIDC an external identity provider authenticates every caller, and the orchestration cluster stores no users. A person signs in through the browser and gets a token. A machine client gets a token with its client credentials. The cluster reads the token and decides who the caller is. The provider authenticates the caller. It does not authorize the caller. A new OIDC cluster has no administrator until you name one on the `CamundaCluster`.

### Configure the identity provider

Create one confidential client in your identity provider for the orchestration cluster. The operator needs these values from the provider:

| Value | Where it goes |
| --- | --- |
| The issuer URL of the realm or tenant | `spec.auth.oidc.issuerUrl` on the platform config |
| The client id | `spec.auth.oidc.clientId` on the platform config, or `spec.auth.clientId` on the cluster |
| The client secret, in a Secret | `spec.auth.oidc.clientSecretRef` on the platform config, or `spec.auth.clientSecretRef` on the cluster |

Register the redirect URI of the cluster on that client. The operator derives it from `spec.externalUrl` of the `CamundaCluster`: the redirect URI is `<externalUrl>/sso-callback`. For the cluster at `https://camunda.example.com` it is `https://camunda.example.com/sso-callback`. If the cluster has no `externalUrl`, the operator sets no redirect URI, and the orchestration cluster uses its own default.

Make sure that the access tokens carry these claims:

- `aud` holds the audience that the cluster validates. The audience is the client id unless you set `audience`. Some providers need a mapper for this. Keycloak, for example, issues `aud: account` by default, and the cluster refuses that token.
- The claim that you name in `usernameClaim` holds the username of a person. Common names are `preferred_username`, `email`, and `sub`.
- The claim that you name in `clientIdClaim` holds the client id of a machine client, and is absent from the tokens of persons. See the caution in the next section.

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

The Secret can live in any namespace. If it is not in the namespace of a cluster, the operator copies the key into that namespace as the Secret `<name>-camunda-oidc-client`.

**How a token becomes a person or a client.** `usernameClaim` and `clientIdClaim` tell the cluster who is behind a token. The cluster reads a token in this order:

1. If the token holds the client id claim, the caller is a machine client with that id.
2. If not, and the token holds the username claim, the caller is a person with that username.
3. If the token holds neither claim, the cluster refuses the request.

If `clientIdClaim` is unset, every token becomes a person. The claims only say who the caller is. They grant nothing. The `spec.auth.admin` block of the `CamundaCluster` makes a caller an administrator.

> **Caution:** Do not name a client id claim that the tokens of persons also carry. A token that holds the client id claim always becomes a machine client, even when a person signed in. Keycloak, for example, puts `azp` in every token. If one client serves both the browser login and the machine callers, name a claim that the provider adds to machine tokens only. Camunda gives the same advice.

### Give a cluster an administrator

Nothing authorizes a caller until `spec.auth.admin` on the `CamundaCluster` names one. A cluster without this block has no administrator. The block takes three kinds of member, and all of them become members of the `admin` role:

| Field | Matched against | Needs |
| --- | --- | --- |
| `users` | The value of `usernameClaim` in the token | Nothing |
| `clients` | The value of `clientIdClaim` in the token | `clientIdClaim` set on the platform config |
| `mappingRules` | A claim name and a claim value in the token | Nothing |

A mapping rule is the general form. It gives the role to every token whose claim `claimName` holds `claimValue`, so one rule can cover a whole group from the identity provider.

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

**Per-cluster client.** A cluster can use a client of its own. Set `spec.auth.clientId`, `spec.auth.audience`, and `spec.auth.clientSecretRef` on the `CamundaCluster`. Each field overrides the default of the platform config on its own. When you set `clientId` and not `audience`, the audience becomes the new client id. The issuer, the endpoints, and the claim names always come from the platform config.

A `CamundaClusterPreset` can carry the same `spec.auth` fields as a baseline for many clusters. The cluster overrides the client fields of the preset one by one. The `admin` block never merges: when the cluster sets `spec.auth.admin`, it replaces the whole block of the preset.

Under basic authentication the cluster ignores `spec.auth`. It does not reject it.

### Connectors

The connectors runtime authenticates against the cluster with the OIDC client of the cluster. It needs the `connectors` role. The operator gives that role to the client when both conditions hold: connectors are enabled on the cluster, and the platform config sets `clientIdClaim`. Without a client id claim the runtime becomes a person, and a client member never matches it. In that case, give the role to the runtime yourself in the Admin web application.

## Where settings live

| Setting | Kind and field | Which wins |
| --- | --- | --- |
| Authentication method | `CamundaPlatformConfig` `spec.auth.method` | Only the platform config sets it |
| Issuer URL and explicit endpoints | `CamundaPlatformConfig` `spec.auth.oidc.issuerUrl`, `jwksUrl`, `tokenUrl`, `authUrl` | Only the platform config sets them |
| `usernameClaim`, `clientIdClaim` | `CamundaPlatformConfig` `spec.auth.oidc` | Only the platform config sets them |
| Client id, audience, client secret | `CamundaPlatformConfig` `spec.auth.oidc`, then `CamundaClusterPreset` `spec.cluster.auth`, then `CamundaCluster` `spec.auth` | The cluster, then the preset, then the platform config, field by field |
| Administrators | `CamundaClusterPreset` `spec.cluster.auth.admin`, then `CamundaCluster` `spec.auth.admin` | The cluster replaces the whole block of the preset |
| Redirect URI | `CamundaCluster` `spec.externalUrl` | Only the cluster sets it |
| Basic-auth admin password | Secret `<name>-camunda-admin` | The operator generates it |

## Related

- [CamundaPlatformConfig](../crds/camundaplatformconfig.md): the authentication method and the identity provider connection.
- [CamundaCluster](../crds/camundacluster.md): the per-cluster client credentials, the administrators, and `externalUrl`.
- [CamundaClusterPreset](../crds/camundaclusterpreset.md): the baseline between the platform config and the cluster, and the merge rules.
