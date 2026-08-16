# OIDC admin bootstrap implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a `CamundaCluster` under `method: oidc` grant the admin role through the CRD, and prove the whole OIDC path against Keycloak in kind.

**Architecture:** Two new claim names on `CamundaPlatformConfig.spec.auth.oidc` describe the tokens of the provider. A new `spec.auth.admin` block on `CamundaCluster` lists the members of the admin role, and reaches a preset baseline for free because `CamundaClusterPresetSpec.Cluster` is a `CamundaClusterSpec`. Both render to `camunda.security.*` environment variables in `pkg/components/camundacluster/render.go`. Nothing new is applied to the API server, so no new condition and no new status field appear.

**Tech Stack:** Go 1.26, kubebuilder, controller-runtime, ocf v0.19.1, testify, Ginkgo and Gomega for e2e, kind, Keycloak 26.

**Spec:** `docs/superpowers/specs/2026-08-17-oidc-admin-bootstrap-design.md`

## Global Constraints

- Camunda version floor is 8.9.0. Every configuration key is proven against the camunda/camunda checkout at tag 8.9.9.
- Every configuration key the operator sets is declared in `pkg/camundaconfig/keys.go` with a class-name GoDoc, and `TestRenderOnlyDeclaredKeys` fails otherwise.
- Every managed resource is applied with server-side apply. Status is written once per reconcile through ocf `FlushStatus`.
- Prose in `api/v1` GoDoc, `docs/`, and CRD field descriptions follows `simple-english:simple-english`.
- Go code follows `how-we-write-go`: exported identifiers carry a GoDoc that is not a restatement of the name, one argument per line when a call is split, no `t.Fatal` in tests.
- Commits carry the sub-issue reference as a subject suffix: `(#73)` or `(#61)`.
- Branch `feat/oidc-admin-bootstrap` off `origin/main`. Sub-PRs target that branch, and the integration PR targets `main`.

---

## Contracts

The two PRs are sequential: PR 2 cannot pass until PR 1 is merged into the feature branch. There is no parallelism to buy, so there is one contract, and it exists only so PR 2 can be written without re-deriving PR 1.

| Name | Producer (issue) | Consumer (issue) | Shape | Realization |
| --- | --- | --- | --- | --- |
| `oidc-admin-crd-surface` | #73 | #61 | `CamundaPlatformConfig.spec.auth.oidc.{usernameClaim,clientIdClaim}` (both `string`, optional). `CamundaCluster.spec.auth.admin.{users,clients}` (both `[]string`), `.mappingRules[].{id,claimName,claimValue}` (all `string`, all required). | sequential — PR 2 branches from the post-merge feature branch |

## Conventions

- **Layout.** CRD types in `api/v1/<crd>_types.go`. Pure render and merge logic in `pkg/components/camundacluster/`. No new file: the admin bootstrap belongs in `render.go` next to `oidcEnv`, and the merge clause in `presetmerge.go` next to `mergeAuth`.
- **Naming.** `ClusterAdminSpec` and `AdminMappingRule` in `api/v1`, matching the existing `ClusterAuthSpec`. Render helpers are unexported and named for what they emit: `adminRoleEnv`, `mappingRuleEnv`, `connectorsRoleEnv`. No name carries `OIDC2`, `New`, or a phase label.
- **Vocabulary.** The block is "admin bootstrap". Its entries are "members". The Camunda side is "the admin role". Never "admin user" for a client, because Camunda's `users` member type means something narrower.
- **Keys.** One `camundaconfig.Key` constant per property, plus one element constant per list, matching the existing `KeyDefaultRolesAdminUsers` and `KeyDefaultRolesAdminUserItem` pair.
- **Tests.** Render assertions use the existing `assertEnv`, `assertNoEnv`, and `assertSecretEnv` helpers on `render(in, process(t, in, ComponentGateway))`. New behavior gets its own `Test...` function, never an extra assertion bolted onto an unrelated one.
- **e2e.** The new flow follows `camundacluster_test.go`: package-level constants at the top, one `Describe(..., Ordered)`, `dumpDiagnostics` in `AfterEach`, resource names from `components.*` helpers and never string literals.

---

# PR 1 — Operator: grant the admin role through the CRD (#73)

Branch `feat/oidc-admin-bootstrap--admin-config` off `feat/oidc-admin-bootstrap`.

### Task 1: Declare the configuration keys

**Files:**
- Modify: `pkg/camundaconfig/keys.go`
- Modify: `pkg/camundaconfig/keys_source_test.go:41-80`

**Interfaces:**
- Consumes: nothing.
- Produces: `KeyOIDCUsernameClaim`, `KeyOIDCClientIDClaim`, `KeyInitializationMappingRules`, `KeyInitializationMappingRuleID`, `KeyInitializationMappingRuleClaimName`, `KeyInitializationMappingRuleClaimValue`, `KeyDefaultRolesAdminClients`, `KeyDefaultRolesAdminClientItem`, `KeyDefaultRolesAdminMappingRules`, `KeyDefaultRolesAdminMappingRuleItem`, `KeyDefaultRolesConnectorsClients`, `KeyDefaultRolesConnectorsClientItem` — all of type `camundaconfig.Key`.

- [ ] **Step 1: Add the two claim keys next to the existing OIDC keys**

Insert after `KeyOIDCAudiences` (`keys.go:126`):

