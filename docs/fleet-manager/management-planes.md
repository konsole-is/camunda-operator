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

The list shows `Could not read <kind>:` and the error for a list that it cannot read. For a list that is not current, it shows `Not known: the fleet manager has no current list of <kind>.` In both cases, the list does not say that no plane exists.

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

When the fleet manager cannot read the `CamundaManagementCluster` or the claim, the page shows `Could not read <kind>:` and the error. The page then does not say that the plane is missing or deleted. The same applies while the copy of one of them is not current, for example the copy of an earlier visit. The page then shows `Not known: the fleet manager has no current list of <kind>.` until it gets a current list.

While the copy of the `CamundaManagementCluster` on the page is not current, the header and the dot of each card show `Not known`. While the page cannot read the pods, a card shows `Could not read Pod`. In the `oidc` mode, the issuer shows `Could not read CamundaPlatformConfig` while the page cannot read the platform config.

While the page cannot read the `CamundaManagementCluster` or the claim, **Suspend** and **Delete** are disabled, and an open confirm closes. The same applies while the copy of one of them is not current. A failed read of the pods or the platform config changes only the cards, so **Suspend** and **Delete** stay available.

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

A tab needs some lists. When the page fails to read such a list, or its copy of the list is not current, the tab names the list. It shows `The fleet manager has no current list of <kind>. This tab can be incomplete or out of date.` The tab does not say that a list is empty or that the contract does not exist. While the page cannot read the namespaces or the environments, the **Clusters served** tab shows `Not known` as the environment of a cluster.

## Change a management plane

The **Claim** tab of the plane page shows the claim of the plane, one section at a time. Select **Edit** on a section to change it. The fleet manager changes only the claim. The operator then changes the plane. When the fleet manager cannot read its settings, the sections show no **Edit**. A plane that no claim made shows no claim on this tab. While the page cannot read the plane or its claim, the tab offers no **Edit** and names the kind that it did not read.

### What stays fixed

These values do not change after the plane is created:

| Value | The tab shows |
| --- | --- |
| Name and namespace | `Set at creation` |
| The option of **Databases** and of **Access** | `Chosen at creation`. A parameter of the option stays open when the policy of the environment lets the claim set it. |
| The claim name and the claim value of the first administrator, in OIDC mode, after Management Identity started | `Fixed after Identity started`. Management Identity keeps the first administrator in its database. |

The **Access** section also shows the host of the plane and the URL of each app. A plane with **No access from outside** shows the URL of each app as a field that you can change.

### Change the platform config

**Basics** has a list of the platform configs. The first entry, **From the environment**, removes the platform config from the claim, so that the plane takes the platform config of its environment. Management Identity, Console, and Web Modeler restart on a move. OIDC sign-in needs a platform config with OIDC authentication. While the plane signs in with OIDC, the tab refuses a platform config with basic authentication. A move of the platform config in OIDC mode can change the identity provider. Then add the callback of each Optimize to its application. The **Optimize** tab lists the callbacks.

### Change where users sign in

