import { describe, expect, it } from "vitest";
import type { Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import type { RollBackupLike } from "@/lib/policy/backups";
import { openSession, setValue, type Session } from "@/lib/session";
import type { WriteMode } from "@/lib/write/types";
import { backupWarning, type BackupWarning } from "./backup-warning";
import type { ClusterChangeInput } from "./impacts";

type Claim = Resource<"CamundaClusterClaim">;
const f = (field: string): Step => ({ field });
const cluster = (...names: string[]): Step[] => [
  f("spec"),
  f("cluster"),
  ...names.map(f),
];

const REFS = { presetRef: "medium", releaseRef: "camunda-8-9-5" };
const SPEC = {
  zeebe: { replicas: 3, storageSize: "32Gi" },
  gateway: { replicas: 2 },
};

function input(
  base: Record<string, unknown>,
  change: (s: Session<Claim>) => Session<Claim>,
): ClusterChangeInput {
  const claim = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { name: "orders", namespace: "team-a", resourceVersion: "41" },
    spec: {
      ...REFS,
      cluster: { ...SPEC, ...base },
      plugins: [
        {
          section: "access",
          registration: "gateway-access",
          parameters: { hostnamePattern: "a" },
        },
      ],
    },
  } as unknown as Claim;
  const s = openSession({
    claim,
    resource: { metadata: { labels: { [CLAIM_LABEL]: "orders" } } },
    locks: () => undefined,
  })!;
  return {
    session: change(s),
    live: {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaCluster",
      metadata: {
        name: "orders",
        namespace: "team-a",
        labels: { [CLAIM_LABEL]: "orders" },
      },
      spec: { ...REFS, ...SPEC, ...base },
    } as unknown as Resource<"CamundaCluster">,
    presets: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaClusterPreset",
        metadata: { name: "medium" },
        spec: { cluster: { tasklist: { replicas: 1 } } },
      } as unknown as Resource<"CamundaClusterPreset">,
    ],
    releases: [
      release("camunda-8-9-5", "8.9.5"),
      release("camunda-8-10-1", "8.10.1"),
    ],
    sections: [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginSection",
        metadata: { name: "access" },
        spec: {
          label: "Access",
          slot: "externalAccess",
          appliesTo: ["CamundaClusterClaim"],
          selection: "One",
        },
      } as unknown as Resource<"PluginSection">,
    ],
  };
}

function release(name: string, version: string): Resource<"CamundaRelease"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaRelease",
    metadata: { name },
    spec: { version },
  } as unknown as Resource<"CamundaRelease">;
}

const set =
  (steps: Step[], value: unknown) =>
  (s: Session<Claim>): Session<Claim> => {
    const result = setValue(s, steps, value);
    if (!("session" in result)) throw new Error("refused");
    return result.session;
  };

const HEAD =
  "CAUTION: Wait until backup nightly finishes. This change rolls the brokers, and a roll fails a running backup.";
const AT_ONCE = "The backup fails at once.";
const WAITS =
  "The backup waits 10 minutes for the change to be undone, then fails.";
const AFTER =
  "A backup that starts after the change waits until the rollout is done.";
const GITOPS =
  "GitOps applies the change later. If a backup runs then, it fails.";
const STILL =
  "Backup nightly is running. This change does not roll the brokers, so it does not affect the backup.";
const FINAL =
  "Backup nightly is running. The backup of its Zeebe partitions is finished. A roll of the brokers does not affect the backup.";
const ANYWAY = {
  label: "Change anyway",
  gitLabel: "Push anyway",
  danger: true,
};

type Expect = (mode: WriteMode) => BackupWarning;
const caution =
  (line: string): Expect =>
  (mode) => ({
    kind: "caution",
    lines: [HEAD, line, AFTER, ...(mode === "git" ? [GITOPS] : [])],
    confirm: ANYWAY,
  });
const note =
  (text: string): Expect =>
  () => ({ kind: "note", note: text });
const none: Expect = () => ({ kind: "none" });

const es = (status: object): RollBackupLike =>
  ({
    kind: "LogicalBackupElasticsearch",
    metadata: { namespace: "team-a", name: "nightly" },
    spec: { clusterRef: { name: "orders" } },
    status,
  }) as RollBackupLike;
const rdbms = (status: object): RollBackupLike => ({
  ...es(status),
  kind: "LogicalBackupRDBMS",
});

