import { readFileSync } from "node:fs";
import { parse } from "yaml";
import { describe, expect, it } from "vitest";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { Environment } from "@/generated/crds/environment";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { clusterFields } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import {
  NO_MANAGEMENT_AUTH_CONFIG,
  OPTIMIZE_NEEDS_ELASTICSEARCH,
} from "./attachments";
import {
  type BasicsContext,
  type ClusterDraft,
  type SettingsData,
  attachmentsErrors,
  basicsErrors,
  claimClusterOf,
  clusterClaimOf,
  clusterCreateChange,
  draftCluster,
  draftOptimize,
  draftSelections,
  holdsWrittenSpec,
  isWrittenClaim,
  offeredPresets,
  pickerEnvironment,
  secretErrors,
  secretFieldErrors,
  secretReferences,
  settingsErrors,
  syncedNamespace,
  withClaimValue,
  withoutClaimValue,
  withoutClaimValues,
} from "./draft";
import type { StorageSummary } from "./storage";
import type { DeletionChoice } from "@/lib/policy";

function environment(
  name: string,
  over: Partial<Environment["spec"]> = {},
  ready?: { status: "True" | "False"; message: string },
): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name },
    spec: over,
    ...(ready && {
      status: {
        conditions: [
          {
            type: "Ready",
            status: ready.status,
            reason: "Test",
            message: ready.message,
            lastTransitionTime: "2026-10-30T00:00:00Z",
          },
        ],
      },
    }),
  };
}

function preset(name: string): CamundaClusterPreset {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaClusterPreset",
    metadata: { name },
    spec: { cluster: {} },
  };
}

const prod = environment("prod", { displayName: "Production" });

const complete: ClusterDraft = {
  environment: "prod",
  namespace: { kind: "existing", name: "team-a" },
  claim: { metadata: { name: "orders" }, spec: { presetRef: "small" } },
};
const context: BasicsContext = {
  environments: [prod],
  namespaces: [{ name: "team-a", environment: "prod" }],
  takenNames: [],
  conflicts: [],
  presets: ["small"],
};

describe("basicsErrors", () => {
  it("passes a complete draft", () => {
    expect(basicsErrors(complete, context)).toEqual([]);
  });

  it("asks for an environment while one exists", () => {
    expect(
      basicsErrors({ ...complete, environment: undefined }, context),
    ).toEqual(["Choose an environment", "Choose a namespace"]);
  });

  it("asks for an environment when the picked one is gone", () => {
    expect(basicsErrors({ ...complete, environment: "old" }, context)).toEqual([
      "Choose an environment",
      "Choose a namespace",
    ]);
  });

  it("asks for no environment when none exists", () => {
    expect(
      basicsErrors(
        { ...complete, environment: undefined },
        { ...context, environments: [] },
      ),
    ).toEqual([]);
  });

  it("refuses an environment that is no longer ready", () => {
    const notReady = environment(
      "prod",
      { displayName: "Production" },
      { status: "False", message: "x" },
    );
    expect(
      basicsErrors(complete, { ...context, environments: [notReady] }),
    ).toEqual(["The environment Production is not ready: x"]);
  });

  it("refuses a not-ready environment with no message without a colon", () => {
    const notReady = environment(
      "prod",
      { displayName: "Production" },
      { status: "False", message: "" },
    );
    expect(
      basicsErrors(complete, { ...context, environments: [notReady] }),
    ).toEqual(["The environment Production is not ready"]);
  });

  it("gives the name errors", () => {
    expect(
      basicsErrors(
        { ...complete, claim: { ...complete.claim, metadata: { name: "" } } },
        context,
      ),
    ).toEqual(["Enter a name"]);
  });

  it("refuses a taken name", () => {
    expect(
      basicsErrors(complete, { ...context, takenNames: ["orders"] }),
    ).toEqual(["The name orders is taken in this namespace."]);
  });

  it("gives each host conflict", () => {
    const conflict =
      "The host orders.team-a.example.com is taken by team-b/billing. Choose another name.";
    expect(
      basicsErrors(complete, { ...context, conflicts: [conflict] }),
    ).toEqual([conflict]);
  });

  it("asks for a namespace", () => {
    expect(
      basicsErrors({ ...complete, namespace: undefined }, context),
    ).toEqual(["Choose a namespace"]);
  });

  it("asks for a namespace when the picked one is gone", () => {
    expect(basicsErrors(complete, { ...context, namespaces: [] })).toEqual([
      "Choose a namespace",
    ]);
  });

  it("asks for a namespace when the picked one moved to another environment", () => {
    expect(
      basicsErrors(complete, {
        ...context,
        namespaces: [{ name: "team-a", environment: "dev" }],
      }),
    ).toEqual(["Choose a namespace"]);
  });

  it("takes any existing namespace without an Environment", () => {
    expect(
      basicsErrors(
        { ...complete, environment: undefined },
        {
          ...context,
          environments: [],
          namespaces: [{ name: "team-a" }],
        },
      ),
    ).toEqual([]);
  });

  it("refuses a new namespace whose name now exists", () => {
    const creating = environment("prod", {
      namespaceCreation: { allowed: true },
    });
    expect(
      basicsErrors(
        {
          ...complete,
          namespace: { kind: "new", name: "team-a", labels: {} },
        },
        { ...context, environments: [creating] },
      ),
    ).toEqual(["The namespace team-a already exists"]);
  });

  it("asks for a namespace when the environment no longer allows creation", () => {
    expect(
      basicsErrors(
        {
          ...complete,
          namespace: { kind: "new", name: "team-new", labels: {} },
        },
        context,
      ),
    ).toEqual(["Choose a namespace"]);
  });

  it("asks for a value of a required label key that the environment added", () => {
    const creating = environment("prod", {
      namespaceCreation: { allowed: true, requiredLabelKeys: ["team"] },
    });
    expect(
      basicsErrors(
        {
          ...complete,
          namespace: { kind: "new", name: "team-new", labels: {} },
        },
        { ...context, environments: [creating] },
      ),
    ).toEqual(["Enter a value for team"]);
  });

  it("takes a required label key that a fixed label sets", () => {
    const creating = environment("prod", {
      namespaceCreation: {
        allowed: true,
        labels: { team: "core" },
        requiredLabelKeys: ["team"],
      },
    });
    expect(
      basicsErrors(
        {
          ...complete,
          namespace: { kind: "new", name: "team-new", labels: {} },
        },
        { ...context, environments: [creating] },
      ),
    ).toEqual([]);
  });

  it("takes a new namespace that the environment allows", () => {
    const creating = environment("prod", {
      namespaceCreation: { allowed: true },
    });
    expect(
      basicsErrors(
        {
          ...complete,
          namespace: { kind: "new", name: "team-new", labels: {} },
        },
        { ...context, environments: [creating] },
      ),
    ).toEqual([]);
  });

  it("asks for a preset when none is picked", () => {
    expect(
      basicsErrors(
        { ...complete, claim: { ...complete.claim, spec: {} } },
        context,
      ),
    ).toEqual(["Choose a preset"]);
  });

  it("asks for a preset when the picked one is not offered", () => {
    expect(basicsErrors(complete, { ...context, presets: ["large"] })).toEqual([
      "Choose a preset",
    ]);
  });

  it("gives the errors in screen order", () => {
    expect(
      basicsErrors(
        { claim: { metadata: { name: "" }, spec: {} } },
        { ...context, conflicts: ["c"] },
      ),
    ).toEqual([
      "Choose an environment",
      "Enter a name",
      "c",
      "Choose a namespace",
      "Choose a preset",
    ]);
  });
});