**Identity** has the three options of [Basics](#basics). When you select an option, the claim moves to it in one change. A move between OIDC and a Keycloak option asks for the first administrator of the new option:

- After a move to OIDC, enter the **Claim name** and the **Claim value**. The move removes the username and the **Password Secret** from the claim.
- After a move to a Keycloak option, enter the **Username**, and the **Email** when Web Modeler runs. The move removes the claim name and the claim value.

The first administrator stays. Management Identity keeps the one in its database. A new username in a Keycloak option creates a second user. The first user keeps its access.

A move of the sign-in shows where the login callbacks of Optimize go. [Change the identity provider](../crds/camundamanagementcluster.md#change-the-identity-provider) explains the move of the callbacks. After a move to OIDC, add the callback of each Optimize to the application at your identity provider. The **Optimize** tab lists the callbacks.

This claim moves a plane from OIDC to an External Keycloak:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: CamundaManagementClusterClaim
metadata:
  name: my-plane
  namespace: my-plane-ns
spec:
  management:
    identityProvider:
      externalKeycloak:
        url: https://keycloak.example.com/auth
        adminCredentialsSecretRef:
          name: my-keycloak-admin
    identity:
      admin:
        username: admin
        email: admin@example.com
  # ... the rest of your claim
```

### Change the clusters served

**Environments served** has one checkbox for each environment, and the **Cluster labels** of the cluster selector. Match expressions of the selector show as text. Change them in the claim YAML. A change of the labels keeps the match expressions. A selector without labels serves every cluster in the environments. A claim without a cluster selector serves no cluster.

Under the fields, the section lists each cluster that the change attaches and each that it detaches. A cluster that attaches while the plane runs Console gets the Console settings and restarts once. A cluster that detaches while the plane runs Console loses its Console settings and restarts once. A cluster with basic authentication that detaches while the plane runs Web Modeler loses the Web Modeler user of the plane. **Expected clusters** shows the result that the page expects for each cluster. The operator decides the real result when the plane runs.

In OIDC mode, the list needs the issuer URL of the platform config. Without it, the section says that the preview needs the issuer and lists no cluster.

### Turn Console and Web Modeler on or off

**Components** has **Run Console** and **Run Web Modeler**. When Console stops, the served clusters lose their Console settings and restart once. When Web Modeler stops, its database stays, and each served cluster with basic authentication loses the Web Modeler user of the plane. An app that you turn off loses its version in the claim. An app that you turn on needs a version in **Versions**.

### Change the versions

**Versions** has one version for each app that runs. The section refuses the versions that [Components and versions](#components-and-versions) refuses, and the error blocks **Review**.

A warning does not block **Review**. The section warns in these cases:

- A version of Management Identity, Console, or Web Modeler goes down. Camunda documents no downgrade.
- A version skips a minor, for example from 8.9 to 8.11. Move one minor at a time.
- Management Identity, Console, and Web Modeler run on different minors. The [component version matrix](https://docs.camunda.io/docs/reference/supported-environments/#component-version-matrix) of Camunda lists the versions that work together. The tab compares the minor only, because each app has its own patch line.
- The Keycloak version changes. Sign-in stops while Keycloak restarts.

### Review and save the change

The bar at the bottom of the page shows the number of unsaved changes, with **Discard** and **Review**. **Review** shows the change of the claim and what it does:

- The move of the platform config and of the sign-in, and where the callbacks go.
- Each cluster that attaches or detaches.
- What Console off and Web Modeler off do, and each version move with its warnings.
- The apps that restart, and the Optimize of each served cluster when its sign-in settings change.
- The apps that restart only in some cases, as `may restart` with the case. For example, a change of a database parameter can restart the apps. The database option decides.

A change that moves where users and the served clusters sign in needs the name of the plane typed before **Save** turns on. These changes are:

- A move of the sign-in.
- Another URL or realm of the External Keycloak.
- Another platform config in OIDC mode.
- A change of the access settings while the plane runs its own Keycloak (**Keycloak, run by this plane**).

In Git mode, the button is **Push to Git**. [Fleet manager settings](settings.md) says how to turn on Git mode.

The save carries the version of the claim that the page read. When the claim changed since then, the save writes nothing and says so. Review the change and save it again.

While the page cannot read a list that the review needs, or the list is not current, **Review** is off. The bar names the list and says that what the change does is not known. An open review closes.

### A claim that another tool applied

Each tool that writes the claim owns the fields that it set, for example `kubectl` or a GitOps tool.

In direct mode, the review names each field that another tool set and that the save changes or removes, with the tool. It asks you to take these fields over:

- Without the choice, a new value of such a field stops the save, and **Save** stays off. A removed field or list item stays in the claim. When the change removes nothing else, the review says that the save changes nothing.
- With the choice, the save gives each changed field its new value and removes each removed field. The impacts in the review show what the save does with the choice.

[Fields that another tool set](settings.md#fields-that-another-tool-set) says what a take-over does, and what `kubectl apply` does after it.

A move of the identity mode cannot save when another tool set the identity provider. The save gives the new values first and removes the cleared fields after that. Between the two writes, the claim has two identity providers, and Kubernetes refuses such a claim. The review shows `exactly one identity provider: keycloak, externalKeycloak, or oidc`, and **Save** stays off. Make this move in the claim file, and apply the file again.

In Git mode, your GitOps tool applies the claim file:

- The review checks the change as your GitOps tool, which takes over each field of the file. So a new value does not fail.
- A removed field or list item that another tool also set stays in the claim. A field that the fleet manager saved in direct mode stays too, until a save in direct mode removes it.
- Before your GitOps tool applied the claim with server-side apply, the review lists each removed field that a tool set. The first sync keeps these fields if it applies server side. If it applies client side, it removes a field of the last applied file.

In Git mode, to change or remove a field that another tool set, make the change with that tool.

A tool that writes without server-side apply, such as `kubectl apply` without `--server-side` or `kubectl edit`, does not see a save of the fleet manager. Its next write can set a field back, and nothing warns about it.

You can apply the claim of a plane with `kubectl apply` or another tool. In direct mode, the review then asks before a save changes or removes a field that the tool set.

## Suspend a management plane

Suspend stops the plane and keeps its data. The header of a plane that a claim made has the button **Suspend**.

1. Select **Suspend**. The page asks you to confirm. It says that every workload of the plane stops. Sign-in to Console, Web Modeler, and Optimize then stops for every cluster that the plane serves. It also says that the served clusters keep running, and that the databases keep the data of the plane. In the `keycloak` mode, it warns about each cluster that uses the Keycloak of the plane as OIDC issuer. The users and API clients of such a cluster cannot sign in or get tokens until the plane resumes. The page says this also while no cluster is attached.
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

When every workload is at zero, the header shows `Ready · Suspended`. Select **Resume** to start the plane again. Resume sets `spec.suspend: false` and asks no question, unless another tool set `spec.suspend`. The [Suspension](../crds/camundamanagementcluster.md#suspension) section of the `CamundaManagementCluster` lists what stops and what stays.

In direct mode, another tool can own `spec.suspend` of the claim, for example `kubectl apply` of a claim file that sets `suspend`. Then **Suspend** or **Resume** shows the field and the tool, and asks to take it over. **Suspend the management plane** or **Resume the management plane** stays off until you take it over. The next **Suspend** or **Resume** asks again. When the claim changes before you confirm, the question closes, and the page shows `The plane changed. Select Suspend again.` or the same line for **Resume**. [Fields that another tool set](settings.md#fields-that-another-tool-set) says what a take-over does.

The page writes the claim only if nobody changed the claim after the page read it. If somebody changed it, the page shows the error and writes nothing. In Git mode, the header shows `1 change waiting for GitOps` until GitOps applies the commit. **Suspend** and **Resume** are disabled until then. After you push a delete of the plane, the browser tab that pushed it shows `1 delete waiting for GitOps`, and **Suspend**, **Resume**, and **Delete** stay disabled there. In another tab, or after a reload, the fleet manager refuses a save of the claim until GitOps applies the delete. The page then shows that refusal.

While the fleet manager cannot read its settings, **Suspend** and **Resume** are disabled, and the header says `Changes are off: the fleet manager could not read its settings.`

A plane that no claim made has no Suspend and no Delete. While the claim is being deleted, **Suspend** and **Resume** are off.

## Delete a management plane

Select **Delete** in the header of the plane. The delete page opens at `/fleet/planes/<namespace>/<name>/delete`.

1. Choose what happens to the resources of the database and access options. **Delete the resources** and **Keep the resources** set `spec.deletionPolicy` of the claim to `Delete` or `Orphan`. The page selects the policy of the claim. Without one, it selects the policy that the claim recorded from its environment, else `Delete`. When an option always deletes what it made, **Keep the resources** is off.
2. Read what the delete takes. The page lists:
    - The objects that always go: the `CamundaManagementCluster`, with its workloads, Services, and generated Secrets, the requests of the options, and the claim.
    - The clusters that the plane serves. They stay and keep running. The plane removes its Console settings and its claim from each cluster, and each cluster restarts its pods once. A cluster with basic authentication also loses its `web-modeler` user. Sign-in to Console, Web Modeler, and Optimize stops for these clusters. In the `keycloak` mode, each cluster that uses the Keycloak of the plane as its OIDC issuer loses sign-in. Its users and API clients can no longer sign in or get tokens. The page says this also while no cluster is attached. When the claim did not make the plane, the page says that the delete does not touch the plane or the clusters that it serves.
    - The contract of the plane, which goes. Each `CamundaOptimize` that reads it then reports `InvalidReference`. The page names each of them.
    - The resources of each option, with `Deleted` or `Kept` for each.
    - With **Delete the resources**: `Deleting a Database keeps its data on its server. Deleting a DatabaseServer erases the server and its data.`
    - With **Delete the resources**, in the `keycloak` mode and while the plane serves clusters, the review also says: `When a DatabaseServer of an option holds the Keycloak database, the delete erases the users and clients of the realm.`
    - With **Keep the resources**, a resource that shows `Kept` stays after the plane is gone. An option can still delete some of its resources, for example a route. Those show `Deleted`.
3. Type the name of the plane.
4. Select **Delete the management plane**.

The page first writes your choice to the claim when the claim has another deletion policy, or none of its own and none recorded. The page then shows "Waiting for the operator to read the deletion policy." In direct mode, the fleet manager then deletes the claim. The operator gives your choice to each option before it removes anything. The page shows each object that goes until it is gone. The [Deletion](../crds/camundamanagementclusterclaim.md#deletion) section of the claim explains the order.

If the spec of the claim changed after the page read it, the delete fails. The page then shows the claim again for a new review. A change of the claim status does not stop the delete. The operator records the deletion policy in the claim status. If the claim then lost the finalizer `fleet.camunda.io/cleanup`, it cannot give that policy to its options. Its delete fails until you add the finalizer back.

In Git mode, the confirm is **Push the delete to Git**. A changed policy and the delete are two commits. The page pushes the delete only after GitOps applied the policy and each option has it. While the claim reports an error, the operator does not update the options. The page then names the options and gives the reason, and the fleet manager refuses the push. The fleet manager removes the claim file, so your GitOps tool must prune to delete the claim. [Delete in Git mode](cluster-page.md#delete-in-git-mode) on the cluster page shows the same steps with examples.

When the page cannot read a list that the delete needs, it shows `Not known` for each fact that comes from that list. It then shows no confirm. It shows `Delete is possible when the fleet manager can read <kind>.`