```go
	// KeyOIDCUsernameClaim is camunda.security.authentication.oidc.username-claim
	// (OidcAuthenticationConfiguration.java).
	KeyOIDCUsernameClaim Key = "camunda.security.authentication.oidc.username-claim"
	// KeyOIDCClientIDClaim is camunda.security.authentication.oidc.client-id-claim
	// (OidcAuthenticationConfiguration.java).
	KeyOIDCClientIDClaim Key = "camunda.security.authentication.oidc.client-id-claim"
```

- [ ] **Step 2: Add the mapping rule and role membership keys**

Insert after `KeyDefaultRolesAdminUserItem` (`keys.go:147`):

```go
	// KeyInitializationMappingRules is camunda.security.initialization.mapping-rules
	// (InitializationConfiguration.java).
	KeyInitializationMappingRules Key = "camunda.security.initialization.mapping-rules"
	// KeyInitializationMappingRuleID is the mapping rule id of one list element
	// of KeyInitializationMappingRules (ConfiguredMappingRule.java).
	KeyInitializationMappingRuleID Key = "camunda.security.initialization.mapping-rules[N].mapping-rule-id"
	// KeyInitializationMappingRuleClaimName is the claim name of one list element
	// of KeyInitializationMappingRules (ConfiguredMappingRule.java).
	KeyInitializationMappingRuleClaimName Key = "camunda.security.initialization.mapping-rules[N].claim-name"
	// KeyInitializationMappingRuleClaimValue is the claim value of one list element
	// of KeyInitializationMappingRules (ConfiguredMappingRule.java).
	KeyInitializationMappingRuleClaimValue Key = "camunda.security.initialization.mapping-rules[N].claim-value"
	// KeyDefaultRolesAdminClients is camunda.security.initialization.default-roles.admin.clients
	// (InitializationConfiguration.java, PlatformDefaultEntities.java).
	KeyDefaultRolesAdminClients Key = "camunda.security.initialization.default-roles.admin.clients"
	// KeyDefaultRolesAdminClientItem is one list element of KeyDefaultRolesAdminClients.
	KeyDefaultRolesAdminClientItem Key = "camunda.security.initialization.default-roles.admin.clients[N]"
	// KeyDefaultRolesAdminMappingRules is
	// camunda.security.initialization.default-roles.admin.mapping-rules
	// (InitializationConfiguration.java, PlatformDefaultEntities.java).
	KeyDefaultRolesAdminMappingRules Key = "camunda.security.initialization.default-roles.admin.mapping-rules"
	// KeyDefaultRolesAdminMappingRuleItem is one list element of KeyDefaultRolesAdminMappingRules.
	KeyDefaultRolesAdminMappingRuleItem Key = "camunda.security.initialization.default-roles.admin.mapping-rules[N]"
	// KeyDefaultRolesConnectorsClients is
	// camunda.security.initialization.default-roles.connectors.clients
	// (InitializationConfiguration.java, PlatformDefaultEntities.java).
	KeyDefaultRolesConnectorsClients Key = "camunda.security.initialization.default-roles.connectors.clients"
	// KeyDefaultRolesConnectorsClientItem is one list element of KeyDefaultRolesConnectorsClients.
	KeyDefaultRolesConnectorsClientItem Key = "camunda.security.initialization.default-roles.connectors.clients[N]"
```

Add all twelve constants to the `declared` slice in the same order.

- [ ] **Step 3: Excuse the new keys in the source scan**

Every new key is a security class, and `defaults.yaml` carries no security class. Add one line per key to `notInDefaultsYAML` with the existing reason string `"security classes are not generated into defaults.yaml"`.

- [ ] **Step 4: Run the key tests**

```bash
go test ./pkg/camundaconfig/...
CAMUNDA_SOURCE_DIR=<checkout at 8.9.9> go test ./pkg/camundaconfig/... -run Source -v
```

Expected: PASS. A key that is wrongly excused fails with "is listed in dist/src/main/config/defaults.yaml; remove it from notInDefaultsYAML".

- [ ] **Step 5: Commit**

```bash
git add pkg/camundaconfig/
git commit -m "feat(camundaconfig): declare the OIDC claim and role membership keys (#73)"
```

### Task 2: Add the CRD fields

**Files:**
- Modify: `api/v1/camundaplatformconfig_types.go:39-71`
- Modify: `api/v1/camundacluster_types.go:187-202`
- Regenerate: `api/v1/zz_generated.deepcopy.go`, `config/crd/bases/`

**Interfaces:**
- Consumes: nothing.
- Produces: `v1.OIDCSpec.UsernameClaim string`, `v1.OIDCSpec.ClientIDClaim string`, `v1.ClusterAuthSpec.Admin *v1.ClusterAdminSpec`, `type v1.ClusterAdminSpec struct { Users, Clients []string; MappingRules []AdminMappingRule }`, `type v1.AdminMappingRule struct { ID, ClaimName, ClaimValue string }`.

- [ ] **Step 1: Add the two claim fields to `OIDCSpec`**

Insert after `Audience` (`camundaplatformconfig_types.go:67`):

```go
	// UsernameClaim is the token claim that holds the username of a person.
	// Empty means the Camunda default, which is "sub".
	// +optional
	UsernameClaim string `json:"usernameClaim,omitempty"`
	// ClientIDClaim is the token claim that holds the id of a machine client.
	// Empty means that no claim identifies a client, and every token is
	// resolved to a person. The claim must be absent from the tokens of
	// persons, because a token that carries it is always resolved to a client.
	// +optional
	ClientIDClaim string `json:"clientIdClaim,omitempty"`
```