describe("offeredPresets", () => {
  const all = [preset("small"), preset("large"), preset("medium")];

  it("keeps the order of the environment and skips a missing name", () => {
    expect(
      offeredPresets(
        all,
        environment("prod", {
          presets: { camundaCluster: ["medium", "gone", "small"] },
        }),
      ).map((p) => p.metadata.name),
    ).toEqual(["medium", "small"]);
  });

  it("offers no preset when the environment lists none", () => {
    expect(offeredPresets(all, prod)).toEqual([]);
  });

  it("offers every preset by name without an environment", () => {
    expect(offeredPresets(all, undefined).map((p) => p.metadata.name)).toEqual([
      "large",
      "medium",
      "small",
    ]);
  });
});

describe("pickerEnvironment", () => {
  it("passes the name, the display name, and the namespace creation", () => {
    const creation = { allowed: true, requiredLabelKeys: ["team"] };
    expect(
      pickerEnvironment(
        environment("prod", {
          displayName: "Production",
          namespaceCreation: creation,
        }),
      ),
    ).toEqual({
      name: "prod",
      displayName: "Production",
      namespaceCreation: creation,
    });
  });
});

describe("syncedNamespace", () => {
  const creating = environment("prod", {
    namespaceCreation: {
      allowed: true,
      labels: { "cost-center": "4712" },
      requiredLabelKeys: ["team"],
    },
  });

  it("takes the current fixed labels and keeps the required values", () => {
    expect(
      syncedNamespace(
        {
          kind: "new",
          name: "team-new",
          labels: {
            "cost-center": "4711",
            old: "x",
            team: "search",
            "fleet.camunda.io/environment": "prod",
          },
        },
        creating,
      ),
    ).toEqual({
      kind: "new",
      name: "team-new",
      labels: {
        "cost-center": "4712",
        team: "search",
        "fleet.camunda.io/environment": "prod",
      },
    });
  });

  it("returns the same choice when the labels follow the rules", () => {
    const choice = {
      kind: "new" as const,
      name: "team-new",
      labels: {
        team: "search",
        "cost-center": "4712",
        "fleet.camunda.io/environment": "prod",
      },
    };
    expect(syncedNamespace(choice, creating)).toBe(choice);
  });

  it("returns an existing namespace as it is", () => {
    const choice = { kind: "existing" as const, name: "team-a" };
    expect(syncedNamespace(choice, creating)).toBe(choice);
  });
});

function draftWith(
  spec: ClusterDraft["claim"]["spec"],
  storage?: ClusterDraft["storage"],
): ClusterDraft {
  return {
    environment: "prod",
    namespace: { kind: "existing", name: "team-a" },
    claim: { metadata: { name: "orders" }, spec },
    ...(storage && { storage }),
  };
}

describe("draftSelections", () => {
  it("gives the selections section after section, in the order the step reported them", () => {
    expect(
      draftSelections(
        draftWith(
          {},
          {
            access: [{ section: "access", registration: "gateway" }],
            "volume-encryption": [],
            "connectors-permissions": [
              { section: "connectors-permissions", registration: "a" },
              { section: "connectors-permissions", registration: "b" },
            ],
          },
        ),
      ),
    ).toEqual([
      { section: "access", registration: "gateway" },
      { section: "connectors-permissions", registration: "a" },
      { section: "connectors-permissions", registration: "b" },
    ]);
  });

  it("gives no selections before step 2 reported any", () => {
    expect(draftSelections(draftWith({}))).toEqual([]);
  });
});

