import { describe, expect, it } from "vitest";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { CamundaClusterPresetSpec } from "@/generated/crds/camundaclusterpreset";
import type { CamundaReleaseSpec } from "@/generated/crds/camundarelease";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster, type ManagedFieldsEntry } from "@/lib/merge";
import { loadMergeFixtures } from "../../../test/merge/fixtures";
import { clusterFields, type ClusterFieldsInput } from "./cluster";
import { lockLabel, stateLabel, stateNote } from "./labels";
import type { ClusterField } from "./types";

const CLAIM = "CamundaClusterClaim/cluster/0b5c7a2e";
const EARLY = "2026-10-01T00:00:00Z";
const LATE = "2026-10-02T00:00:00Z";

type Json = Record<string, unknown>;

// treeOf writes the fieldsV1 tree that owns every leaf of value.
function treeOf(value: unknown): Json {
  if (typeof value !== "object" || value === null || Array.isArray(value))
    return {};
  return Object.fromEntries(
    Object.entries(value).map(([k, v]) => [`f:${k}`, treeOf(v)]),
  );
}

// managedFieldsOf writes one entry of manager that owns each path, given as
// the JSON field names from the root.
function managedFieldsOf(
  manager: string,
  paths: string[][],
  time = EARLY,
): ManagedFieldsEntry {
  const tree: Json = {};
  for (const names of paths) {
    let at = tree;
    for (const name of names) {
      const key = `f:${name}`;
      at[key] ??= {};
      at = at[key] as Json;
    }
  }
  return { manager, time, fieldsV1: tree };
}

function owns(manager: string, object: Json, time = EARLY): ManagedFieldsEntry {
  return { manager, time, fieldsV1: treeOf(object) };
}

function cluster(
  spec: Json,
  managedFields: ManagedFieldsEntry[],
  labelled = true,
): Resource<"CamundaCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: {
      name: "orders",
      namespace: "team-a",
      labels: labelled ? { [CLAIM_LABEL]: "orders" } : {},
      managedFields,
    },
    spec,
  } as unknown as Resource<"CamundaCluster">;
}

function claim(
  spec: Json,
  plugins: [string, string][] = [],
): Resource<"CamundaClusterClaim"> {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { name: "orders", namespace: "team-a" },
    spec: {
      presetRef: "medium",
      ...spec,
      plugins: plugins.map(([section, registration]) => ({
        section,
        registration,
      })),
    },
  } as unknown as Resource<"CamundaClusterClaim">;
}

function section(
  name: string,
  label: string,
  slot?: string,
): Resource<"PluginSection"> {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name },
    spec: {
      label,
      slot,
      appliesTo: ["CamundaClusterClaim"],
      selection: "One",
    },
  } as unknown as Resource<"PluginSection">;
}

// The core sections of the contract, with their labels and slots.
const SECTIONS = [
  section("cloud-identity", "Cloud identity", "workloadIdentity"),
  section("encryption-key", "Encryption key"),
  section("volume-encryption", "Volume encryption", "volumeEncryption"),
  section("backup-storage", "Backup storage", "backupStorage"),
  section("secondary-storage", "Secondary storage", "secondaryStorage"),
  section("access", "Access", "externalAccess"),
  section(
    "connectors-permissions",
    "Connectors permissions",
    "connectorsIdentity",
  ),
];

type Case = {
  spec: Json;
  managedFields?: ManagedFieldsEntry[];
  preset?: Json;
  release?: Json;
  claim?: Resource<"CamundaClusterClaim">;
  labelled?: boolean;
} & Partial<Pick<ClusterFieldsInput, "optimize" | "backupSchedule">>;

function fieldsOf(c: Case): ClusterField[] {
  return clusterFields({
    cluster: cluster(c.spec, c.managedFields ?? [], c.labelled ?? true),
    merged: mergeCluster(
      c.preset as CamundaClusterPresetSpec | undefined,
      c.release as CamundaReleaseSpec | undefined,
      c.spec as CamundaClusterSpec,
    ),
    claim: c.claim,
    sections: SECTIONS,
    registrations: [],
    optimize: c.optimize,
    backupSchedule: c.backupSchedule,
  });
}

function at(
  fields: ClusterField[],
  path: string,
  kind = "CamundaCluster",
): ClusterField {
  const field = fields.find((f) => f.key === `${kind}:${path}`);
  expect(field, `a field at ${kind}:${path}`).toBeDefined();
  return field!;
}

