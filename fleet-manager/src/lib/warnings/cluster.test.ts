import { describe, expect, it } from "vitest";
import type { SectionComponent } from "@/lib/claims/sections";
import type { Resource } from "@/lib/k8s/kinds";
import { backupWarning, requestWarnings } from "./cluster";

type Summary = NonNullable<
  NonNullable<Resource<"PluginRequest">["status"]>["summary"]
>;

const component = ({
  label = "Secondary storage",
  summary,
  ready,
}: {
  label?: string;
  summary?: Summary;
  ready?: { status: "True" | "False" | "Unknown"; reason?: string; message?: string };
}): SectionComponent => {
  const request = {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRequest",
    metadata: { namespace: "team-a", name: "orders-storage" },
    spec: {},
    ...(summary ? { status: { summary } } : {}),
  } as unknown as Resource<"PluginRequest">;
  return {
    id: "section:secondary-storage",
    section: "secondary-storage",
    label,
    order: 0,
    selection: { section: "secondary-storage", registration: "es" },
    request,
    ...(ready ? { condition: ready } : {}),
  };
};

describe("backupWarning", () => {
  it("warns when the newest backup failed", () => {
    expect(backupWarning({ name: "orders-nightly-1", phase: "Failed" })).toEqual(
      {
        kind: "backup",
        severity: "Warning",
        label: "Backup failed",
        detail: "The newest backup orders-nightly-1 failed",
      },
    );
  });

  it("does not warn when an older backup failed and the newest completed", () => {
    expect(
      backupWarning({ name: "orders-nightly-2", phase: "Completed" }),
    ).toBeUndefined();
  });

  it("gives no backup warning without backups", () => {
    expect(backupWarning(undefined)).toBeUndefined();
  });
});

describe("requestWarnings", () => {
  it("lists the summary items with severity Warning and Error and skips Info", () => {
    expect(
      requestWarnings([
        component({
          summary: [
            { label: "Health", value: "green", severity: "Info" },
            { label: "Disk", value: "81 % used", severity: "Warning" },
            { label: "Shards", value: "3 unassigned", severity: "Error" },
            { label: "Nodes", value: "3" },
          ],
        }),
      ]),
    ).toEqual([
      {
        kind: "summary",
        severity: "Warning",
        label: "Disk",
        detail: "Disk: 81 % used",
      },
      {
        kind: "summary",
        severity: "Error",
        label: "Shards",
        detail: "Shards: 3 unassigned",
      },
    ]);
  });

  it("gives an error for a request that is not ready, with its reason and message", () => {
    expect(
      requestWarnings([
        component({
          ready: {
            status: "False",
            reason: "GrantRejected",
            message: "The grant orders-es is not ready",
          },
        }),
      ]),
    ).toEqual([
      {
        kind: "request",
        severity: "Error",
        label: "Secondary storage not ready",
        detail: "GrantRejected: The grant orders-es is not ready",
      },
    ]);
  });

  it("gives the reason alone when the request has no message", () => {
    expect(
      requestWarnings([
        component({ ready: { status: "False", reason: "GrantRejected" } }),
      ])[0]?.detail,
    ).toBe("GrantRejected");
  });

  it("gives no request warning while the request has no Ready condition", () => {
    expect(requestWarnings([component({})])).toEqual([]);
  });

  it("gives no request warning while Ready is Unknown or True", () => {
    expect(
      requestWarnings([
        component({ ready: { status: "Unknown", reason: "Pending" } }),
        component({ ready: { status: "True", reason: "Ready" } }),
      ]),
    ).toEqual([]);
  });
});