describe("clusterClaimOf", () => {
  const deleting: DeletionChoice = {
    defaultPolicy: "Delete",
    keepAllowed: true,
    alwaysDeleted: [],
    policy: "Delete",
  };
  const env = environment("prod", {
    releases: { allowed: ["r1", "r2"], default: "r1" },
    platformConfigRef: "pc",
  });

  it("writes the default release when the draft picks none", () => {
    expect(
      clusterClaimOf(draftWith({ presetRef: "small" }), env, deleting).spec
        .releaseRef,
    ).toBe("r1");
  });

  it("writes the picked release also when it is the default", () => {
    expect(
      clusterClaimOf(
        draftWith({ presetRef: "small", releaseRef: "r1" }),
        env,
        deleting,
      ).spec.releaseRef,
    ).toBe("r1");
  });

  it("writes the picked release that is not the default", () => {
    expect(
      clusterClaimOf(
        draftWith({ presetRef: "small", releaseRef: "r2" }),
        env,
        deleting,
      ).spec.releaseRef,
    ).toBe("r2");
  });

  it("writes no platform config when the pick is the environment default", () => {
    expect(
      clusterClaimOf(
        draftWith({ presetRef: "small", platformConfigRef: "pc" }),
        env,
        deleting,
      ).spec,
    ).not.toHaveProperty("platformConfigRef");
  });

  it("writes no platform config when the draft picks none", () => {
    expect(
      clusterClaimOf(draftWith({ presetRef: "small" }), env, deleting).spec,
    ).not.toHaveProperty("platformConfigRef");
  });

  it("writes a platform config that differs from the environment default", () => {
    expect(
      clusterClaimOf(
        draftWith({ presetRef: "small", platformConfigRef: "other" }),
        env,
        deleting,
      ).spec.platformConfigRef,
    ).toBe("other");
  });

  it("writes no release without an Environment and without a pick", () => {
    expect(
      clusterClaimOf(draftWith({ presetRef: "small" }), undefined, deleting)
        .spec,
    ).not.toHaveProperty("releaseRef");
  });

  it("writes the step 2 selections with their names, never a value", () => {
    const claim = clusterClaimOf(
      draftWith(
        { presetRef: "small" },
        {
          "connectors-permissions": [
            {
              section: "connectors-permissions",
              registration: "aws-connectors-role",
              parameters: { extraPolicyArns: ["sqs"] },
            },
          ],
        },
      ),
      env,
      deleting,
    );

    expect(claim.spec.plugins).toEqual([
      {
        section: "connectors-permissions",
        registration: "aws-connectors-role",
        parameters: { extraPolicyArns: ["sqs"] },
      },
    ]);
    expect(JSON.stringify(claim)).not.toContain("arn:");
  });

  it("leaves plugins out when no section has a selection", () => {
    expect(
      clusterClaimOf(
        draftWith({ presetRef: "small" }, { access: [] }),
        env,
        deleting,
      ).spec,
    ).not.toHaveProperty("plugins");
  });

  it("copies the cluster and the attachments that the draft sets", () => {
    const spec = clusterClaimOf(
      draftWith({
        presetRef: "small",
        cluster: { zeebe: { replicas: 3 } },
        attachments: { optimize: {} },
      } as ClusterDraft["claim"]["spec"]),
      env,
      deleting,
    ).spec;

    expect(spec.cluster).toEqual({ zeebe: { replicas: 3 } });
    expect(spec.attachments).toEqual({ optimize: {} });
  });

  it("writes the deletion policy of the choice when it differs from the default", () => {
    expect(
      clusterClaimOf(draftWith({ presetRef: "small" }), env, {
        ...deleting,
        defaultPolicy: "Orphan",
        keepAllowed: false,
      }).spec.deletionPolicy,
    ).toBe("Delete");
    expect(
      clusterClaimOf(draftWith({ presetRef: "small" }), env, {
        ...deleting,
        policy: "Orphan",
      }).spec.deletionPolicy,
    ).toBe("Orphan");
  });

  it("writes no deletion policy when the policy of the choice is the default", () => {
    expect(
      clusterClaimOf(draftWith({ presetRef: "small" }), env, deleting).spec,
    ).not.toHaveProperty("deletionPolicy");
  });

  it("writes the policy of the choice, not a pick of the draft that the choice refuses", () => {
    expect(
      clusterClaimOf(
        draftWith({ presetRef: "small", deletionPolicy: "Orphan" }),
        env,
        { ...deleting, keepAllowed: false },
      ).spec,
    ).not.toHaveProperty("deletionPolicy");
  });

  it("names the claim and its namespace", () => {
    expect(
      clusterClaimOf(draftWith({ presetRef: "small" }), env, deleting),
    ).toEqual({
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaClusterClaim",
      metadata: { name: "orders", namespace: "team-a" },
      spec: { presetRef: "small", releaseRef: "r1" },
    });
  });
});

function release(name: string, version: string): Resource<"CamundaRelease"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaRelease",
    metadata: { name },
    spec: { version },
  } as Resource<"CamundaRelease">;
}

function platformConfig(name: string): Resource<"CamundaPlatformConfig"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaPlatformConfig",
    metadata: { name },
    spec: {},
  } as unknown as Resource<"CamundaPlatformConfig">;
}

function clusterPreset(
  name: string,
  cluster: CamundaClusterSpec,
): Resource<"CamundaClusterPreset"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaClusterPreset",
    metadata: { name },
    spec: { cluster },
  } as Resource<"CamundaClusterPreset">;
}

const withReleases = (over: Partial<Environment["spec"]> = {}): Environment =>
  environment("prod", {
    releases: {
      allowed: ["camunda-8-8", "camunda-8-9"],
      default: "camunda-8-9",
    },
    platformConfigRef: "basic",
    ...over,
  });

