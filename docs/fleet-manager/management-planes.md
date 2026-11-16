# Management planes

A management plane runs Management Identity, Console, and Web Modeler for the clusters of one or more environments. The fleet manager creates a plane as a [CamundaManagementClusterClaim](../crds/camundamanagementclusterclaim.md). The operator then creates the plane from the claim.

## Create a management plane

Open the page **Management planes** at `/fleet/planes` and select **New management plane**. The form has five steps. **Next** stays off while this step or an earlier step has an error. The errors show at the bottom of the form, each with the name of its step. You can go back to an earlier step at any time. The fleet manager keeps what you entered.

The form follows the cluster while you fill it in. An Environment, a namespace, or a host that changes shows in the errors of its step at once. A pick that goes away after you made it shows the same way. An example is a deleted `DatabaseConfig`, or a preset that the environment no longer lists. While a step has an error, **Create** stays off. The review, the last step, lists that error at the bottom of the form in the same way.

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
- **Existing databases** uses `DatabaseConfig` objects of the namespace of the plane. Pick one for each component that runs. Two components that run cannot use the same `DatabaseConfig`. A new namespace has no `DatabaseConfig`, so this option is off for it.

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

**When the management plane is deleted** says what happens to the resources of the options that you pick in this step. **Keep the resources** is the deletion policy `Orphan`, and **Delete the resources** is `Delete`. The **Default** label marks the default of the environment, see [When a claim is deleted](environments.md#when-a-claim-is-deleted). Some options always delete what they made. When you pick one, it shows under the choice with **Always deleted**, and **Keep the resources** is off. If the default keeps the resources and you pick such an option, the plane gets `Delete`, and a line under the choice says so. This is no error, so you can still create the plane. The `CamundaManagementCluster` is always deleted with the claim.

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

The claim names `platformConfigRef` only when you picked a platform config that is not the default of the environment. It names `managementAuthConfigName` only when the contract name is not the name of the plane. It names `deletionPolicy` only when the deletion policy of the plane is not the default of the environment.

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

## Read the state of a management plane

Select the name of a plane in the list at `/fleet/planes`. The page of the plane opens at `/fleet/planes/<namespace>/<name>`.

The header shows the name of the plane and its `Ready` condition. A reason other than `Healthy` shows next to `Ready`, for example `Ready · Suspended`. Under the name, the header shows these facts:

| Fact | What it shows |
| --- | --- |
| Identity provider | `Keycloak`, `External Keycloak`, or `OIDC` |
| Namespace | The namespace of the plane |
| Platform config | The platform config of the plane, or of its environment when the claim names none |
| Serves | The environments of the claim. A plane that no claim made shows its namespace selector, or `Every namespace`. A plane that no claim made and that has no `spec.clusterSelector` serves no cluster, and shows `No cluster, because it has no cluster selector`. |

### Components

The **Components** section of the **Overview** tab shows a card for each component that runs on the plane:

| Card | When it shows |
| --- | --- |
| Keycloak | In the `keycloak` mode |
| Identity provider | In the `externalKeycloak` and `oidc` modes |
| Management Identity | Always |
| Console | When the plane runs Console |
| Web Modeler REST API, Web Modeler websockets | When the plane runs Web Modeler |

The dot of a card shows the condition of its component: `KeycloakReady`, `IdentityReady`, `ConsoleReady`, or `WebModelerReady`. The two Web Modeler cards both show `WebModelerReady`. For example, `IdentityReady` with the status `False` turns the dot of **Management Identity** red. A card also shows how many of its pods are ready, and the highest CPU or memory use of one pod against its limit.

The **Identity provider** card shows the mode and the issuer URL. In the `externalKeycloak` mode, the issuer is the URL of your Keycloak followed by `/realms/<realm>`. In the `oidc` mode, the issuer comes from the platform config. Your identity provider runs outside the plane, so this card shows no pods and opens no detail.

Select another card to open the detail of its component. The detail shows the pods of the component, its condition, and the events of the `CamundaManagementCluster`. **All components** goes back to the cards. The **Settings** tab of the detail shows the settings of the claim for that component, with the reason when you cannot change one. The tab shows the settings and does not change them:

| Setting | What the tab shows |
| --- | --- |
| Administrator claim name and value, in the `oidc` mode | `Fixed after Identity started`, once Management Identity recorded the first administrator. You cannot change them after that. See [The first administrator](../crds/camundamanagementcluster.md#the-first-administrator). |
| Administrator username, in a Keycloak mode | `A new username creates a second user.` |
| Web Modeler mail | No reason, because you can change it |

A plane that the claim controller has not created yet shows no cards.

### Conditions

The **Conditions** section lists the conditions of the `CamundaManagementCluster` that belong to no component: `SecretsReady`, `MirroredSecretsReady`, `ServiceAccountsReady`, `ManagementAuthReady`, and `OptimizeCallbacksReady`. The group **Claim** follows, with `RequestsReady` and `ManagementClusterReady` of the claim. Each row shows the status, the reason, the message, and the time since the last change.

`KeycloakReady`, `IdentityReady`, `ConsoleReady`, and `WebModelerReady` belong to a component, so they do not show in this section. `Ready` shows in the header.

In OIDC mode, three conditions read `True` with the reason `Disabled`. The plane runs no Keycloak, so `KeycloakReady` is `Disabled`. Your provider issues every client secret, so `SecretsReady` is `Disabled`. Your provider also holds the login callbacks of Optimize, so `OptimizeCallbacksReady` is `Disabled`:

```yaml
status:
  conditions:
    - type: SecretsReady
      status: "True"
      reason: Disabled
      message: Component is disabled.
    - type: KeycloakReady
      status: "True"
      reason: Disabled
      message: Component is disabled.
    - type: ManagementAuthReady
      status: "True"
      reason: Healthy
      message: ManagementAuthConfig "plane-a" is up to date
    - type: OptimizeCallbacksReady
      status: "True"
      reason: Disabled
      message: The identity provider of the platform config holds the callback URLs of Optimize
```

A condition with the reason `Disabled` does not hold `Ready` back. The [Status](../crds/camundamanagementcluster.md#status) section of the `CamundaManagementCluster` lists each reason and what to do.

### A plane that does not exist yet

After you create a plane, its claim can exist before the `CamundaManagementCluster`. The page then shows the `Ready` condition of the claim in the header, and the line `The claim controller has not created the management plane yet.` The **Conditions** section shows only the group **Claim**. The page shows the plane when the `CamundaManagementCluster` appears. If the plane does not appear, read the reason and the message of `RequestsReady` and `ManagementClusterReady`. The [Status](../crds/camundamanagementclusterclaim.md#status) section of the claim lists each reason and what to do.

When the plane and its claim are both deleted, the page shows `This management plane no longer exists.` and a link to all management planes.

### A plane that the page could not read

When the fleet manager cannot read the `CamundaManagementCluster` or the claim, the page shows `Could not read <kind>:` and the error. The page then does not say that the plane is missing or deleted.

While the copy of the `CamundaManagementCluster` on the page is not current, the header and the dot of each card show `Not known`. While the page cannot read the pods, a card shows `Could not read Pod`. In the `oidc` mode, the issuer shows `Could not read CamundaPlatformConfig` while the page cannot read the platform config. The buttons **Suspend** and **Delete** are disabled, and an open confirm closes.

When the page cannot read the `CamundaManagementCluster`, it does not know if the plane is managed or which claim made it. The **Conditions** section shows the conditions of a claim with the same name in the group **Claim**. The line `The page could not read the management plane, so it cannot tell if the claim <name> made it.` stands above that section. The header shows `Not known` in place of the `Ready` condition. It shows a dash for the identity provider, the platform config, and **Serves**, because the page does not know them. The page also does not show the `Not managed by the fleet manager.` banner.

### A plane that no claim made

A `CamundaManagementCluster` without the label `fleet.camunda.io/claim` is not managed by the fleet manager. The page shows `Not managed by the fleet manager. You can read this management plane but not change it here.` The **Conditions** section has no group **Claim**. A claim with the same name in the same namespace does not change this plane, so the page does not show that claim. To change such a plane, change the `CamundaManagementCluster` where you manage it, for example in your GitOps repository.

### Clusters served

The **Clusters served** tab lists each orchestration cluster that the plane selects. Each row shows the cluster, its environment, its namespace, whether the plane serves it, and the reason and the message. Select the name of a cluster to open its page.

Filter the rows by **Environment**, and by **Attached** with the values `Attached` and `Not attached`. A cluster whose namespace belongs to no environment shows `No environment`. The search box finds the name, the namespace, the reason, and the message.

A row with **Attached** `No` gives the reason why the plane does not serve the cluster. The reason `BasicAuthUserFailed` is on a row with **Attached** `Yes`. The plane serves that cluster, but the Web Modeler user on it is missing. The [Clusters](../crds/camundamanagementcluster.md#clusters) section of the `CamundaManagementCluster` lists each reason and what to do.

The tab shows `This plane lists no clusters yet.` when the cluster selector of the plane matches no cluster, or when the plane has no cluster selector.

### Optimize

The **Optimize** tab lists each Optimize that signs in through this plane, with its external URL and its callback URL. The callback URL is the external URL followed by `/api/authentication/callback`. For example, an Optimize at `https://orders.example.com/optimize` has the callback URL `https://orders.example.com/optimize/api/authentication/callback`. **Callbacks** above the table shows the state of `OptimizeCallbacksReady` once for the plane. A row has no state of its own.

In the two Keycloak modes, the plane registers each callback URL in the realm. In OIDC mode, the plane registers nothing. The tab then shows `Add the callbacks to your identity provider.` Add each callback URL of the table to the Optimize application of your identity provider. The [Optimize](../crds/camundamanagementcluster.md#optimize) section of the `CamundaManagementCluster` explains the callbacks.

An Optimize without `spec.externalUrl` gets no callback, so it does not show on this tab. It shows on the **Contract** tab.

### Contract

The **Contract** tab shows the [ManagementAuthConfig](../crds/managementauthconfig.md) that the plane writes and that Optimize reads. It shows the base URL of Management Identity and the issuer URL, also the one for use inside the cluster. It also shows the authorization, token, and JWKS URLs, the client ID, and the audience. When the issuer URL inside the cluster is not set, the tab shows `Not set`, and Optimize uses the issuer URL.

The client secret shows as the namespace, the name, and the key of its Secret. The page does not read the Secret, so it never shows the value.

**Readers** lists each `CamundaOptimize` whose `spec.managementAuthRef` names the contract, with its cluster and its external URL. Before the plane writes its contract, the tab shows `This plane has not written its contract yet.`

### Endpoints

The **Endpoints** tab lists the routes of the plane. Each row shows the name, the path, the Service with its port, the protocol, the public URL, and the app that serves the route. The [Routes](../crds/camundamanagementcluster.md#routes) section of the `CamundaManagementCluster` explains each route.

The public URL comes from the URL of the app on the plane. It is the scheme and the host of that URL, followed by the path of the route. For example, Management Identity at `https://camunda.example.com/identity/` gives the route `identity` the public URL `https://camunda.example.com/identity`. A URL of an app that is not valid shows `Not a valid URL`.

### A tab with a list that the page did not read

A tab needs some lists. When the page fails to read such a list, or its copy of the list is not current, the tab names the list. It shows `The page could not read <kind>, so this tab can be incomplete or out of date.` The tab does not say that a list is empty or that the contract does not exist. While the page cannot read the namespaces or the environments, the **Clusters served** tab shows `Not known` as the environment of a cluster.

## Suspend a management plane

Suspend stops the plane and keeps its data. The header of a plane that a claim made has the button **Suspend**.

1. Select **Suspend**. The page asks you to confirm. It says that every workload of the plane stops. Sign-in to Console, Web Modeler, and Optimize then stops for every cluster that the plane serves. It also says that the served clusters keep running, and that the databases keep the data of the plane. In the `keycloak` mode, it says that the served OIDC clusters sign in at the Keycloak of the plane. Their users and API clients cannot sign in or get tokens until the plane resumes.
2. Select **Suspend the management plane**.

The page sets `spec.suspend: true` on the claim:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: CamundaManagementClusterClaim
metadata:
  name: plane-a
  namespace: platform
spec:
  suspend: true
  # ... the rest of your claim
```

When every workload is at zero, the header shows `Ready · Suspended`. Select **Resume** to start the plane again. Resume sets `spec.suspend: false` and asks no question. The [Suspension](../crds/camundamanagementcluster.md#suspension) section of the `CamundaManagementCluster` lists what stops and what stays.

The page writes the claim only if nobody changed the claim after the page read it. If somebody changed it, the page shows the error and writes nothing. In Git mode, the header shows `1 change waiting for GitOps` until GitOps applies the commit. **Suspend** and **Resume** are disabled until then.

A plane that no claim made has no Suspend and no Delete. While the claim is being deleted, **Suspend** and **Resume** are off.

## Delete a management plane

Select **Delete** in the header of the plane. The delete page opens at `/fleet/planes/<namespace>/<name>/delete`.

1. Choose what happens to the resources of the database and access options. **Delete the resources** and **Keep the resources** set `spec.deletionPolicy` of the claim to `Delete` or `Orphan`. The page selects the policy of the claim. Without one, it selects the policy that the claim recorded from its environment, else `Delete`. When an option always deletes what it made, **Keep the resources** is off.
2. Read what the delete takes. The page lists:
    - The objects that always go: the `CamundaManagementCluster`, with its workloads, Services, and generated Secrets, the requests of the options, and the claim.
    - The clusters that the plane serves. They stay and keep running. The plane removes its Console settings and its claim from each cluster, and each cluster restarts its pods once. A cluster with basic authentication also loses its `web-modeler` user. Sign-in to Console, Web Modeler, and Optimize stops for these clusters. In the `keycloak` mode, a cluster that uses OIDC signs in at the Keycloak of the plane. Its users and API clients then can no longer sign in or get tokens. When the claim did not make the plane, the page says that the delete does not touch the plane or the clusters that it serves.
    - The contract of the plane, which goes. Each `CamundaOptimize` that reads it then reports `InvalidReference`. The page names each of them.
    - The resources of each option, with `Deleted` or `Kept` for each.
    - With **Delete the resources**: `Deleting a Database keeps its data on its server. Deleting a DatabaseServer erases the server and its data.`
    - With **Delete the resources**, in the `keycloak` mode and while the plane serves clusters, the review also says: `When a DatabaseServer of an option holds the Keycloak database, the delete erases the users and clients of the realm.`
    - With **Keep the resources**, a resource that shows `Kept` stays after the plane is gone. An option can still delete some of its resources, for example a route. Those show `Deleted`.
3. Type the name of the plane.
4. Select **Delete the management plane**.

When the policy that you chose differs from the policy of the claim, the page first writes the new policy. It deletes the claim after the claim controller read that policy. The page then shows each object that goes until it is gone. The [Deletion](../crds/camundamanagementclusterclaim.md#deletion) section of the claim explains the order.

In Git mode, the confirm is **Push the delete to Git**. A changed policy and the delete are two commits. The page pushes the delete only after GitOps applied the policy. The fleet manager removes the claim file, so your GitOps tool must prune to delete the claim.

When the page cannot read a list that the delete needs, it shows `Not known` for each fact that comes from that list. It then shows no confirm. It shows `Delete is possible when the fleet manager can read <kind>.`
