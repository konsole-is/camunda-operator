# Management planes

A management plane runs Management Identity, Console, and Web Modeler for the clusters of one or more environments. The fleet manager creates a plane as a [CamundaManagementClusterClaim](../crds/camundamanagementclusterclaim.md). The operator then creates the plane from the claim.

## Create a management plane

Open the page **Management planes** at `/fleet/planes` and select **New management plane**. The form has five steps. **Next** stays off while a step has an error, and the errors show under the step. You can go back to an earlier step at any time. The fleet manager keeps what you entered.

The form follows the cluster while you fill it in. An Environment, a namespace, or a host that changes shows in the errors of its step at once.

### Basics

Pick the environment of the plane, a name, and a namespace. The name is the name of the claim. The namespace can be a namespace of the environment, or a new one when the environment allows **Create a namespace**.

**Platform config** shows the platform config of the environment. You can pick another one.

**Where users sign in** has three options:

- **Keycloak, run by this plane**: the plane runs Keycloak through the Keycloak Operator.
- **External Keycloak**: the plane uses a Keycloak that you run. Enter its URL, the realm, and the Secret with the administrator credentials.
- **OIDC from the platform config**: users sign in at the OIDC provider of the platform config. This option is off while the platform config uses basic authentication.

### Environments served

The plane serves the clusters of the environments that you select. By default, it serves the environment of the plane. **Cluster labels** narrow the clusters. Leave them empty to serve every cluster in these environments. **Expected clusters** shows the result that each cluster can expect.

### Components and versions

Management Identity always runs. Enter its first administrator:

- With Keycloak, enter a **Username** and an **Email**. **Password Secret** is optional. Without it, the operator generates a password.
- With OIDC, enter the **Claim name** and the **Claim value** of the token claim that identifies the administrator. An example is `sub` with the ID of the user. The claim name cannot hold `=`.

Management Identity reads the administrator once, at its first start.

Select **Run Console** and **Run Web Modeler** to run them. Web Modeler sends mail, so it needs an SMTP server: the host, the port (587 by default), the address that it sends from, and STARTTLS (on by default). **Credentials Secret** names a Secret with the keys `username` and `password`. With Keycloak, Web Modeler also needs the **Email** of the administrator.

The step shows one version for each component that runs. The fleet manager fills them from its images table, for the Camunda minor of the default release of the first environment that the plane serves. The note under the versions says `Tested with release 8.9`. The table gives each component its own version line, so Management Identity can be `8.9.12` while Console is `8.9.130`. Web Modeler takes the version of its `restapi` image. You can type another version. A version that you typed stays when the environments change.

The step refuses a version that the operator refuses:

| Component | Accepted versions |
| --- | --- |
| Management Identity, Console, Web Modeler | `8.9.0` and later |
| Keycloak | From `26.0.0`, below `27.0.0` |

With Management Identity 8.9 and Keycloak `26.7.0` or later, the step shows a warning and still lets you continue. Management Identity 8.9 cannot create its realm in that Keycloak. [The operator runs Keycloak](../crds/camundamanagementcluster.md#the-operator-runs-keycloak) gives the reason.

When the images table has no versions for the minor, the note says so. Enter each version then.

### Storage and access

**Databases** shows the database options that the policy of the environment allows. The option with the **Default** label is the default of the policy:

- **New PostgreSQL server** creates one `DatabaseServer` `<plane>-db` and one `Database` for each component that runs, for example `my-plane-keycloak` and `my-plane-identity`.
- **Existing databases** uses `DatabaseConfig` objects of the namespace of the plane. Pick one for each component that runs. Two components cannot use the same `DatabaseConfig`. A new namespace has no `DatabaseConfig`, so this option is off for it.

**Access** shows the access options that the policy allows, and **No access from outside**.

With an access option, the plane gets one host. The pattern of the option makes the host from the name and the namespace of the plane, for example `{name}.{namespace}.example.com`. **Hostname** shows the host and its pattern. You cannot change the host here. Each component serves over `https` under a fixed path on that host:

| Component | Path |
| --- | --- |
| Keycloak | `/auth` |
| Management Identity | `/identity` |
| Console | `/console` |
| Web Modeler | `/modeler` |
| Web Modeler websockets | `/modeler-ws` |

If another cluster or plane uses the same host, the step shows `The host <host> is taken by <namespace>/<claim>. Choose another name.` Go back to **Basics** and pick another name for the plane.

With **No access from outside**, enter the URL where browsers reach each component that runs. Each URL must start with `http://` or `https://`. The Keycloak URL must have the path `/auth`, and no query and no fragment. A Management Identity URL with a path must use `https`. A URL must not hold a `%`, a `\`, or a `.` or `..` path segment.

**Contract name** is the name of the [ManagementAuthConfig](../crds/managementauthconfig.md) that the plane writes for Optimize. Leave it empty to use the name of the plane. The step refuses a name that a `ManagementAuthConfig` or another plane already uses.

### Review

The review shows the claim as YAML and a list of what it creates:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: CamundaManagementClusterClaim
metadata:
  name: my-plane
  namespace: my-plane-ns
spec:
  clusterSelector: {}
  environments:
    - production
  management:
    console:
      externalUrl: https://plane.example.com/console
    identity:
      admin:
        email: admin@example.com
        username: admin
      externalUrl: https://plane.example.com/identity
    identityProvider:
      keycloak:
        externalUrl: https://plane.example.com/auth
  plugins:
    - parameters:
        identity: my-identity-db
        keycloak: my-keycloak-db
      registration: existing-database-configs
      section: management-database
  versions:
    console: 8.9.130
    identity: 8.9.12
    keycloak: 26.6.4
```

The claim names `platformConfigRef` only when you picked a platform config that is not the default of the environment. It names `managementAuthConfigName` only when the contract name is not the name of the plane.

Select **Create** to write the claim. In Git mode, the button reads **Push to Git**. [Fleet manager settings](settings.md) says how to turn on Git mode. The fleet manager commits the claim to the repository, and the new namespace too when the plane needs one, in one commit.

Another claim with the same name can appear while the review is open. The fleet manager then writes nothing and shows `The CamundaManagementClusterClaim <name> exists already. Pick another name.` Go back to **Basics** and pick another name.

Without Git mode, the fleet manager creates a new namespace before the claim. If the namespace is created and the claim is not, the namespace stays. The form then uses that namespace as an existing namespace and says so. Correct the cause, and select **Create** again.

### Progress

After **Create**, the page shows the progress of the plane, one row for each part, in the order in which the operator creates them:

| Row | Done when |
| --- | --- |
| Environment and policy | The claim records its environment and passes the policy. |
| Databases | The database option reports its outputs and is ready. |
| Access | The access option reports its outputs and is ready. The row shows only with an access option. |
| Management cluster | The `CamundaManagementCluster` is ready. |

In Git mode, the rows stay pending and the page shows `Waiting for GitOps to apply` until your GitOps tool applies the commit.

When the operator refuses the claim, the page shows the reason and the message of the claim:

```yaml
status:
  environment: production
  conditions:
    - type: Ready
      status: "False"
      reason: MissingInput
      message: without an access selection, the claim must set spec.management.console.externalUrl
      observedGeneration: 1
```

In this example, the plane has no access option and the claim has no Console URL. The [Status](../crds/camundamanagementclusterclaim.md#status) section of the claim lists each reason and what to do.
