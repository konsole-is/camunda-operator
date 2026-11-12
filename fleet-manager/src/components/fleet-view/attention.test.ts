import { describe, expect, it } from "vitest";
import type { BackupObject } from "@/components/cluster/backups";
import type { Plane } from "@/components/planes/plane";
import type { Resource } from "@/lib/k8s/kinds";
import { attentionItems } from "./attention";
import type { ClusterRow } from "./model";

const NOW = "2026-11-10T12:00:00Z";
const hoursAgo = (hours: number) =>
  new Date(Date.parse(NOW) - hours * 3_600_000).toISOString();

const row = (name: string, over: Partial<ClusterRow> = {}): ClusterRow => ({
  id: `cluster/team-a/${name}`,
  namespace: "team-a",
  name,
  managed: true,
  components: [],
  sections: [],
  href: `/fleet/clusters/team-a/${name}`,
  ...over,
});

const backup = (
  name: string,
  phase: string,
  { completed, created }: { completed?: string; created?: string },
) =>
  ({
    kind: "LogicalBackupElasticsearch",
    metadata: { namespace: "team-a", name, creationTimestamp: created },
    spec: { clusterRef: { name: "orders" } },
    status: {
      phase,
      ...(completed ? { completionTime: completed } : {}),
      failureMessage: "The snapshot repository is not reachable",
    },
  }) as unknown as BackupObject;

const registration = (
  name: string,
  conditions: {
    type: string;
    status: string;
    reason?: string;
    message?: string;
  }[],
  label?: string,
) =>
  ({
    kind: "PluginRegistration",
    metadata: { name },
    spec: label ? { label } : {},
    status: { conditions },
  }) as unknown as Resource<"PluginRegistration">;

const plane = (name: string, status: "True" | "False"): Plane =>
  ({
    id: `cluster/platform/${name}`,
    namespace: "platform",
    name,
    ready: { status, reason: status === "False" ? "Down" : "Healthy" },
  }) as Plane;

const none = { rows: [], backups: [], registrations: [], planes: [], now: NOW };

describe("attentionItems", () => {
  it("lists a cluster that is not ready with its reason", () => {
    expect(
      attentionItems({
        ...none,
        rows: [
          row("orders", {
            ready: { status: "False", reason: "Down", message: "No broker" },
          }),
          row("billing", { ready: { status: "True", reason: "Healthy" } }),
          row("fresh", { ready: { status: "Unknown", reason: "Creating" } }),
        ],
      }),
    ).toEqual([
      {
        id: "cluster/team-a/orders",
        what: "Cluster",
        name: "team-a/orders",
        reason: "Down",
        message: "No broker",
        href: "/fleet/clusters/team-a/orders",
      },
    ]);
  });

  it("lists a claim without a cluster that is not ready without a link", () => {
    const [item] = attentionItems({
      ...none,
      rows: [
        row("returns", {
          id: "claim/team-a/returns",
          href: undefined,
          ready: { status: "False", reason: "NotAllowed" },
        }),
      ],
    });
    expect(item).toMatchObject({
      name: "team-a/returns",
      reason: "NotAllowed",
    });
    expect(item.href).toBeUndefined();
  });

  it("lists a backup that failed in the last 24 hours and not one of more than 24 hours ago", () => {
    expect(
      attentionItems({
        ...none,
        backups: [
          backup("recent", "Failed", { completed: hoursAgo(2) }),
          backup("created-recently", "Failed", { created: hoursAgo(23) }),
          backup("old", "Failed", { completed: hoursAgo(25) }),
          backup("ok", "Completed", { completed: hoursAgo(1) }),
        ],
      }).map((i) => [i.name, i.href, i.message]),
    ).toEqual([
      [
        "team-a/created-recently",
        "/fleet/clusters/team-a/orders?tab=backups",
        "The snapshot repository is not reachable",
      ],
      [
        "team-a/recent",
        "/fleet/clusters/team-a/orders?tab=backups",
        "The snapshot repository is not reachable",
      ],
    ]);
  });

  it("lists a registration with Accepted or Ready False", () => {
    expect(
      attentionItems({
        ...none,
        registrations: [
          registration(
            "es",
            [{ type: "Accepted", status: "False", reason: "InvalidSpec" }],
            "Elasticsearch",
          ),
          registration("pg", [
            { type: "Accepted", status: "True" },
            { type: "Ready", status: "False", reason: "ControllerDown" },
          ]),
          registration("s3", [
            { type: "Accepted", status: "True" },
            { type: "Ready", status: "True" },
          ]),
        ],
      }),
    ).toEqual([
      {
        id: "registration/es",
        what: "Option",
        name: "Elasticsearch",
        reason: "InvalidSpec",
      },
      {
        id: "registration/pg",
        what: "Option",
        name: "pg",
        reason: "ControllerDown",
      },
    ]);
  });

  it("lists a management plane with Ready False", () => {
    expect(
      attentionItems({
        ...none,
        planes: [plane("plane-a", "False"), plane("plane-b", "True")],
      }),
    ).toEqual([
      {
        id: "plane/cluster/platform/plane-a",
        what: "Management plane",
        name: "platform/plane-a",
        reason: "Down",
      },
    ]);
  });

  it("gives a plane and a cluster of the same namespace and name distinct ids", () => {
    const ids = attentionItems({
      ...none,
      rows: [
        row("plane-a", {
          id: "cluster/platform/plane-a",
          namespace: "platform",
          ready: { status: "False", reason: "Down" },
        }),
      ],
      planes: [plane("plane-a", "False")],
    }).map((i) => i.id);
    expect(new Set(ids).size).toBe(2);
  });

  it("orders clusters, backups, options, then planes", () => {
    expect(
      attentionItems({
        rows: [row("orders", { ready: { status: "False", reason: "Down" } })],
        backups: [backup("recent", "Failed", { completed: hoursAgo(1) })],
        registrations: [
          registration("pg", [{ type: "Ready", status: "False" }]),
        ],
        planes: [plane("plane-a", "False")],
        now: NOW,
      }).map((i) => i.what),
    ).toEqual(["Cluster", "Backup", "Option", "Management plane"]);
  });

  it("lists nothing for a healthy fleet", () => {
    expect(
      attentionItems({
        rows: [row("orders", { ready: { status: "True", reason: "Healthy" } })],
        backups: [backup("ok", "Completed", { completed: hoursAgo(1) })],
        registrations: [
          registration("pg", [{ type: "Ready", status: "True" }]),
        ],
        planes: [plane("plane-a", "True")],
        now: NOW,
      }),
    ).toEqual([]);
  });
});