const releases = [
  release("camunda-8-8", "8.8.4"),
  release("camunda-8-9", "8.9.9"),
];

describe("draftCluster", () => {
  it("marks the cluster as made by the claim and merges the preset and the release under the draft", () => {
    const draft: ClusterDraft = {
      environment: "prod",
      namespace: { kind: "existing", name: "team-a" },
      claim: {
        metadata: { name: "orders" },
        spec: { presetRef: "small", cluster: { podLabels: { tier: "gold" } } },
      },
      storage: { access: [{ section: "access", registration: "gateway" }] },
    };
    const { cluster, claim, merged } = draftCluster(
      draft,
      [clusterPreset("small", { podLabels: { team: "a" } })],
      releases,
      withReleases(),
    );
    expect(cluster.metadata?.labels?.[CLAIM_LABEL]).toBe("orders");
    expect(cluster.spec).toMatchObject({
      presetRef: "small",
      releaseRef: "camunda-8-9",
      platformConfigRef: "basic",
    });
    expect(merged.spec.version).toBe("8.9.9");
    expect(merged.spec.podLabels).toEqual({ team: "a", tier: "gold" });
    expect(claim.spec.plugins).toEqual([
      { section: "access", registration: "gateway" },
    ]);
    expect(claim.spec.deletionPolicy).toBeUndefined();
  });

  it("uses the picked release and platform config over the Environment defaults", () => {
    const { cluster, merged } = draftCluster(
      draftWith({
        presetRef: "small",
        releaseRef: "camunda-8-8",
        platformConfigRef: "oidc",
      }),
      [],
      releases,
      withReleases(),
    );
    expect(cluster.spec).toMatchObject({
      releaseRef: "camunda-8-8",
      platformConfigRef: "oidc",
    });
    expect(merged.spec.version).toBe("8.8.4");
  });

  it("merges no release that the lists do not hold", () => {
    const { cluster, merged } = draftCluster(
      draftWith({ presetRef: "small" }),
      [],
      [],
      withReleases(),
    );
    expect(merged.spec.version).toBeUndefined();
    expect(cluster.spec.releaseRef).toBe("camunda-8-9");
  });
});

describe("claimClusterOf", () => {
  it("is the cluster that the claim would make, with the claim label", () => {
    const cluster = claimClusterOf(
      {
        name: "orders",
        namespace: "team-a",
        spec: { presetRef: "small", cluster: { zeebe: { replicas: 3 } } },
      },
      withReleases(),
    );
    expect(cluster.metadata).toEqual({
      name: "orders",
      namespace: "team-a",
      labels: { [CLAIM_LABEL]: "orders" },
    });
    expect(cluster.spec).toEqual({
      zeebe: { replicas: 3 },
      presetRef: "small",
      releaseRef: "camunda-8-9",
      platformConfigRef: "basic",
    });
    expect(cluster.status).toBeUndefined();
  });

  it("keeps the release and the platform config that the claim names", () => {
    const cluster = claimClusterOf(
      {
        name: "orders",
        namespace: "team-a",
        spec: { releaseRef: "camunda-8-8", platformConfigRef: "oidc" },
      },
      withReleases(),
    );
    expect(cluster.spec).toMatchObject({
      releaseRef: "camunda-8-8",
      platformConfigRef: "oidc",
    });
  });

  it("sets no reference that neither the claim nor the Environment names", () => {
    expect(
      claimClusterOf(
        { name: "orders", namespace: "team-a", spec: {} },
        undefined,
      ).spec,
    ).toEqual({});
  });

  it("does not share the cluster spec of the claim", () => {
    const spec = { cluster: { zeebe: { replicas: 3 } } };
    const cluster = claimClusterOf(
      { name: "orders", namespace: "team-a", spec },
      undefined,
    );
    (cluster.spec as { zeebe: { replicas: number } }).zeebe.replicas = 5;
    expect(spec.cluster.zeebe.replicas).toBe(3);
  });
});

