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

const both =
  (...changes: ((s: Session<Claim>) => Session<Claim>)[]) =>
  (s: Session<Claim>) =>
    changes.reduce((at, c) => c(at), s);

const set =
  (steps: Step[], value: unknown) =>
  (s: Session<Claim>): Session<Claim> => {
    const result = setValue(s, steps, value);
    if (!("session" in result)) throw new Error("refused");
    return result.session;
  };

const HEAD =
  "CAUTION: Wait until backup nightly finishes. This change can restart the brokers, which can fail a running backup.";
const SUSPEND_HEAD =
  "CAUTION: Wait until backup nightly finishes. This change suspends the cluster, which can fail a running backup.";
const MOVED =
  "The backup fails at once if the change gives Zeebe another configuration or Camunda version.";
const ROLLOUT =
  "The backup waits up to 10 minutes for the rollout to finish. It fails if the rollout takes longer, or if Zeebe then runs another configuration or Camunda version.";
const RESUME =
  "The backup waits 10 minutes for the cluster to resume, then fails.";
const AFTER =
  "A backup that starts after the change waits until the rollout is done.";
const AFTER_SUSPEND =
  "A backup that starts after the change waits until the cluster resumes.";
const GITOPS =
  "GitOps applies the change later. If a backup runs then, it can fail.";
const STILL =
  "Backup nightly is running. This change does not roll the brokers, so it does not affect the backup.";
const FINAL =
  "Backup nightly is running. The backup of its Zeebe partitions is finished. A roll of the brokers does not affect the backup.";
const PARKS =
  "Backup nightly is running. The backup waits until the cluster resumes.";
const ANYWAY = {
  label: "Change anyway",
  gitLabel: "Push anyway",
  danger: true,
};

type Expect = (mode: WriteMode) => BackupWarning;
const caution =
  (line: string, head = HEAD, after = AFTER): Expect =>
  (mode) => ({
    kind: "caution",
    lines: [head, line, after, ...(mode === "git" ? [GITOPS] : [])],
    confirm: ANYWAY,
  });
const suspended = caution(RESUME, SUSPEND_HEAD, AFTER_SUSPEND);
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

type Effect = "rolls" | "keeps" | "suspends";

// Each row: the backup, and its result for a change that rolls Zeebe, one
// that does not, and one that suspends the cluster.
const BACKUPS: [string, RollBackupLike[], Record<Effect, Expect>][] = [
  ["no backup", [], { rolls: none, keeps: none, suspends: none }],
  [
    "a pending backup",
    [es({ phase: "Pending" })],
    { rolls: none, keeps: none, suspends: none },
  ],
  [
    "a running Elasticsearch backup",
    [es({ phase: "Running", runtime: { state: "InProgress" } })],
    { rolls: caution(MOVED), keeps: note(STILL), suspends: note(PARKS) },
  ],
  [
    "an Elasticsearch backup with its partitions backed up",
    [es({ phase: "Running", runtime: { state: "Completed" } })],
    { rolls: note(FINAL), keeps: note(STILL), suspends: note(PARKS) },
  ],
  [
    "a running PostgreSQL backup",
    [rdbms({ phase: "Running" })],
    { rolls: caution(ROLLOUT), keeps: note(STILL), suspends: suspended },
  ],
  [
    "a PostgreSQL backup with its Zeebe backup id",
    [rdbms({ phase: "Running", zeebeBackupId: 7 })],
    { rolls: caution(MOVED), keeps: note(STILL), suspends: suspended },
  ],
];

const EMBEDDED_GATEWAY = { gateway: { replicas: 2, mode: "Embedded" } };
const SECTION: Step[] = [
  f("spec"),
  f("plugins"),
  { item: { section: "access", registration: "gateway-access" } },
  f("parameters"),
  f("hostnamePattern"),
];
// Each row: the change, the cluster spec of the claim and the live cluster
// before it, the step and the value, and what it does to Zeebe.
const CHANGES: [string, Record<string, unknown>, Step[], unknown, Effect][] = [
  [
    "the Camunda version",
    {},
    [f("spec"), f("releaseRef")],
    "camunda-8-10-1",
    "rolls",
  ],
  [
    "a zeebe field",
    {},
    cluster("zeebe", "resources"),
    { limits: { cpu: "2" } },
    "rolls",
  ],
  [
    "a cluster-wide field",
    {},
    cluster("scheduling"),
    { nodeSelector: { pool: "b" } },
    "rolls",
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
    "keeps",
  ],
  ["the gateway mode", {}, cluster("gateway", "mode"), "Embedded", "rolls"],
  [
    "an app moving into the brokers",
    { ...EMBEDDED_GATEWAY, operate: { mode: "Standalone" } },
    cluster("operate", "mode"),
    "Embedded",
    "rolls",
  ],
  [
    "an app moving out of the brokers",
    EMBEDDED_GATEWAY,
    cluster("operate", "mode"),
    "Standalone",
    "rolls",
  ],
  [
    "an Operate-only field",
    { operate: { mode: "Standalone" } },
    cluster("operate", "resources"),
    { limits: { cpu: "2" } },
    "keeps",
  ],
  [
    "the deletion policy",
    {},
    [f("spec"), f("deletionPolicy")],
    "Orphan",
    "keeps",
  ],
  ["a new secondary storage", {}, cluster("storageRef"), "other", "rolls"],
  ["a new backup bucket", {}, cluster("backupStorageRef"), "other", "rolls"],
  ["a section parameter", {}, SECTION, "b", "rolls"],
  ["suspend", {}, [f("spec"), f("suspend")], true, "suspends"],
];

describe("backupWarning", () => {
  const rows = CHANGES.flatMap(([change, base, steps, value, effect]) =>
    BACKUPS.flatMap(([backup, list, results]) =>
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
            results[effect](mode),
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

  it("treats a suspend with a roll as a roll for Elasticsearch, and as a suspend for PostgreSQL", () => {
    const i = input(
      {},
      both(
        set([f("spec"), f("suspend")], true),
        set(cluster("zeebe", "resources"), { limits: { cpu: "2" } }),
      ),
    );
    const running = { phase: "Running", runtime: { state: "InProgress" } };
    expect(backupWarning(i, [es(running)], "direct")).toEqual(
      caution(MOVED)("direct"),
    );
    expect(backupWarning(i, [rdbms({ phase: "Running" })], "direct")).toEqual(
      suspended("direct"),
    );
  });

  it("finds a suspend inside a pending change of the whole spec", () => {
    const i = input({}, (session) =>
      set([f("spec")], { ...session.claim.spec, suspend: true })(session),
    );
    const running = { phase: "Running", runtime: { state: "InProgress" } };
    expect(backupWarning(i, [es(running)], "direct")).toEqual(
      note(PARKS)("direct"),
    );
  });

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
      "CAUTION: Wait until backup manual-1 finishes. This change can restart the brokers, which can fail a running backup.",
    );
  });
});