function view(field: ClusterField) {
  return {
    value: field.value,
    state: stateLabel(field.state),
    lock: field.lock && lockLabel(field.lock),
  };
}

const fixtures = loadMergeFixtures("camundacluster");
function fixture(name: string) {
  const found = fixtures.find((f) => f.file.endsWith(`/${name}`));
  if (!found) throw new Error(`no merge fixture ${name}`);
  return found;
}

describe("clusterFields on the merge fixtures", () => {
  it("the version and the Connectors version are From release and locked by the release", () => {
    const f = fixture("the-version-comes-from-the-release.json");
    const fields = fieldsOf({
      spec: f.spec,
      preset: f.preset,
      release: f.release,
      claim: claim({}),
    });
    expect(view(at(fields, "spec.version"))).toEqual({
      value: "8.9.4",
      state: "From release",
      lock: "Set by the release",
    });
    expect(view(at(fields, "spec.connectors.version"))).toEqual({
      value: "8.9.7",
      state: "From release",
      lock: "Set by the release",
    });
    expect(view(at(fields, "spec.connectors.enabled"))).toEqual({
      value: true,
      state: "From preset",
      lock: undefined,
    });
  });

  it("a value of the preset alone is From preset", () => {
    const f = fixture("unset-scalars-inherit-the-preset.json");
    const fields = fieldsOf({
      spec: f.spec,
      preset: f.preset,
      claim: claim({}),
    });
    expect(fields.length).toBeGreaterThan(0);
    for (const field of fields) {
      expect(stateLabel(field.state), field.path).toBe("From preset");
      expect(field.lock, field.path).toBeUndefined();
    }
  });

  it("a value that the claim sets is Changed in the claim", () => {
    const f = fixture("scalars-replace-the-preset-value.json");
    const field = at(
      fieldsOf({
        spec: f.spec,
        preset: f.preset,
        managedFields: [owns(CLAIM, { spec: f.spec })],
        claim: claim({ cluster: f.spec }),
      }),
      "spec.zeebe.storageSize",
    );
    expect(view(field)).toEqual({
      value: "64Gi",
      state: "Changed in the claim",
      lock: undefined,
    });
    expect(field.claimPath).toBe("spec.cluster.zeebe.storageSize");
  });

  it("the instance fields get their locks", () => {
    const f = fixture("instance-bound-fields-come-from-the-cluster.json");
    const fields = fieldsOf({
      spec: f.spec,
      preset: f.preset,
      managedFields: [owns(CLAIM, { spec: f.spec })],
      claim: claim(
        {
          presetRef: "standard",
          releaseRef: "camunda-8-9",
          platformConfigRef: "my-platform-config",
        },
        [
          ["secondary-storage", "elasticsearch"],
          ["access", "gateway-access"],
        ],
      ),
    });
    expect(view(at(fields, "spec.storageRef"))).toEqual({
      value: "my-storage",
      state: "Filled by Secondary storage",
      lock: "Filled by Secondary storage",
    });
    expect(stateLabel(at(fields, "spec.externalUrl").state)).toBe(
      "Filled by Access",
    );
    expect(lockLabel(at(fields, "spec.backupStorageRef").lock!)).toBe(
      "Set by a section",
    );
    for (const path of [
      "spec.serviceAccount.name",
      "spec.connectors.serviceAccount.name",
    ]) {
      expect(view(at(fields, path)).state, path).toBe("Changed in the claim");
      expect(view(at(fields, path)).lock, path).toBe(
        "Fixed by the claim controller",
      );
    }
    const preset = at(fields, "spec.presetRef");
    expect(stateLabel(preset.state)).toBe("Changed in the claim");
    expect(preset.claimPath).toBe("spec.presetRef");
  });

  it("splits the serviceAccount block into its fields", () => {
    const f = fixture("instance-bound-fields-come-from-the-cluster.json");
    const spec = {
      ...f.spec,
      serviceAccount: {
        name: "my-cluster",
        annotations: { a: "b" },
        create: true,
      },
    };
    const paths = fieldsOf({ spec, preset: f.preset, claim: claim({}) }).map(
      (field) => field.path,
    );
    expect(paths).toEqual(
      expect.arrayContaining([
        "spec.serviceAccount.name",
        "spec.serviceAccount.annotations",
        "spec.serviceAccount.create",
      ]),
    );
    expect(paths).not.toContain("spec.serviceAccount");
  });

  it("keeps a slot map as one field", () => {
    const f = fixture("connectors-service-account-merges-by-field.json");
    const fields = fieldsOf({
      spec: f.spec,
      preset: f.preset,
      claim: claim({}),
    });
    expect(
      at(fields, "spec.connectors.serviceAccount.annotations").value,
    ).toEqual({ shared: "cluster", team: "preset" });
    expect(
      fields.filter((field) =>
        [
          "spec.connectors.serviceAccount.annotations.shared",
          "spec.connectors.serviceAccount.annotations.team",
        ].includes(field.path),
      ),
    ).toEqual([]);
  });
});

