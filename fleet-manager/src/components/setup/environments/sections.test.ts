import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { sectionGroups, withSectionPolicy } from "./sections";

const CLUSTER = "CamundaClusterClaim" as const;
const PLANE = "CamundaManagementClusterClaim" as const;

const section = (
  name: string,
  appliesTo: PluginSection["spec"]["appliesTo"],
  extra: Partial<PluginSection["spec"]> = {},
): PluginSection => ({
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginSection",
  metadata: { name },
  spec: { label: name, appliesTo, selection: "One", ...extra },
});

type Condition = { type: string; status: string; message?: string };
const ACCEPTED_READY: Condition[] = [
  { type: "Accepted", status: "True" },
  { type: "Ready", status: "True" },
];

const registration = (
  name: string,
  sections: string[],
  conditions: Condition[] = ACCEPTED_READY,
  extra: Partial<PluginRegistration["spec"]> = {},
): PluginRegistration =>
  ({
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      label: `Label of ${name}`,
      sections,
      mode: "Optional",
      deletionPolicies: ["Delete"],
      serviceAccount: { name: "sa", namespace: "ns" },
      ...extra,
    },
    status: { conditions },
  }) as PluginRegistration;

const rowOf = (
  groups: ReturnType<typeof sectionGroups>,
  group: "Clusters" | "Management planes",
  name: string,
) => groups.find((g) => g.label === group)?.rows.find((r) => r.name === name);

describe("sectionGroups", () => {
  it("groups the sections by the claim kind and puts access in both groups", () => {
    const groups = sectionGroups(
      [
        section("access", [CLUSTER, PLANE]),
        section("backup-storage", [CLUSTER]),
        section("management-database", [PLANE]),
      ],
      [],
      {},
    );
    expect(
      groups.map((g) => [g.label, g.rows.map((r) => r.name)]),
    ).toEqual([
      ["Clusters", ["access", "backup-storage"]],
      ["Management planes", ["access", "management-database"]],
    ]);
  });

  it("orders the rows by order, then by name", () => {
    const groups = sectionGroups(
      [
        section("b", [CLUSTER], { order: 10 }),
        section("a", [CLUSTER], { order: 10 }),
        section("c", [CLUSTER], { order: 5 }),
        section("d", [CLUSTER]),
      ],
      [],
      {},
    );
    expect(groups[0].rows.map((r) => r.name)).toEqual(["d", "c", "a", "b"]);
  });

  it("gives no group for a claim kind without a section", () => {
    const groups = sectionGroups([section("a", [CLUSTER])], [], {});
    expect(groups.map((g) => g.label)).toEqual(["Clusters"]);
  });

  it("reads the policy of the section from the spec", () => {
    const groups = sectionGroups(
      [section("backup-storage", [CLUSTER])],
      [],
      {
        sections: {
          "backup-storage": {
            required: true,
            allowed: ["rustfs"],
            default: "rustfs",
          },
        },
      },
    );
    expect(rowOf(groups, "Clusters", "backup-storage")?.policy).toEqual({
      required: true,
      default: "rustfs",
      allowed: ["rustfs"],
    });
    expect(
      rowOf(sectionGroups([section("x", [CLUSTER])], [], {}), "Clusters", "x")
        ?.policy,
    ).toEqual({ required: false, allowed: [] });
  });

  it("offers only registrations that serve the section and have Accepted=True, by name", () => {
    const groups = sectionGroups(
      [section("backup-storage", [CLUSTER])],
      [
        registration("s3", ["backup-storage"]),
        registration("rustfs", ["backup-storage"]),
        registration("refused", ["backup-storage"], [
          { type: "Accepted", status: "False" },
          { type: "Ready", status: "True" },
        ]),
        registration("gateway", ["access"]),
      ],
      {},
    );
    expect(
      rowOf(groups, "Clusters", "backup-storage")?.options.map(
        (o) => o.registration,
      ),
    ).toEqual(["rustfs", "s3"]);
    expect(
      rowOf(groups, "Clusters", "backup-storage")?.options[0].label,
    ).toBe("Label of rustfs");
  });

  it("notes a registration that is not ready with its Ready message", () => {
    const groups = sectionGroups(
      [section("encryption-key", [CLUSTER])],
      [
        registration("kms", ["encryption-key"], [
          { type: "Accepted", status: "True" },
          {
            type: "Ready",
            status: "False",
            message: "AccessDenied on the key.",
          },
        ]),
        registration("vault", ["encryption-key"], [
          { type: "Accepted", status: "True" },
        ]),
        registration("plain", ["encryption-key"]),
      ],
      {},
    );
    const notes = rowOf(groups, "Clusters", "encryption-key")?.options.map(
      (o) => [o.registration, o.readyNote],
    );
    expect(notes).toEqual([
      ["kms", "Not ready: AccessDenied on the key."],
      ["plain", undefined],
      ["vault", "Not ready yet."],
    ]);
  });

  it("notes a required input from a section without a default", () => {
    const sections = [
      section("storage", [CLUSTER]),
      section("encryption-key", [CLUSTER], { label: "Encryption key" }),
    ];
    const registrations = [
      registration("aws-ebs-encrypted", ["storage"], ACCEPTED_READY, {
        inputs: [
          { name: "keyArn", from: "encryption-key/keyRef", required: true },
        ],
      }),
      registration("kms", ["encryption-key"]),
    ];
    const noDefault: Environment["spec"] = {
      sections: { "encryption-key": { allowed: ["kms"] } },
    };
    expect(
      rowOf(sectionGroups(sections, registrations, noDefault), "Clusters", "storage")
        ?.options[0].inputNotes,
    ).toEqual([
      "Needs an option of Encryption key. Set a default in Encryption key.",
    ]);
    const withDefault: Environment["spec"] = {
      sections: { "encryption-key": { allowed: ["kms"], default: "kms" } },
    };
    expect(
      rowOf(
        sectionGroups(sections, registrations, withDefault),
        "Clusters",
        "storage",
      )?.options[0].inputNotes,
    ).toEqual([]);
  });

  it("hides the empty option of a required section", () => {
    const sections = [
      section("backup-storage", [CLUSTER], { emptyOptionLabel: "No backups" }),
    ];
    expect(
      rowOf(sectionGroups(sections, [], {}), "Clusters", "backup-storage")
        ?.emptyOptionLabel,
    ).toBe("No backups");
    expect(
      rowOf(
        sectionGroups(sections, [], {
          sections: { "backup-storage": { required: true } },
        }),
        "Clusters",
        "backup-storage",
      )?.emptyOptionLabel,
    ).toBeUndefined();
  });

  it("lists an allowed name that is not offered as missing", () => {
    const groups = sectionGroups(
      [section("backup-storage", [CLUSTER])],
      [
        registration("rustfs", ["backup-storage"]),
        registration("refused", ["backup-storage"], [
          { type: "Accepted", status: "False" },
        ]),
      ],
      {
        sections: {
          "backup-storage": { allowed: ["rustfs", "refused", "gone"] },
        },
      },
    );
    expect(rowOf(groups, "Clusters", "backup-storage")?.missing).toEqual([
      "refused",
      "gone",
    ]);
  });
});