describe("settingsErrors", () => {
  const data = (over: Partial<SettingsData> = {}): SettingsData => ({
    environment: withReleases(),
    releases,
    platformConfigs: [platformConfig("basic"), platformConfig("oidc")],
    fields: [],
    ...over,
  });
  const draft = (spec: ClusterDraft["claim"]["spec"] = {}) =>
    draftWith({ presetRef: "small", ...spec });
  const access: PluginSection = {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name: "access" },
    spec: {
      appliesTo: ["CamundaClusterClaim"],
      label: "Access",
      selection: "One",
      slot: "externalAccess",
    },
  };

  it("takes the Environment defaults", () => {
    expect(settingsErrors(draft(), data())).toEqual([]);
  });

  it("asks for a release when the Environment has no default", () => {
    expect(
      settingsErrors(
        draft(),
        data({
          environment: withReleases({
            releases: { allowed: ["camunda-8-9"] },
          }),
        }),
      ),
    ).toEqual(["Pick a release."]);
  });

  it("asks for a release without an Environment", () => {
    expect(
      settingsErrors(
        draft({ platformConfigRef: "basic" }),
        data({ environment: undefined }),
      ),
    ).toEqual(["Pick a release."]);
  });

  it("refuses a release that does not exist", () => {
    expect(settingsErrors(draft(), data({ releases: [releases[0]] }))).toEqual([
      "Release camunda-8-9 of the environment does not exist. Pick another one.",
    ]);
    expect(settingsErrors(draft({ releaseRef: "gone" }), data())).toEqual([
      "Release gone does not exist. Pick another one.",
    ]);
  });

  it("refuses a picked release that the Environment no longer allows", () => {
    expect(
      settingsErrors(
        draft({ releaseRef: "camunda-8-7" }),
        data({ releases: [...releases, release("camunda-8-7", "8.7.2")] }),
      ),
    ).toEqual([
      "The environment does not allow release camunda-8-7. Pick another one.",
    ]);
  });

  it("refuses a draft with no platform config", () => {
    expect(
      settingsErrors(
        draft(),
        data({ environment: withReleases({ platformConfigRef: undefined }) }),
      ),
    ).toEqual(["Pick a platform config."]);
  });

  it("refuses a platform config that does not exist", () => {
    expect(settingsErrors(draft(), data({ platformConfigs: [] }))).toEqual([
      "Platform config basic of the environment does not exist. Pick another one.",
    ]);
    expect(
      settingsErrors(draft({ platformConfigRef: "gone" }), data()),
    ).toEqual(["Platform config gone does not exist. Pick another one."]);
  });

  it("names no release or platform config as missing while its kind cannot be read", () => {
    expect(
      settingsErrors(
        draft({ releaseRef: "camunda-8-8", platformConfigRef: "oidc" }),
        data({
          releases: [],
          platformConfigs: [],
          unread: ["CamundaRelease", "CamundaPlatformConfig"],
        }),
      ),
    ).toEqual([]);
  });

  it("refuses a picked release that the Environment does not allow while the releases cannot be read", () => {
    expect(
      settingsErrors(
        draft({ releaseRef: "camunda-8-7" }),
        data({ releases: [], unread: ["CamundaRelease"] }),
      ),
    ).toEqual([
      "The environment does not allow release camunda-8-7. Pick another one.",
    ]);
  });

  const fieldsOf = (d: ClusterDraft) => {
    const drafted = draftCluster(d, [], releases, withReleases());
    return clusterFields({
      cluster: drafted.cluster,
      merged: drafted.merged,
      claim: drafted.claim as unknown as Resource<"CamundaClusterClaim">,
      sections: [access as unknown as Resource<"PluginSection">],
      registrations: [],
    });
  };

  it("refuses a cluster value that a selected section fills, by the label of its field", () => {
    const filled = draftWith(
      {
        presetRef: "small",
        cluster: { externalUrl: "https://orders.example.com" },
      },
      { access: [{ section: "access", registration: "gateway" }] },
    );
    expect(settingsErrors(filled, data({ fields: fieldsOf(filled) }))).toEqual([
      "externalUrl: Filled by Access, so the claim cannot set it.",
    ]);
  });

  it("allows a field that a selected section fills while the claim leaves it unset", () => {
    const unset = draftWith(
      { presetRef: "small" },
      { access: [{ section: "access", registration: "gateway" }] },
    );
    expect(settingsErrors(unset, data({ fields: fieldsOf(unset) }))).toEqual(
      [],
    );
  });
});

describe("attachmentsErrors", () => {
  const summary = (over: Partial<StorageSummary> = {}): StorageSummary => ({
    optimizeUrl: false,
    slots: [],
    picks: [],
    ...over,
  });
  const backups = { section: "backup-storage", registration: "bucket" };
  const draft = (attachments?: ClusterDraft["claim"]["spec"]["attachments"]) =>
    draftWith({ presetRef: "small", ...(attachments && { attachments }) });

  it("allows no attachment", () => {
    expect(
      attachmentsErrors(draft(), {
        summary: summary(),
        managementAuthConfigs: [],
      }),
    ).toEqual([]);
  });

  it("allows Optimize and a schedule that can attach", () => {
    expect(
      attachmentsErrors(
        draft({
          optimize: { managementAuthConfigRef: "central" },
          backupSchedule: { schedule: "0 2 * * *" },
        }),
        {
          summary: summary({
            secondaryStorageType: "elasticsearch",
            backupStorage: backups,
          }),
          managementAuthConfigs: ["central"],
        },
      ),
    ).toEqual([]);
  });

  it("refuses Optimize with PostgreSQL secondary storage", () => {
    expect(
      attachmentsErrors(
        draft({ optimize: { managementAuthConfigRef: "central" } }),
        {
          summary: summary({ secondaryStorageType: "rdbms" }),
          managementAuthConfigs: ["central"],
        },
      ),
    ).toEqual([OPTIMIZE_NEEDS_ELASTICSEARCH]);
  });

  it("refuses Optimize while no ManagementAuthConfig exists", () => {
    expect(
      attachmentsErrors(
        draft({ optimize: { managementAuthConfigRef: "old" } }),
        {
          summary: summary({ secondaryStorageType: "elasticsearch" }),
          managementAuthConfigs: [],
        },
      ),
    ).toEqual([NO_MANAGEMENT_AUTH_CONFIG]);
  });

  it("refuses Optimize that names a ManagementAuthConfig that does not exist", () => {
    expect(
      attachmentsErrors(
        draft({ optimize: { managementAuthConfigRef: "old" } }),
        {
          summary: summary({ secondaryStorageType: "elasticsearch" }),
          managementAuthConfigs: ["central"],
        },
      ),
    ).toEqual(["ManagementAuthConfig old does not exist. Pick another one."]);
  });

  it("names no ManagementAuthConfig as missing while the kind cannot be read", () => {
    expect(
      attachmentsErrors(
        draft({ optimize: { managementAuthConfigRef: "old" } }),
        {
          summary: summary({ secondaryStorageType: "elasticsearch" }),
          managementAuthConfigs: [],
          unread: ["ManagementAuthConfig"],
        },
      ),
    ).toEqual([]);
  });

  it("refuses a schedule without backup storage", () => {
    expect(
      attachmentsErrors(draft({ backupSchedule: { schedule: "0 2 * * *" } }), {
        summary: summary(),
        managementAuthConfigs: [],
      }),
    ).toEqual(["Back up on a schedule: Needs a backup storage option."]);
  });

  it("refuses a blank schedule", () => {
    expect(
      attachmentsErrors(draft({ backupSchedule: { schedule: "  " } }), {
        summary: summary({ backupStorage: backups }),
        managementAuthConfigs: [],
      }),
    ).toEqual(["Schedule (cron, UTC): Enter a schedule."]);
  });
});

