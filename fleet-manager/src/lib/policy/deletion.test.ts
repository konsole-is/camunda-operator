import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import {
  claimDeletionChoice,
  deletionChoice,
  deletionPolicyOf,
} from "./deletion";
import type { ResolvedPolicy } from "./resolve";

const environment = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "prod" },
  spec: { deletionPolicy: "Orphan" },
} as Environment;

const keeps = {
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginRegistration",
  metadata: { name: "keeps" },
  spec: {
    sections: ["storage"],
    mode: "Optional",
    label: "Keeps",
    deletionPolicies: ["Delete", "Orphan"],
    serviceAccount: { name: "keeps", namespace: "plugins" },
  },
} as PluginRegistration;

const selections = [{ section: "storage", registration: "keeps" }];

describe("deletionChoice", () => {
  it("takes the default of the Environment without a given default", () => {
    expect(
      deletionChoice(undefined, selections, environment, [keeps]),
    ).toMatchObject({ defaultPolicy: "Orphan", policy: "Orphan" });
  });

  it("takes a given default over the default of the Environment", () => {
    expect(
      deletionChoice(undefined, selections, environment, [keeps], [], "Delete"),
    ).toMatchObject({ defaultPolicy: "Delete", policy: "Delete" });
  });
});

describe("deletionPolicyOf", () => {
  const orphan: ResolvedPolicy = { deletionPolicy: "Orphan" };

  it.each([
    ["the policy of the claim wins", "Delete", orphan, "Delete"],
    ["no policy of the claim takes the default", undefined, orphan, "Orphan"],
    ["an empty policy of the claim takes the default", "", orphan, "Orphan"],
    ["no snapshot means Delete", undefined, undefined, "Delete"],
    ["a snapshot without a default means Delete", undefined, {}, "Delete"],
  ] as const)("%s", (_, own, resolved, want) => {
    expect(deletionPolicyOf(own, resolved)).toBe(want);
  });
});

describe("claimDeletionChoice", () => {
  const section = (
    name: string,
    label: string,
    appliesTo: PluginSection["spec"]["appliesTo"] = ["CamundaClusterClaim"],
  ) =>
    ({
      apiVersion: "plugins.camunda.io/v1alpha1",
      kind: "PluginSection",
      metadata: { name },
      spec: { label, selection: "One", appliesTo },
    }) as PluginSection;
  const deletesOnly = {
    ...keeps,
    metadata: { name: "gateway" },
    spec: {
      ...keeps.spec,
      sections: ["access"],
      label: "Gateway",
      deletionPolicies: ["Delete"],
    },
  } as PluginRegistration;
  const sections = [section("storage", "Storage"), section("access", "Access")];
  const claim = (
    spec: {
      deletionPolicy?: "Delete" | "Orphan";
      plugins?: { section: string; registration: string }[];
    } = {},
    resolved?: "Delete" | "Orphan",
  ) => ({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace: "team-a", name: "orders", uid: "uid-1" },
    spec: { plugins: selections, ...spec },
    ...(resolved && {
      status: { resolvedPolicy: { deletionPolicy: resolved } },
    }),
  });
  const choiceOf = (
    c: ReturnType<typeof claim>,
    registrations: PluginRegistration[] = [keeps],
  ) =>
    claimDeletionChoice({
      claim: c,
      environment: { ...environment, spec: { deletionPolicy: "Delete" } },
      sections,
      registrations,
      kind: "CamundaClusterClaim",
    });

  it("the default is the policy that the claim recorded, not the Environment default", () => {
    expect(choiceOf(claim({}, "Orphan"))).toMatchObject({
      defaultPolicy: "Orphan",
      policy: "Orphan",
    });
  });

  it("a claim without a recorded policy defaults to Delete", () => {
    expect(
      claimDeletionChoice({
        claim: claim(),
        environment,
        sections,
        registrations: [keeps],
        kind: "CamundaClusterClaim",
      }),
    ).toMatchObject({ defaultPolicy: "Delete", policy: "Delete" });
  });

  it("the policy of the claim wins over its default", () => {
    expect(
      choiceOf(claim({ deletionPolicy: "Orphan" }, "Delete")),
    ).toMatchObject({ defaultPolicy: "Delete", policy: "Orphan" });
  });

  it("a claim on Orphan with an option that cannot keep stays on Orphan, with Keep refused", () => {
    const c = claim({
      deletionPolicy: "Orphan",
      plugins: [{ section: "access", registration: "gateway" }],
    });
    expect(choiceOf(c, [deletesOnly])).toEqual({
      defaultPolicy: "Delete",
      keepAllowed: false,
      alwaysDeleted: [{ section: "access", label: "Gateway (Access)" }],
      policy: "Orphan",
    });
  });

  it("a plane claim reads the sections offered to a plane", () => {
    const plane = {
      ...claim({ plugins: [{ section: "identity", registration: "gateway" }] }),
      kind: "CamundaManagementClusterClaim",
    };
    const identity = {
      ...deletesOnly,
      spec: { ...deletesOnly.spec, sections: ["identity"] },
    } as PluginRegistration;
    expect(
      claimDeletionChoice({
        claim: plane,
        environment: undefined,
        sections: [
          section("identity", "Identity", ["CamundaManagementClusterClaim"]),
        ],
        registrations: [identity],
        kind: "CamundaManagementClusterClaim",
      }),
    ).toEqual({
      defaultPolicy: "Delete",
      keepAllowed: false,
      alwaysDeleted: [{ section: "identity", label: "Gateway (Identity)" }],
      policy: "Delete",
    });
  });
});