- [ ] **Step 2: Add the admin bootstrap types to `camundacluster_types.go`**

Insert after `ClusterAuthSpec` (`camundacluster_types.go:202`):

```go
// ClusterAdminSpec holds the identities that get the admin role of one
// cluster. The identity provider authenticates them, and this block is the
// only thing that authorizes them, so a cluster with an empty block has no
// administrator.
type ClusterAdminSpec struct {
	// Users are the values of the username claim that get the admin role.
	// +optional
	Users []string `json:"users,omitempty"`
	// Clients are the values of the client id claim that get the admin role.
	// A client entry only matches when the platform config sets
	// clientIdClaim.
	// +optional
	Clients []string `json:"clients,omitempty"`
	// MappingRules give the admin role to every token that matches a claim.
	// +optional
	MappingRules []AdminMappingRule `json:"mappingRules,omitempty"`
}

// AdminMappingRule gives the admin role to every token in which the claim
// claimName holds the value claimValue.
type AdminMappingRule struct {
	// ID names the rule inside the cluster. Admin shows it under Mapping
	// Rules.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	ID string `json:"id"`
	// ClaimName is the name of a claim, or a JSONPath expression that points
	// at a claim.
	// +kubebuilder:validation:MinLength=1
	ClaimName string `json:"claimName"`
	// ClaimValue is the value that the claim must hold.
	// +kubebuilder:validation:MinLength=1
	ClaimValue string `json:"claimValue"`
}
```

Add the field to `ClusterAuthSpec`, after `ClientSecretRef`:

```go
	// Admin holds the identities that get the admin role of this cluster. It
	// applies under OIDC only, and it is ignored under basic authentication,
	// which seeds its own administrator.
	// +optional
	Admin *ClusterAdminSpec `json:"admin,omitempty"`
```

- [ ] **Step 3: Regenerate**

```bash
make manifests generate
```

Expected: `zz_generated.deepcopy.go` gains `ClusterAdminSpec` and `AdminMappingRule`, and the four CRD bases that embed `CamundaClusterSpec` gain the new properties.

- [ ] **Step 4: Build**