describe("withClaimValue", () => {
  it("sets a value at steps from the root of the claim and keeps the rest of the draft", () => {
    const base = draftWith({ presetRef: "small" });
    const d = withClaimValue(
      base,
      [{ field: "spec" }, { field: "cluster" }, { field: "podLabels" }],
      { a: "1" },
    );
    expect(d.claim.spec).toEqual({
      presetRef: "small",
      cluster: { podLabels: { a: "1" } },
    });
    expect(d.namespace).toBe(base.namespace);
    expect(base.claim.spec).toEqual({ presetRef: "small" });
  });
});

describe("withoutClaimValue", () => {
  it("removes emptied objects and keeps the claim spec", () => {
    const base = draftWith({ presetRef: "small" });
    const steps = [
      { field: "spec" },
      { field: "cluster" },
      { field: "podLabels" },
    ];
    const back = withoutClaimValue(
      withClaimValue(base, steps, { a: "1" }),
      steps,
    );
    expect(back.claim.spec.cluster).toBeUndefined();
    expect(back.claim.spec).toEqual(base.claim.spec);
  });

  it("keeps a spec object when the last field goes", () => {
    expect(
      withoutClaimValue(draftWith({ releaseRef: "x" }), [
        { field: "spec" },
        { field: "releaseRef" },
      ]).claim.spec,
    ).toEqual({});
  });
});

describe("withoutClaimValues", () => {
  const url = ["spec", "cluster", "externalUrl"].map((field) => ({ field }));
  const annotations = ["spec", "cluster", "serviceAccount", "annotations"].map(
    (field) => ({ field }),
  );

  it("removes the value at each path", () => {
    const base = draftWith({
      presetRef: "small",
      cluster: {
        externalUrl: "https://orders.example.com",
        serviceAccount: { annotations: { role: "orders" } },
      },
    });
    expect(withoutClaimValues(base, [url, annotations]).claim.spec).toEqual({
      presetRef: "small",
    });
  });

  it("gives the same draft when it holds no value at the paths", () => {
    const base = draftWith({ presetRef: "small" });
    expect(withoutClaimValues(base, [url, annotations])).toBe(base);
  });
});

describe("draftOptimize", () => {
  it("gives an empty CamundaOptimize of the claim while Optimize is on", () => {
    expect(
      draftOptimize(
        draftWith({
          presetRef: "small",
          attachments: { optimize: { managementAuthConfigRef: "central" } },
        }),
      ),
    ).toEqual({
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaOptimize",
      metadata: { name: "orders", namespace: "team-a" },
      spec: {},
    });
  });

  it("gives nothing while Optimize is off", () => {
    expect(draftOptimize(draftWith({ presetRef: "small" }))).toBeUndefined();
  });
});

describe("clusterCreateChange", () => {
  const deletion: DeletionChoice = {
    defaultPolicy: "Delete",
    keepAllowed: true,
    alwaysDeleted: [],
    policy: "Delete",
  };
  const creating = environment("prod", {
    namespaceCreation: {
      allowed: true,
      requiredLabelKeys: ["team"],
      labels: { tier: "prod" },
    },
  });
  const draft: ClusterDraft = {
    environment: "prod",
    namespace: { kind: "new", name: "team-new", labels: { team: "orders" } },
    claim: { metadata: { name: "orders" }, spec: { presetRef: "small" } },
    storage: { access: [{ section: "access", registration: "gateway" }] },
  };

  it("creates the new namespace with the environment labels, then the claim", () => {
    expect(clusterCreateChange(draft, creating, deletion)).toEqual({
      intent: "Create the cluster orders in team-new",
      apply: [
        {
          apiVersion: "v1",
          kind: "Namespace",
          metadata: {
            name: "team-new",
            labels: {
              tier: "prod",
              team: "orders",
              [ENVIRONMENT_LABEL]: "prod",
            },
          },
        },
        clusterClaimOf(draft, creating, deletion),
      ],
      create: ["Namespace//team-new", "CamundaClusterClaim/team-new/orders"],
    });
  });

  it("creates only the claim in an existing namespace", () => {
    const existing: ClusterDraft = {
      ...draft,
      namespace: { kind: "existing", name: "team-a" },
    };
    expect(clusterCreateChange(existing, creating, deletion)).toEqual({
      intent: "Create the cluster orders in team-a",
      apply: [clusterClaimOf(existing, creating, deletion)],
      create: ["CamundaClusterClaim/team-a/orders"],
    });
  });

  it("writes no environment label without an Environment", () => {
    const change = clusterCreateChange(
      { ...draft, environment: undefined },
      undefined,
      deletion,
    );
    expect(change.apply[0]).toEqual({
      apiVersion: "v1",
      kind: "Namespace",
      metadata: { name: "team-new", labels: { team: "orders" } },
    });
  });
});