describe("withSectionPolicy", () => {
  it("sets the policy of the section and keeps the other sections", () => {
    const spec: Environment["spec"] = {
      displayName: "Staging",
      sections: { access: { allowed: ["gateway"] } },
    };
    expect(
      withSectionPolicy(spec, "backup-storage", {
        required: true,
        default: "rustfs",
        allowed: ["rustfs"],
      }),
    ).toEqual({
      displayName: "Staging",
      sections: {
        access: { allowed: ["gateway"] },
        "backup-storage": {
          required: true,
          default: "rustfs",
          allowed: ["rustfs"],
        },
      },
    });
  });

  it("leaves out required false, an empty default, and an empty allowed list", () => {
    expect(
      withSectionPolicy({}, "backup-storage", {
        required: false,
        default: "",
        allowed: ["rustfs"],
      }),
    ).toEqual({ sections: { "backup-storage": { allowed: ["rustfs"] } } });
  });

  it("removes the entry of an empty policy, and spec.sections without entries", () => {
    expect(
      withSectionPolicy(
        { displayName: "Staging", sections: { access: { allowed: ["a"] } } },
        "access",
        { required: false, allowed: [] },
      ),
    ).toEqual({ displayName: "Staging" });
  });

  it("does not change spec", () => {
    const spec: Environment["spec"] = { sections: { access: { allowed: ["a"] } } };
    withSectionPolicy(spec, "access", { required: true, allowed: [] });
    expect(spec).toEqual({ sections: { access: { allowed: ["a"] } } });
  });
});
