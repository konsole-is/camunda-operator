import { describe, expect, it } from "vitest";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { clusterProgressRows } from "./progress";

const sections = [
  { metadata: { name: "access" }, spec: { label: "Access", selection: "One" } },
  {
    metadata: { name: "backup-storage" },
    spec: { label: "Backup storage", selection: "One" },
  },
] as PluginSection[];

const registrations = [
  { metadata: { name: "gateway" }, spec: { label: "Gateway" } },
  { metadata: { name: "s3" }, spec: {} },
] as PluginRegistration[];

const access = { section: "access", registration: "gateway" };
const backup = { section: "backup-storage", registration: "s3" };

const condition = (
  type: string,
  status: "True" | "False" | "Unknown",
  reason = "Test",
  message = "",
) => ({
  type,
  status,
  reason,
  message,
  observedGeneration: 1,
  lastTransitionTime: "2026-11-16T00:00:00Z",
});

const claimOf = (plugins: object[], status?: CamundaClusterClaim["status"]) =>
  ({
    metadata: { namespace: "team-a", name: "orders", generation: 1 },
    spec: { presetRef: "small", plugins },
    ...(status && { status }),
  }) as unknown as CamundaClusterClaim;

describe("clusterProgressRows", () => {
  it("shows each selection and the three claim conditions pending before the status", () => {
    const rows = clusterProgressRows(
      claimOf([access, backup]),
      sections,
      registrations,
      [],
    );
    expect(rows.map((r) => [r.id, r.label, r.detail])).toEqual([
      ["section:access/gateway", "Access", "Gateway"],
      ["section:backup-storage/s3", "Backup storage", "s3"],
      ["requests", "Requests", undefined],
      ["cluster", "Cluster", undefined],
      ["attachments", "Attachments", undefined],
    ]);
    expect(rows.flatMap((r) => r.checks.map((c) => c.condition))).toEqual(
      rows.flatMap((r) => r.checks.map(() => undefined)),
    );
    expect(rows[0].checks.map((c) => c.label)).toEqual(["Outputs", "Ready"]);
  });

  it("shows the request state and the summary headline of each request", () => {
    const request = {
      metadata: { namespace: "team-a", name: "orders-access" },
      status: {
        summary: [
          { label: "Host", value: "orders.example.com", severity: "Info" },
          { label: "Other", value: "x" },
        ],
      },
    } as unknown as PluginRequest;
    const [row] = clusterProgressRows(
      claimOf([access], {
        requests: [
          {
            section: "access",
            registration: "gateway",
            name: "orders-access",
            outputsReady: "True",
            ready: "False",
          },
        ],
      }),
      sections,
      registrations,
      [request],
    );
    expect(row).toEqual({
      id: "section:access/gateway",
      label: "Access",
      detail: "Gateway, orders-access",
      summary: "Host: orders.example.com",
      checks: [
        { label: "Outputs", condition: { status: "True" } },
        { label: "Ready", condition: { status: "False" } },
      ],
    });
  });

  it("takes no summary from a request of another namespace", () => {
    const elsewhere = {
      metadata: { namespace: "team-b", name: "orders-access" },
      status: { summary: [{ label: "Host", value: "b.example.com" }] },
    } as unknown as PluginRequest;
    const [row] = clusterProgressRows(
      claimOf([access], {
        requests: [
          { section: "access", registration: "gateway", name: "orders-access" },
        ],
      }),
      sections,
      registrations,
      [elsewhere],
    );
    expect(row.summary).toBeUndefined();
  });

  it("shows the reason and the message of the cluster", () => {
    const message =
      'StorageAlreadyAttached: CamundaCluster "fleet-smoke/camundacluster-sample" already holds the backend';
    const rows = clusterProgressRows(
      claimOf([access], {
        conditions: [
          condition("RequestsReady", "True", "Ready", "All requests are ready"),
          condition("ClusterReady", "False", "StorageAlreadyAttached", message),
        ],
      }),
      sections,
      registrations,
      [],
    );
    const cluster = rows.find((r) => r.id === "cluster")!;
    expect(cluster.summary).toBe(message);
    expect(cluster.checks).toEqual([
      {
        label: "Ready",
        condition: {
          status: "False",
          reason: "StorageAlreadyAttached",
          message,
        },
      },
    ]);
    expect(rows.find((r) => r.id === "requests")!.checks[0].condition).toEqual({
      status: "True",
      reason: "Ready",
      message: "All requests are ready",
    });
    expect(
      rows.find((r) => r.id === "attachments")!.checks[0].condition,
    ).toBeUndefined();
  });

  it("shows no rows of selections without a claim", () => {
    expect(
      clusterProgressRows(undefined, sections, registrations, []).map(
        (r) => r.id,
      ),
    ).toEqual(["requests", "cluster", "attachments"]);
  });
});