describe("Secret references", () => {
  const ref = { name: "orders-oidc", key: "secret" };
  const withSecrets = (namespace: string): ClusterDraft => ({
    ...draftWith({
      presetRef: "small",
      cluster: {
        auth: { clientId: "orders", clientSecretRef: ref },
        zeebe: {
          extraEnv: [
            { name: "A", value: "a" },
            { name: "B", valueFrom: { secretKeyRef: ref } },
            {
              name: "C",
              valueFrom: { configMapKeyRef: { name: "settings", key: "mode" } },
            },
          ],
          extraEnvFrom: [{ secretRef: { name: "orders-env" } }],
        },
      } as unknown as CamundaClusterSpec,
    }),
    namespace: { kind: "existing", name: namespace },
  });

  it("finds each Secret reference of the claim, and nothing else", () => {
    expect(secretReferences(withSecrets("team-a")).map((r) => r.path)).toEqual([
      "auth.clientSecretRef",
      "zeebe.extraEnv[1].valueFrom.secretKeyRef",
      "zeebe.extraEnvFrom[0].secretRef",
    ]);
    expect(secretReferences(draftWith({ presetRef: "small" }))).toEqual([]);
  });

  const read = (namespace: string, error?: string) => ({
    namespace,
    secrets: [
      { name: "orders-oidc", keys: ["secret"] },
      { name: "orders-env", keys: ["A"] },
    ],
    ...(error !== undefined && { error }),
  });
  const pathsOf = (errors: string[]) => errors.map((e) => e.split(": ")[0]);

  it("refuses nothing while each Secret and key exists in the namespace of the draft", () => {
    expect(secretErrors(withSecrets("team-a"), read("team-a"))).toEqual([]);
  });

  it("names a Secret that the namespace does not hold, however the reference got there", () => {
    const errors = secretErrors(withSecrets("team-b"), {
      namespace: "team-b",
      secrets: [{ name: "orders-env", keys: [] }],
    });
    expect(pathsOf(errors)).toEqual([
      "auth.clientSecretRef",
      "zeebe.extraEnv[1].valueFrom.secretKeyRef",
    ]);
    expect(errors[0]).toBe(
      "auth.clientSecretRef: The namespace team-b holds no Secret orders-oidc. Pick a Secret of team-b, or reset the field.",
    );
  });

  it("gives each Secret error as a field error at the claim path of its reference", () => {
    const draft = withSecrets("team-b");
    const missing = {
      namespace: "team-b",
      secrets: [{ name: "orders-env", keys: [] }],
    };
    expect(secretFieldErrors(draft, missing)).toEqual([
      {
        path: "spec.cluster.auth.clientSecretRef",
        message:
          "The namespace team-b holds no Secret orders-oidc. Pick a Secret of team-b, or reset the field.",
      },
      {
        path: "spec.cluster.zeebe.extraEnv[1].valueFrom.secretKeyRef",
        message:
          "The namespace team-b holds no Secret orders-oidc. Pick a Secret of team-b, or reset the field.",
      },
    ]);
    expect(secretFieldErrors(withSecrets("team-a"), read("team-a"))).toEqual(
      [],
    );
  });

  it("names a key that the Secret does not hold", () => {
    const errors = secretErrors(withSecrets("team-b"), {
      namespace: "team-b",
      secrets: [
        { name: "orders-oidc", keys: ["other"] },
        { name: "orders-env", keys: [] },
      ],
    });
    expect(errors).toEqual([
      "auth.clientSecretRef: The Secret orders-oidc of team-b holds no key secret. Pick a key of it, or reset the field.",
      "zeebe.extraEnv[1].valueFrom.secretKeyRef: The Secret orders-oidc of team-b holds no key secret. Pick a key of it, or reset the field.",
    ]);
  });

  it("accepts a reference that the new namespace holds, also when it did not change", () => {
    expect(secretErrors(withSecrets("team-b"), read("team-b"))).toEqual([]);
  });

  it("says that a reference cannot be checked while the Secrets of the namespace are not read", () => {
    expect(secretErrors(withSecrets("team-b"), read("team-a"))).toContain(
      "auth.clientSecretRef: The Secrets of team-b are not read yet, so the fleet manager cannot check this reference.",
    );
    expect(secretErrors(withSecrets("team-b"), undefined)).toHaveLength(3);
    expect(
      secretErrors(withSecrets("team-b"), read("team-b", "forbidden")),
    ).toContain(
      "auth.clientSecretRef: The fleet manager could not read the Secrets of team-b, so it cannot check this reference: forbidden",
    );
  });

  it("checks a reference of an attachment under its attachments path", () => {
    const attached: ClusterDraft = {
      ...draftWith({
        presetRef: "small",
        attachments: {
          optimize: {
            webapp: {
              extraEnv: [{ name: "A", valueFrom: { secretKeyRef: ref } }],
            },
          },
        },
      } as unknown as ClusterDraft["claim"]["spec"]),
      namespace: { kind: "existing", name: "team-b" },
    };
    expect(
      secretErrors(attached, { namespace: "team-b", secrets: [] }),
    ).toEqual([
      "attachments.optimize.webapp.extraEnv[0].valueFrom.secretKeyRef: The namespace team-b holds no Secret orders-oidc. Pick a Secret of team-b, or reset the field.",
    ]);
  });

  it("checks no reference that is optional", () => {
    const optional = withClaimValue(
      {
        ...withSecrets("team-b"),
        namespace: { kind: "new", name: "team-new", labels: {} },
      },
      [
        { field: "spec" },
        { field: "cluster" },
        { field: "zeebe" },
        { field: "extraEnvFrom" },
        { index: 0 },
        { field: "secretRef" },
      ],
      { name: "orders-env", optional: true },
    );
    expect(pathsOf(secretErrors(optional, undefined))).toEqual([
      "auth.clientSecretRef",
      "zeebe.extraEnv[1].valueFrom.secretKeyRef",
    ]);
  });

  it("refuses each reference in a new namespace, which holds no Secret", () => {
    const fresh: ClusterDraft = {
      ...withSecrets("team-a"),
      namespace: { kind: "new", name: "team-new", labels: {} },
    };
    const errors = secretErrors(fresh, read("team-a"));
    expect(errors).toHaveLength(3);
    expect(errors[0]).toBe(
      "auth.clientSecretRef: The new namespace team-new holds no Secret yet. Reset the field.",
    );
  });

  it("refuses nothing without a reference or a namespace", () => {
    expect(secretErrors(draftWith({ presetRef: "small" }), undefined)).toEqual(
      [],
    );
    expect(
      secretErrors(
        { ...withSecrets("team-a"), namespace: undefined },
        undefined,
      ),
    ).toEqual([]);
  });
});