describe("clusterFields", () => {
  it("a slot fills a field that the claim also sets", () => {
    const field = at(
      fieldsOf({
        spec: { zeebe: { storageClassName: "encrypted" } },
        managedFields: [
          managedFieldsOf(CLAIM, [["spec", "zeebe", "storageClassName"]]),
        ],
        claim: claim({ cluster: { zeebe: { storageClassName: "fast" } } }, [
          ["volume-encryption", "aws-ebs-kms"],
        ]),
      }),
      "spec.zeebe.storageClassName",
    );
    expect(view(field)).toEqual({
      value: "encrypted",
      state: "Filled by Volume encryption",
      lock: "Filled by Volume encryption",
    });
    expect(field.claimPath).toBe(
      "spec.plugins[section=volume-encryption,registration=aws-ebs-kms]",
    );
  });

  it("shows a slot field of a selected section before it has a value", () => {
    const field = at(
      fieldsOf({
        spec: { zeebe: { replicas: 3 } },
        claim: claim({}, [["volume-encryption", "x"]]),
      }),
      "spec.zeebe.storageClassName",
    );
    expect(view(field)).toEqual({
      value: undefined,
      state: "Filled by Volume encryption",
      lock: "Filled by Volume encryption",
    });
    expect(field.group).toBe("zeebe");
    expect(field.label).toBe("storageClassName");
  });

  it("a field of another manager is Set by that manager", () => {
    const field = at(
      fieldsOf({
        spec: { zeebe: { replicas: 3, storageSize: "64Gi" } },
        managedFields: [
          managedFieldsOf(CLAIM, [
            ["spec", "zeebe", "replicas"],
            ["spec", "zeebe", "storageSize"],
          ]),
          managedFieldsOf(
            "pvc-autoresizer",
            [["spec", "zeebe", "storageSize"]],
            LATE,
          ),
        ],
        claim: claim({ cluster: { zeebe: { replicas: 3 } } }),
      }),
      "spec.zeebe.storageSize",
    );
    expect(view(field)).toEqual({
      value: "64Gi",
      state: "Set by pvc-autoresizer",
      lock: undefined,
    });
    expect(stateNote(field.state)).toBe(
      "Setting it in the claim takes it over.",
    );
  });

  it("a manager that starts with CamundaClusterClaim/ shows as the claim", () => {
    const field = at(
      fieldsOf({
        spec: { zeebe: { replicas: 3 } },
        managedFields: [
          managedFieldsOf(CLAIM, [["spec", "zeebe", "replicas"]]),
        ],
        claim: claim({}),
      }),
      "spec.zeebe.replicas",
    );
    expect(stateLabel(field.state)).toBe("Changed in the claim");
  });

  it("the app manager is not shown as a writer on a managed cluster", () => {
    const field = at(
      fieldsOf({
        spec: { zeebe: { replicas: 3 } },
        managedFields: [
          managedFieldsOf("fleet-manager", [["spec", "zeebe", "replicas"]]),
        ],
        claim: claim({ cluster: { zeebe: { replicas: 3 } } }),
      }),
      "spec.zeebe.replicas",
    );
    expect(stateLabel(field.state)).toBe("Changed in the claim");
  });

  it("With a Connectors permissions selection, only the Connectors annotations are filled", () => {
    const fields = fieldsOf({
      spec: {
        serviceAccount: { annotations: { a: "b" } },
        connectors: { serviceAccount: { annotations: { c: "d" } } },
      },
      claim: claim({}, [["connectors-permissions", "aws-connectors-role"]]),
    });
    expect(
      stateLabel(
        at(fields, "spec.connectors.serviceAccount.annotations").state,
      ),
    ).toBe("Filled by Connectors permissions");
    const workload = at(fields, "spec.serviceAccount.annotations");
    expect(workload.state.kind).not.toBe("filled");
    expect(workload.lock).toBeUndefined();
  });

  it("every field of an unmanaged cluster is locked as not managed", () => {
    const f = fixture("scalars-replace-the-preset-value.json");
    const fields = fieldsOf({
      spec: f.spec,
      preset: f.preset,
      managedFields: [owns("kubectl-client-side-apply", { spec: f.spec })],
      claim: claim({ cluster: f.spec }, [["volume-encryption", "x"]]),
      labelled: false,
    });
    expect(fields.length).toBeGreaterThan(0);
    for (const field of fields) {
      expect(lockLabel(field.lock!), field.path).toBe(
        "Not managed by the fleet manager",
      );
      expect(field.state.kind, field.path).not.toBe("claim");
      expect(field.state.kind, field.path).not.toBe("filled");
      expect(field.claimPath, field.path).toBeUndefined();
    }
    const size = at(fields, "spec.zeebe.storageSize");
    expect(stateLabel(size.state)).toBe("Set by kubectl-client-side-apply");
    expect(stateNote(size.state)).toBeUndefined();
  });

  it("treats a labelled cluster without its claim as managed", () => {
    const f = fixture("the-version-comes-from-the-release.json");
    const fields = fieldsOf({
      spec: f.spec,
      preset: f.preset,
      release: f.release,
    });
    expect(lockLabel(at(fields, "spec.version").lock!)).toBe(
      "Set by the release",
    );
    expect(fields.some((field) => field.lock?.reason === "notManaged")).toBe(
      false,
    );
  });

  it("the Optimize fields follow the attachment of the claim", () => {
    const spec = {
      externalUrl: "https://optimize.example.com",
      version: "8.9.4",
      clusterRef: { name: "orders" },
      managementAuthRef: "auth",
      webapp: { replicas: 2 },
    };
    const fields = fieldsOf({
      spec: {},
      claim: claim(
        {
          attachments: {
            optimize: {
              managementAuthConfigRef: "auth",
              webapp: { replicas: 2 },
            },
          },
        },
        [["access", "gateway-access"]],
      ),
      optimize: {
        kind: "CamundaOptimize",
        metadata: { name: "orders", managedFields: [owns(CLAIM, { spec })] },
        spec,
      } as unknown as Resource<"CamundaOptimize">,
    });
    const field = (path: string) => at(fields, path, "CamundaOptimize");
    expect(stateLabel(field("spec.externalUrl").state)).toBe(
      "Filled by Access",
    );
    expect(view(field("spec.version"))).toEqual({
      value: "8.9.4",
      state: "From release",
      lock: "Set by the release",
    });
    expect(lockLabel(field("spec.clusterRef.name").lock!)).toBe(
      "Fixed by the claim controller",
    );
    expect(stateLabel(field("spec.managementAuthRef").state)).toBe(
      "Changed in the claim",
    );
    expect(field("spec.managementAuthRef").claimPath).toBe(
      "spec.attachments.optimize.managementAuthConfigRef",
    );
    expect(stateLabel(field("spec.webapp.replicas").state)).toBe(
      "Changed in the claim",
    );
    expect(field("spec.webapp.replicas").group).toBe("optimize");
  });

  it("the backup schedule fields follow the attachment of the claim", () => {
    const spec = { clusterRef: { name: "orders" }, schedule: "0 2 * * *" };
    const fields = fieldsOf({
      spec: {},
      claim: claim({
        attachments: { backupSchedule: { schedule: "0 2 * * *" } },
      }),
      backupSchedule: {
        kind: "BackupSchedule",
        metadata: { name: "orders", managedFields: [owns(CLAIM, { spec })] },
        spec,
      } as unknown as Resource<"BackupSchedule">,
    });
    const schedule = at(fields, "spec.schedule", "BackupSchedule");
    expect(stateLabel(schedule.state)).toBe("Changed in the claim");
    expect(schedule.claimPath).toBe("spec.attachments.backupSchedule.schedule");
    expect(
      lockLabel(at(fields, "spec.clusterRef.name", "BackupSchedule").lock!),
    ).toBe("Fixed by the claim controller");
  });

  it("groups and labels each field by its component", () => {
    const fields = fieldsOf({
      spec: {
        zeebe: { storageSize: "64Gi" },
        extraEnv: [{ name: "TZ", value: "UTC" }],
      },
      claim: claim({}),
    });
    const size = at(fields, "spec.zeebe.storageSize");
    expect([size.group, size.label]).toEqual(["zeebe", "storageSize"]);
    expect(size.docNames).toEqual(["spec", "zeebe", "storageSize"]);
    const tz = at(fields, "spec.extraEnv[name=TZ]");
    expect([tz.group, tz.label]).toEqual(["cluster", "extraEnv[name=TZ]"]);
  });
});