```bash
go build ./... && go vet ./...
```

Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add api/ config/
git commit -m "feat(api): add the OIDC claim names and the admin bootstrap block (#73)"
```

### Task 3: Merge the admin block from a preset

**Files:**
- Modify: `pkg/components/camundacluster/presetmerge.go:112-131`
- Test: `pkg/components/camundacluster/presetmerge_test.go`

**Interfaces:**
- Consumes: `v1.ClusterAdminSpec` from Task 2.
- Produces: `mergeAuth` handles `Admin`. No new exported symbol.

- [ ] **Step 1: Write the failing test**

Add to `presetmerge_test.go`:

```go
func TestMergePresetAdminBlockReplacesWholesale(t *testing.T) {
	t.Parallel()

	preset := &v1.CamundaClusterPresetSpec{Cluster: v1.CamundaClusterSpec{
		Auth: &v1.ClusterAuthSpec{
			ClientID: "preset-client",
			Admin:    &v1.ClusterAdminSpec{Users: []string{"platform-ops"}, Clients: []string{"platform-ops"}},
		},
	}}

	inherited := MergePreset(v1.CamundaClusterSpec{}, preset)
	require.NotNil(t, inherited.Auth.Admin)
	assert.Equal(t, []string{"platform-ops"}, inherited.Auth.Admin.Users)

	replaced := MergePreset(v1.CamundaClusterSpec{
		Auth: &v1.ClusterAuthSpec{Admin: &v1.ClusterAdminSpec{Users: []string{"team-a"}}},
	}, preset)
	require.NotNil(t, replaced.Auth.Admin)
	assert.Equal(t, []string{"team-a"}, replaced.Auth.Admin.Users)
	assert.Empty(t, replaced.Auth.Admin.Clients, "the preset clients do not survive a cluster block")
	assert.Equal(t, "preset-client", replaced.Auth.ClientID, "the other auth fields still merge per field")
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./pkg/components/camundacluster/ -run TestMergePresetAdminBlockReplacesWholesale -v
```

Expected: FAIL, because `replaced.Auth.Admin.Clients` still holds `platform-ops`.

- [ ] **Step 3: Add the merge clause**

In `mergeAuth`, after the `ClientSecretRef` clause:

```go
	if over.Admin != nil {
		base.Admin = over.Admin
	}
```

- [ ] **Step 4: Update the `MergePreset` GoDoc**

The doc comment lists what merges and how. Add the admin block to the sentence about `Scheduling`, because they now share the rule: a block set on the cluster replaces the block of the preset entirely.

- [ ] **Step 5: Run the package tests**

```bash
go test ./pkg/components/camundacluster/ -run TestMergePreset -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/components/camundacluster/presetmerge.go pkg/components/camundacluster/presetmerge_test.go
git commit -m "feat(camundacluster): let a preset carry the admin bootstrap block (#73)"
```

### Task 4: Resolve the admin block into the effective auth

**Files:**
- Modify: `pkg/components/camundacluster/input.go:76-119`
- Test: `pkg/components/camundacluster/render_test.go` (next to `TestResolveAuth:392`)

**Interfaces:**
- Consumes: `v1.ClusterAdminSpec` from Task 2.
- Produces: `EffectiveAuth.Admin *v1.ClusterAdminSpec`, set only when `Method` is `oidc`.

- [ ] **Step 1: Write the failing test**

```go
func TestResolveAuthAdmin(t *testing.T) {
	t.Parallel()

	admin := &v1.ClusterAdminSpec{Clients: []string{"my-cluster-client"}}

	basic := ResolveAuth(newInput(t, func(in *Input) {
		in.Cluster.Spec.Auth = &v1.ClusterAuthSpec{Admin: admin}
		in.Effective = NewEffective(in.Cluster.Spec)
	}))
	assert.Nil(t, basic.Admin, "basic authentication seeds its own administrator")

	oidc := ResolveAuth(newInput(t, func(in *Input) {
		in.Platform = oidcPlatform()
		in.Cluster.Spec.Auth = &v1.ClusterAuthSpec{Admin: admin}
		in.Effective = NewEffective(in.Cluster.Spec)
	}))
	require.NotNil(t, oidc.Admin)
	assert.Equal(t, []string{"my-cluster-client"}, oidc.Admin.Clients)
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./pkg/components/camundacluster/ -run TestResolveAuthAdmin -v
```

Expected: FAIL with "oidc.Admin undefined".

- [ ] **Step 3: Add the field and fill it**

In `EffectiveAuth`:

```go
	// Admin holds the members of the admin role. It is set when Method is
	// oidc and the effective cluster auth carries the block.
	Admin *v1.ClusterAdminSpec
```

In `ResolveAuth`, inside the `override := in.Effective.Auth` block, after the `ClientSecretRef` clause:

```go
		auth.Admin = override.Admin
```

The assignment sits after the early return for the non-OIDC case, so `Admin` stays nil under basic authentication.

- [ ] **Step 4: Run the test**

```bash
go test ./pkg/components/camundacluster/ -run TestResolveAuth -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/components/camundacluster/input.go pkg/components/camundacluster/render_test.go
git commit -m "feat(camundacluster): resolve the admin bootstrap into the effective auth (#73)"
```

### Task 5: Render the claims, the admin role, and the connectors role

**Files:**
- Modify: `pkg/components/camundacluster/render.go:231-264`
- Test: `pkg/components/camundacluster/render_test.go`

**Interfaces:**
- Consumes: `EffectiveAuth.Admin` from Task 4, the keys from Task 1.
- Produces: no exported symbol. Three unexported helpers: `adminRoleEnv(admin *v1.ClusterAdminSpec) []corev1.EnvVar`, `mappingRuleEnv(rules []v1.AdminMappingRule) []corev1.EnvVar`, `connectorsRoleEnv(in Input, oidc *v1.OIDCSpec) []corev1.EnvVar`.

- [ ] **Step 1: Write the failing tests**

```go
func TestRenderOIDCClaims(t *testing.T) {
	t.Parallel()

	bare := newInput(t, func(in *Input) { in.Platform = oidcPlatform() })
	r := render(bare, process(t, bare, ComponentGateway))
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_USERNAMECLAIM")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_CLIENTIDCLAIM")

	set := newInput(t, func(in *Input) {
		in.Platform = oidcPlatform()
		in.Platform.Auth.OIDC.UsernameClaim = "preferred_username"
		in.Platform.Auth.OIDC.ClientIDClaim = "client_id"
	})
	r = render(set, process(t, set, ComponentGateway))
	assertEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_USERNAMECLAIM", "preferred_username")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_CLIENTIDCLAIM", "client_id")
}

func TestRenderOIDCAdminBootstrap(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Platform = oidcPlatform()
		in.Cluster.Spec.Auth = &v1.ClusterAuthSpec{Admin: &v1.ClusterAdminSpec{
			Users:   []string{"ada@example.com", "grace@example.com"},
			Clients: []string{"my-cluster-client"},
			MappingRules: []v1.AdminMappingRule{
				{ID: "platform-admins", ClaimName: "groups", ClaimValue: "camunda-admins"},
			},
		}}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
	r := render(in, process(t, in, ComponentGateway))

	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_USERS_0", "ada@example.com")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_USERS_1", "grace@example.com")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_CLIENTS_0", "my-cluster-client")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_MAPPINGRULES_0", "platform-admins")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_MAPPINGRULES_0_MAPPINGRULEID", "platform-admins")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_MAPPINGRULES_0_CLAIMNAME", "groups")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_MAPPINGRULES_0_CLAIMVALUE", "camunda-admins")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_USERS_0_USERNAME")
}

// Basic authentication seeds its own administrator, so the block renders
// nothing and the seeded admin user is untouched.
func TestRenderAdminBootstrapIgnoredUnderBasicAuth(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Cluster.Spec.Auth = &v1.ClusterAuthSpec{Admin: &v1.ClusterAdminSpec{
			Clients: []string{"my-cluster-client"},
		}}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
	r := render(in, process(t, in, ComponentGateway))

	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_USERS_0", "admin")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_CLIENTS_0")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_MAPPINGRULES_0_MAPPINGRULEID")
}