describe("holdsWrittenSpec", () => {
  const sent = {
    presetRef: "small",
    plugins: [{ section: "access", registration: "gateway" }],
    cluster: {
      zeebe: {
        extraEnv: [{ name: "A", valueFrom: { secretKeyRef: { key: "k" } } }],
      },
    },
    attachments: { backupSchedule: { schedule: "@daily", retained: {} } },
  };

  const retained = (value: object) => ({
    ...sent,
    attachments: { backupSchedule: { schedule: "@daily", retained: value } },
  });

  it("accepts the sent spec with the defaults of the API server", () => {
    const live = {
      ...retained({ completed: 7, failed: 3 }),
      cluster: {
        zeebe: {
          extraEnv: [
            { name: "A", valueFrom: { secretKeyRef: { key: "k", name: "" } } },
          ],
        },
      },
    };
    expect(holdsWrittenSpec(live, sent)).toBe(true);
  });

  it("refuses a spec with a field that the write did not send", () => {
    expect(holdsWrittenSpec({ ...sent, suspend: true }, sent)).toBe(false);
    expect(holdsWrittenSpec(retained({ completed: 30, failed: 3 }), sent)).toBe(
      false,
    );
  });

  it("allows no default inside the parameters of a selection, or at another place", () => {
    const withParameters = {
      ...sent,
      plugins: [{ section: "access", registration: "gateway", parameters: {} }],
    };
    expect(
      holdsWrittenSpec(
        {
          ...withParameters,
          plugins: [
            {
              section: "access",
              registration: "gateway",
              parameters: { whenDeleted: "Delete" },
            },
          ],
        },
        withParameters,
      ),
    ).toBe(false);
    expect(holdsWrittenSpec({ ...sent, failed: 3 }, sent)).toBe(false);
  });

  it("refuses a spec with another value or a missing field", () => {
    expect(holdsWrittenSpec({ ...sent, presetRef: "large" }, sent)).toBe(false);
    expect(holdsWrittenSpec({ ...sent, cluster: undefined }, sent)).toBe(false);
    expect(holdsWrittenSpec({ ...sent, plugins: [] }, sent)).toBe(false);
  });

  // The CRD is the source of the defaults. A new default fails here.
  it("knows each default of the claim CRD", () => {
    const crd = parse(
      readFileSync(
        new URL(
          "../../../../config/crd/bases/fleet.camunda.io_camundaclusterclaims.yaml",
          import.meta.url,
        ),
        "utf8",
      ),
    );
    // A step is a field name, or "[]" for a list item or a map value.
    const defaults: { path: string[]; value: unknown }[] = [];
    const walk = (node: unknown, path: string[]) => {
      if (typeof node !== "object" || node === null) return;
      const schema = node as {
        default?: unknown;
        properties?: Record<string, unknown>;
        items?: unknown;
        additionalProperties?: unknown;
      };
      if ("default" in schema) defaults.push({ path, value: schema.default });
      for (const [name, child] of Object.entries(schema.properties ?? {}))
        walk(child, [...path, name]);
      walk(schema.items, [...path, "[]"]);
      walk(schema.additionalProperties, [...path, "{}"]);
    };
    walk(crd.spec.versions[0].schema.openAPIV3Schema.properties.spec, []);
    // at builds the object that holds leaf at the end of path.
    const at = (path: string[], leaf: unknown): unknown => {
      if (path.length === 0) return leaf;
      const [step, ...rest] = path;
      if (step === "[]") return [at(rest, leaf)];
      return { [step === "{}" ? "key" : step]: at(rest, leaf) };
    };
    for (const { path, value } of defaults) {
      const live = at(path, value);
      const sent = at(path.slice(0, -1), {});
      expect(holdsWrittenSpec(live, sent), path.join(".")).toBe(true);
    }
    expect(defaults.length).toBeGreaterThan(0);
  });
});

describe("isWrittenClaim", () => {
  const spec = { presetRef: "small" };
  const started = Date.parse("2026-11-16T12:00:00Z");
  const made = (creationTimestamp?: string) => ({
    metadata: creationTimestamp === undefined ? {} : { creationTimestamp },
    spec,
  });

  it("accepts a claim made after the write started, or a minute before it", () => {
    expect(isWrittenClaim(made("2026-11-16T12:00:05Z"), spec, started)).toBe(
      true,
    );
    expect(isWrittenClaim(made("2026-11-16T11:59:30Z"), spec, started)).toBe(
      true,
    );
  });

  it("refuses a claim made earlier, or with no creation time", () => {
    expect(isWrittenClaim(made("2026-11-16T11:58:00Z"), spec, started)).toBe(
      false,
    );
    expect(isWrittenClaim(made(), spec, started)).toBe(false);
  });

  it("refuses a claim with another spec", () => {
    expect(
      isWrittenClaim(
        { ...made("2026-11-16T12:00:05Z"), spec: { presetRef: "large" } },
        spec,
        started,
      ),
    ).toBe(false);
  });
});