// Each row: the backup, its result when the change rolls Zeebe, and its
// result when it does not.
const BACKUPS: [string, RollBackupLike[], Expect, Expect][] = [
  ["no backup", [], none, none],
  ["a pending backup", [es({ phase: "Pending" })], none, none],
  [
    "a running Elasticsearch backup",
    [es({ phase: "Running", runtime: { state: "InProgress" } })],
    caution(AT_ONCE),
    note(STILL),
  ],
  [
    "an Elasticsearch backup with its partitions backed up",
    [es({ phase: "Running", runtime: { state: "Completed" } })],
    note(FINAL),
    note(STILL),
  ],
  [
    "a running PostgreSQL backup",
    [rdbms({ phase: "Running" })],
    caution(WAITS),
    note(STILL),
  ],
  [
    "a PostgreSQL backup with its Zeebe backup id",
    [rdbms({ phase: "Running", zeebeBackupId: 7 })],
    caution(AT_ONCE),
    note(STILL),
  ],
];

const EMBEDDED_GATEWAY = { gateway: { replicas: 2, mode: "Embedded" } };
// Each row: the change, the cluster spec of the claim and the live cluster
// before it, the step and the value, and whether it rolls Zeebe.
const CHANGES: [string, Record<string, unknown>, Step[], unknown, boolean][] = [
  [
    "the Camunda version",
    {},
    [f("spec"), f("releaseRef")],
    "camunda-8-10-1",
    true,
  ],
  [
    "a zeebe field",
    {},
    cluster("zeebe", "resources"),
    { limits: { cpu: "2" } },
    true,
  ],
  [
    "a cluster-wide field",
    {},
    cluster("scheduling"),
    { nodeSelector: { pool: "b" } },
    true,
  ],
  [
    "a cluster-wide field that the brokers set themselves",
    {
      zeebe: {
        replicas: 3,
        storageSize: "32Gi",
        scheduling: { nodeSelector: { pool: "z" } },
      },
    },
    cluster("scheduling"),
    { nodeSelector: { pool: "b" } },
    false,
  ],
  ["the gateway mode", {}, cluster("gateway", "mode"), "Embedded", true],
  [
    "an app moving into the brokers",
    { ...EMBEDDED_GATEWAY, operate: { mode: "Standalone" } },
    cluster("operate", "mode"),
    "Embedded",
    true,
  ],
  [
    "an app moving out of the brokers",
    EMBEDDED_GATEWAY,
    cluster("operate", "mode"),
    "Standalone",
    true,
  ],
  [
    "an Operate-only field",
    { operate: { mode: "Standalone" } },
    cluster("operate", "resources"),
    { limits: { cpu: "2" } },
    false,
  ],
  [
    "the deletion policy",
    {},
    [f("spec"), f("deletionPolicy")],
    "Orphan",
    false,
  ],
];

describe("backupWarning", () => {
  const rows = CHANGES.flatMap(([change, base, steps, value, rolls]) =>
    BACKUPS.flatMap(([backup, list, ifRolls, ifNot]) =>
      (["direct", "git"] as const).map(
        (mode) =>
          [
            change,
            backup,
            mode,
            base,
            steps,
            value,
            list,
            (rolls ? ifRolls : ifNot)(mode),
          ] as const,
      ),
    ),
  );

  it.each(rows)(
    "%s with %s in %s mode",
    (_c, _b, mode, base, steps, value, list, expected) => {
      expect(backupWarning(input(base, set(steps, value)), list, mode)).toEqual(
        expected,
      );
    },
  );

  it("keeps the caution for a section change after the partition backup is final", () => {
    const i = input(
      {},
      set(
        [
          f("spec"),
          f("plugins"),
          { item: { section: "access", registration: "gateway-access" } },
          f("parameters"),
          f("hostnamePattern"),
        ],
        "b",
      ),
    );
    expect(
      backupWarning(
        i,
        [es({ phase: "Running", runtime: { state: "Completed" } })],
        "direct",
      ),
    ).toEqual(caution(AT_ONCE)("direct"));
  });

  it.each([
    ["storageRef", "other-storage"],
    ["backupStorageRef", "other-bucket"],
  ])(
    "keeps the caution for a new %s after the partition backup is final",
    (field, value) => {
      const i = input({}, set(cluster(field), value));
      expect(
        backupWarning(
          i,
          [es({ phase: "Running", runtime: { state: "Completed" } })],
          "direct",
        ),
      ).toEqual(caution(AT_ONCE)("direct"));
    },
  );

  it("names the backup that runs", () => {
    const list = [
      {
        ...rdbms({ phase: "Running" }),
        metadata: { namespace: "team-a", name: "manual-1" },
      },
    ];
    const w = backupWarning(
      input({}, set(cluster("zeebe", "resources"), { limits: { cpu: "2" } })),
      list,
      "direct",
    );
    expect(w.kind === "caution" && w.lines[0]).toBe(
      "CAUTION: Wait until backup manual-1 finishes. This change rolls the brokers, and a roll fails a running backup.",
    );
  });
});