func TestRenderConnectorsRoleGrant(t *testing.T) {
	t.Parallel()

	withClaim := func(claim string, connectors bool) Input {
		return newInput(t, func(in *Input) {
			in.Platform = oidcPlatform()
			in.Platform.Auth.OIDC.ClientIDClaim = claim
			if connectors {
				in.Cluster.Spec.Connectors = &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"}
			}
			in.Effective = NewEffective(in.Cluster.Spec)
		})
	}

	in := withClaim("client_id", true)
	r := render(in, process(t, in, ComponentGateway))
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_CONNECTORS_CLIENTS_0", "platform-client")

	in = withClaim("", true)
	r = render(in, process(t, in, ComponentGateway))
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_CONNECTORS_CLIENTS_0")

	in = withClaim("client_id", false)
	r = render(in, process(t, in, ComponentGateway))
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_CONNECTORS_CLIENTS_0")
}
```

- [ ] **Step 2: Run them and watch them fail**

```bash
go test ./pkg/components/camundacluster/ -run 'TestRenderOIDCClaims|TestRenderOIDCAdminBootstrap|TestRenderAdminBootstrapIgnored|TestRenderConnectorsRoleGrant' -v
```

Expected: FAIL. The claim fields do not exist yet on the fixture, and the role variables are absent.

- [ ] **Step 3: Extend `oidcEnv` and add the three helpers**

In `oidcEnv`, extend the `optional` table with the two claim keys, so they follow the same "render only when set" rule as the discovery overrides:

```go
	optional := []struct {
		key   camundaconfig.Key
		value string
	}{
		{camundaconfig.KeyOIDCJWKSetURI, oidc.JWKSURL},
		{camundaconfig.KeyOIDCTokenURI, oidc.TokenURL},
		{camundaconfig.KeyOIDCAuthorizationURI, oidc.AuthURL},
		{camundaconfig.KeyOIDCUsernameClaim, oidc.UsernameClaim},
		{camundaconfig.KeyOIDCClientIDClaim, oidc.ClientIDClaim},
	}
```

At the end of `oidcEnv`, after the redirect clause, append the bootstrap:

```go
	env = append(env, adminRoleEnv(ResolveAuth(in).Admin)...)
	env = append(env, connectorsRoleEnv(in, oidc)...)
```

Then add the helpers below `oidcEnv`:

```go
// adminRoleEnv makes the members of admin members of the admin role, and
// declares the mapping rules that the block references. Camunda seeds no
// administrator under OIDC, so an empty block leaves the cluster with none.
func adminRoleEnv(admin *v1.ClusterAdminSpec) []corev1.EnvVar {
	if admin == nil {
		return nil
	}

	var env []corev1.EnvVar
	for i, user := range admin.Users {
		env = append(env, camundaconfig.Var(
			camundaconfig.Index(camundaconfig.KeyDefaultRolesAdminUsers, i, ""), user,
		))
	}

	for i, client := range admin.Clients {
		env = append(env, camundaconfig.Var(
			camundaconfig.Index(camundaconfig.KeyDefaultRolesAdminClients, i, ""), client,
		))
	}

	for i, rule := range admin.MappingRules {
		env = append(
			env,
			camundaconfig.Var(
				camundaconfig.Index(camundaconfig.KeyDefaultRolesAdminMappingRules, i, ""), rule.ID,
			),
			camundaconfig.Var(
				camundaconfig.Index(camundaconfig.KeyInitializationMappingRules, i, "mapping-rule-id"), rule.ID,
			),
			camundaconfig.Var(
				camundaconfig.Index(camundaconfig.KeyInitializationMappingRules, i, "claim-name"), rule.ClaimName,
			),
			camundaconfig.Var(
				camundaconfig.Index(camundaconfig.KeyInitializationMappingRules, i, "claim-value"), rule.ClaimValue,
			),
		)
	}

	return env
}

// connectorsRoleEnv gives the connectors role to the OIDC client of the
// cluster, which is the client that the connectors runtime authenticates
// with. Without a client id claim the runtime is resolved to a person, and a
// client member would never match it, so the grant is left out.
func connectorsRoleEnv(in Input, oidc *v1.OIDCSpec) []corev1.EnvVar {
	if oidc.ClientIDClaim == "" || !in.Effective.ConnectorsEnabled() {
		return nil
	}

	return []corev1.EnvVar{camundaconfig.Var(
		camundaconfig.Index(camundaconfig.KeyDefaultRolesConnectorsClients, 0, ""), oidc.ClientID,
	)}
}
```

`mappingRuleEnv` from the Interfaces block is folded into `adminRoleEnv`, because the rule definition and its membership are written in one loop over the same slice. Do not split it back out.

- [ ] **Step 4: Run the tests**

```bash
go test ./pkg/components/camundacluster/ -run 'TestRender' -v
```

Expected: PASS, including `TestRenderOnlyDeclaredKeys`.

- [ ] **Step 5: Commit**

```bash
git add pkg/components/camundacluster/render.go pkg/components/camundacluster/render_test.go
git commit -m "feat(camundacluster): render the OIDC claims and the admin bootstrap (#73)"
```

### Task 6: Pin the new render in the goldens

**Files:**
- Modify: `pkg/components/camundacluster/fixtures_test.go:188-204`
- Modify: `pkg/components/camundacluster/testdata/golden/oidc/*.yaml`

**Interfaces:**
- Consumes: everything from Tasks 1 to 5.
- Produces: the `oidc` golden fixture now covers the claims, all three member kinds, and the connectors grant.

- [ ] **Step 1: Extend `fixtureOIDC`**

The fixture already sets an `externalUrl`, a cluster client id override, and connectors. Add the claims to the platform and the admin block to the cluster auth:

```go
		in.Platform = oidcPlatform()
		in.Platform.Auth.OIDC.UsernameClaim = "preferred_username"
		in.Platform.Auth.OIDC.ClientIDClaim = "client_id"
		in.Cluster.Spec.ExternalURL = "https://my-cluster.camunda.example.com"
		in.Cluster.Spec.Auth = &v1.ClusterAuthSpec{
			ClientID: "my-cluster-client",
			ClientSecretRef: &v1.SecretKeyRef{
				Name:      "my-cluster-oidc-secret",
				Namespace: "my-cluster-ns",
				Key:       "client-secret",
			},
			Admin: &v1.ClusterAdminSpec{
				Users:   []string{"ada@example.com"},
				Clients: []string{"my-cluster-client"},
				MappingRules: []v1.AdminMappingRule{
					{ID: "platform-admins", ClaimName: "groups", ClaimValue: "camunda-admins"},
				},
			},
		}
```

Update the fixture's doc comment to name what it now encodes.

- [ ] **Step 2: Refresh the goldens**

```bash
go test ./pkg/components/camundacluster/ -run Golden -update-golden
```

- [ ] **Step 3: Read the diff before you trust it**

```bash
git diff pkg/components/camundacluster/testdata/golden/oidc/
```

Expected: `zeebe.yaml`, `gateway.yaml`, and `connectors.yaml` gain the claim variables and the role variables. `connectors.yaml` gains `CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_CONNECTORS_CLIENTS_0` only if connectors run a unified process; if the connectors bundle carries only `camunda.client.*` variables, the role grant appears on the unified processes alone, which is correct — the engine reads it, not the runtime. No other golden directory changes.

- [ ] **Step 4: Run the whole package**

```bash
go test ./pkg/components/camundacluster/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/components/camundacluster/fixtures_test.go pkg/components/camundacluster/testdata/
git commit -m "test(camundacluster): pin the OIDC admin bootstrap in the goldens (#73)"
```

### Task 7: Document the new surface

**Files:**
- Modify: `docs/crds/camundaplatformconfig.md`
- Modify: `docs/crds/camundacluster.md`
- Modify: `docs/crds/camundaclusterpreset.md`

**Interfaces:**
- Consumes: the CRD surface from Task 2.
- Produces: nothing in code.

- [ ] **Step 1: Document the claim names**

In `camundaplatformconfig.md`, add `usernameClaim` and `clientIdClaim` to the annotated `spec.auth.oidc` example, in the field order of the Go type. Add a short paragraph under the auth section that states how Camunda resolves a principal: the client id claim first, then the username claim, and a token with neither is refused. State the trap plainly — a client id claim that also appears in the tokens of persons resolves a person to a client.

- [ ] **Step 2: Document the admin block**

In `camundacluster.md`, add `spec.auth.admin` to the annotated spec example with all three member kinds. Add a paragraph that states three facts: the block applies under OIDC only and is ignored under basic authentication, a `clients` entry needs `clientIdClaim` on the platform config, and the operator grants the connectors role to the cluster client on its own when connectors are enabled and `clientIdClaim` is set.

Add the second trap next to it: Camunda shows the first-run setup page while the admin role has no member of the `users` kind, so a cluster whose only administrator is a client still redirects a browser to `/admin/setup`.

- [ ] **Step 3: Document the merge rule**

In `camundaclusterpreset.md`, add `spec.auth.admin` to the merge rules table with the replacement rule, next to the `scheduling` row it matches.

- [ ] **Step 4: Read the prose back against the rules**

Descriptive text, so sentences stay at or under 25 words. No `should`. Conditions come before commands. One topic per paragraph.

- [ ] **Step 5: Commit**

```bash
git add docs/crds/
git commit -m "docs(crds): describe the OIDC claim names and the admin bootstrap (#73)"
```

### Task 8: Close out PR 1

- [ ] **Step 1: Full gate**

```bash
make all
go test ./...
```

Expected: lint clean, every package PASS.

- [ ] **Step 2: Open the sub-PR**

Target `feat/oidc-admin-bootstrap`, body carries `Towards #73`. Follow `feature-dev-workflow:opening-a-pull-request`.

---

# PR 2 — e2e: prove the OIDC path against Keycloak (#61)

Branch `feat/oidc-admin-bootstrap--keycloak-e2e` off `feat/oidc-admin-bootstrap` **after PR 1 has merged into it**.

### Task 9: Add Keycloak to the kind suite

**Files:**
- Create: `test/e2e/testdata/keycloak.yaml`

**Interfaces:**
- Consumes: nothing.
- Produces: a Keycloak Service at `keycloak.<namespace>.svc:8080`, issuer `http://keycloak.<namespace>.svc:8080/realms/camunda`, client id `camunda`, client secret `camunda-e2e-secret`, static user `ada@example.com`.

- [ ] **Step 1: Write the manifest**

One file with a ConfigMap, a Deployment, and a Service, in that order. The ConfigMap holds `realm.json`. The Deployment runs `quay.io/keycloak/keycloak:26.4` with args `["start-dev", "--import-realm"]`, mounts the ConfigMap at `/opt/keycloak/data/import`, and sets `KC_BOOTSTRAP_ADMIN_USERNAME` and `KC_BOOTSTRAP_ADMIN_PASSWORD`. Give it a readiness probe on `/realms/camunda/.well-known/openid-configuration`, port 8080, so `kubectl rollout status` means the realm is actually served.

The realm needs exactly this, and nothing more:

```json
{
  "realm": "camunda",
  "enabled": true,
  "accessTokenLifespan": 1800,
  "clients": [
    {
      "clientId": "camunda",
      "enabled": true,
      "publicClient": false,
      "secret": "camunda-e2e-secret",
      "standardFlowEnabled": true,
      "serviceAccountsEnabled": true,
      "redirectUris": ["*"],
      "protocolMappers": [
        {
          "name": "camunda-audience",
          "protocol": "openid-connect",
          "protocolMapper": "oidc-audience-mapper",
          "config": {
            "included.client.audience": "camunda",
            "access.token.claim": "true"
          }
        },
        {
          "name": "camunda-client-id",
          "protocol": "openid-connect",
          "protocolMapper": "oidc-hardcoded-claim-mapper",
          "config": {
            "claim.name": "client_id",
            "claim.value": "camunda",
            "access.token.claim": "true",
            "jsonType.label": "String"
          }
        }
      ]
    }
  ],
  "users": [
    {
      "username": "ada@example.com",
      "enabled": true,
      "email": "ada@example.com",
      "emailVerified": true,
      "credentials": [{"type": "password", "value": "ada-e2e-password", "temporary": false}]
    }
  ]
}
```

The audience mapper exists because Keycloak issues `aud: account` by default and the Camunda `audiences` check would refuse the token. The hardcoded claim exists because a Keycloak access token carries `azp` but no `client_id`, and `azp` is also present in the tokens of persons.

- [ ] **Step 2: Try it against the kind cluster by hand**

```bash
kubectl create ns keycloak-probe
kubectl apply -n keycloak-probe -f test/e2e/testdata/keycloak.yaml
kubectl rollout status -n keycloak-probe deployment/keycloak --timeout 5m
kubectl run probe -n keycloak-probe --rm -i --restart=Never --image=curlimages/curl:8.17.0 -- \
  sh -ec 'curl -sS -d grant_type=client_credentials -d client_id=camunda -d client_secret=camunda-e2e-secret \
    http://keycloak.keycloak-probe.svc:8080/realms/camunda/protocol/openid-connect/token'
```

Expected: a JSON body with `access_token`. Decode the payload and confirm it carries `"aud"` containing `camunda` and `"client_id": "camunda"`. If either is missing, correct the mapper config before you write any Go.

```bash
kubectl delete ns keycloak-probe --wait=false
```

- [ ] **Step 3: Commit**

```bash
git add test/e2e/testdata/keycloak.yaml
git commit -m "test(e2e): add a Keycloak realm for the OIDC flow (#61)"
```

### Task 10: Teach the request helper two more authentication modes

**Files:**
- Modify: `test/utils/camunda.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `CamundaRequest.Auth CamundaAuth` replaces the three credential fields. `BasicAuth{Secret, UsernameKey, PasswordKey string}` and `ClientCredentials{TokenURL, ClientID, Audience string, Secret, SecretKey string}` implement it. A nil `Auth` sends no credentials.

- [ ] **Step 1: Replace the credential fields with a mode**

`CamundaRequest` currently carries `CredentialsSecret`, `UsernameKey`, and `PasswordKey`, and `CamundaREST` always passes `-u`. Replace those three fields with one:

```go
// Auth selects how the call authenticates. A nil Auth sends no credentials,
// which is how the suite proves that an endpoint refuses an anonymous call.
Auth CamundaAuth
```

```go
// CamundaAuth is the authentication of one call. BasicAuth and
// ClientCredentials implement it.
type CamundaAuth interface {
	// env returns the variables the helper pod needs, and script returns the
	// shell that turns them into curl arguments in $AUTH_ARGS.
	env() []corev1.EnvVar
	script() string
}

// BasicAuth sends the user of a Secret with curl -u. The pod reads the Secret
// through secretKeyRef, so the password never appears in the pod spec.
type BasicAuth struct {
	Secret      string
	UsernameKey string
	PasswordKey string
}

// ClientCredentials reads an access token from TokenURL with the OAuth
// client-credentials grant, then sends it as a bearer token. Both requests
// happen inside the helper pod, so the token never reaches the test output.
type ClientCredentials struct {
	TokenURL  string
	ClientID  string
	Audience  string
	Secret    string
	SecretKey string
}
```

`BasicAuth.script()` sets `AUTH_ARGS` from the two variables. `ClientCredentials.script()` posts the grant with curl, extracts `access_token` with `sed`, fails the pod when the field is absent, and sets `AUTH_ARGS` to an `Authorization: Bearer` header. Keep the extraction dependency-free — `curlimages/curl` carries no `jq`:

```sh
TOKEN=$(curl -sS -X POST \
  -d grant_type=client_credentials \
  -d "client_id=$CAMUNDA_CLIENT_ID" \
  -d "client_secret=$CAMUNDA_CLIENT_SECRET" \
  -d "audience=$CAMUNDA_AUDIENCE" \
  "$CAMUNDA_TOKEN_URL" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')
if [ -z "$TOKEN" ]; then echo "no access_token from $CAMUNDA_TOKEN_URL" >&2; exit 1; fi
```

The existing script builds curl arguments through `"$@"`. Keep that shape and add `$AUTH_ARGS` unquoted at the call, so an empty value contributes no argument.

- [ ] **Step 2: Update the one existing caller**

`camundaRESTOn` in `test/e2e/camundacluster_test.go:537` sets the three old fields. Replace with:

```go
		Auth: utils.BasicAuth{
			Secret:      components.AdminSecretName(cluster),
			UsernameKey: components.AdminUsernameKey,
			PasswordKey: components.AdminPasswordKey,
		},
```

- [ ] **Step 3: Build the e2e package**

```bash
go vet -tags e2e ./test/...
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add test/utils/camunda.go test/e2e/camundacluster_test.go
git commit -m "test(e2e): give the request helper a bearer-token and an anonymous mode (#61)"
```

### Task 11: Write the OIDC flow

**Files:**
- Create: `test/e2e/camundacluster_oidc_test.go`
- Modify: `test/e2e/helpers_test.go` if a shared helper is needed

**Interfaces:**
- Consumes: Tasks 9 and 10, and the CRD surface of PR 1.
- Produces: nothing.

- [ ] **Step 1: Write the constants and the fixtures**

Follow the constant block of `camundacluster_test.go:43-81`. Namespace `camunda-oidc-e2e`, platform config `camunda-oidc-e2e`, cluster `camunda`, realm issuer built from the Keycloak Service in the same namespace, client `camunda`, secret name `camunda-oidc-client` created in `camunda-system` so the mirror path runs.

The `externalUrl` is the gateway Service URL, because the redirect assertion compares against it:
`http://<gateway service>.camunda-oidc-e2e.svc:8080`.

- [ ] **Step 2: Write `BeforeAll`**

Create the namespace, apply `testdata/keycloak.yaml`, wait for the rollout, create the client-secret Secret in `camunda-system`, create the `ElasticsearchCluster` and wait for `Ready` `Healthy`, then apply the `CamundaPlatformConfig` with `auth.method: oidc`, the issuer, `clientId: camunda`, `clientIdClaim: client_id`, `usernameClaim: preferred_username`, and the cross-namespace `clientSecretRef`.

- [ ] **Step 3: Write the specs**

Six `It` blocks, in the order of the spec document:

1. `reaches Ready Healthy with the mirrored client secret` — `expectReady` on the cluster, and `expectCondition` for `MirroredSecretsReady`.
2. `accepts a client-credentials token on the topology endpoint` — `camundaOIDCREST` with `ClientCredentials`, expect 200 and one broker.
3. `refuses an anonymous call` — the same URL with a nil `Auth`, expect 401.
4. `redirects a web application login to the identity provider` — `GET /operate/` with a nil `Auth`, following redirects, and assert that the final URL holds both the Keycloak authorization path and `redirect_uri` with the `externalUrl` and `/sso-callback`. The value is percent-encoded in the query, so compare against the encoded form or unescape before the compare.
5. `runs the connectors runtime under OIDC` — `expectCondition` for `ConnectorsReady` `Healthy`.
6. `deploys a process, starts an instance, and finds it in secondary storage with the token` — the same three calls as `itRunsTheOrchestrationCluster`, with the token instead of the admin user. This is the spec that fails without the admin grant.

`AfterAll` removes the cluster, the platform config, the Secret in `camunda-system`, and the namespace. `AfterEach` calls `dumpDiagnostics(ccOIDCNamespace)`.

- [ ] **Step 4: Run only the new flow**

```bash
make setup-test-e2e
go test -tags e2e ./test/e2e/ -v --ginkgo.focus="CamundaCluster with OIDC" 2>&1 | tail -60
```

Expected: all six specs PASS. If spec 6 fails with 403, the admin grant did not reach the engine — read `CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_CLIENTS_0` on the zeebe pod and check the `client_id` claim in the token.

- [ ] **Step 5: Run the whole suite and measure it**

```bash
time make test-e2e 2>&1 | tail -40
```

Expected: green. Record the wall-clock time. If it does not fit inside `E2E_TIMEOUT`, move the OIDC flow to its own job and say so in the PR body.

- [ ] **Step 6: Commit**

```bash
git add test/e2e/
git commit -m "test(e2e): prove the OIDC path of a CamundaCluster against Keycloak (#61)"
```

### Task 12: Close out PR 2 and the feature

- [ ] **Step 1: Full gate**

```bash
make all
go test ./...
make test-e2e
```

- [ ] **Step 2: Open the sub-PR**

Target `feat/oidc-admin-bootstrap`, body carries `Towards #61`.

- [ ] **Step 3: Open the integration PR**

Target `main`, body carries `Closes #72`. Delete the plan and the state file in the last commit of the feature branch, and keep the spec.

---

## Self-review

**Spec coverage.** Claim names → Tasks 1, 2, 5, 7. Admin block → Tasks 1, 2, 5, 7. Preset merge → Task 3. `EffectiveAuth` → Task 4. Connectors grant → Tasks 1, 5, 7. Goldens → Task 6. Keycloak realm → Task 9. Request helper → Task 10. The six e2e assertions → Task 11. Every spec section maps to a task.

**Type consistency.** `ClusterAdminSpec` and `AdminMappingRule` are declared in Task 2 and used with the same field names in Tasks 3, 4, 5, and 6. `EffectiveAuth.Admin` is declared in Task 4 and read in Task 5. The key constants are declared in Task 1 and used in Task 5 with the same names. `CamundaAuth`, `BasicAuth`, and `ClientCredentials` are declared in Task 10 and used in Task 11.

**Open risk carried into execution.** Task 6 Step 3 names an outcome that depends on how the connectors bundle consumes the environment. Read the diff and decide there. Task 9 Step 2 proves the token shape before any Go is written, because the audience mapper and the hardcoded claim are the two things most likely to be wrong.
