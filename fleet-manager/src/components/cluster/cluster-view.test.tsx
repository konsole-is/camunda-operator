// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import {
  CLAIM_LABEL,
  COMPONENT_LABEL,
  ENVIRONMENT_LABEL,
  REVISION_ANNOTATION,
} from "@/lib/labels";
import { type ClusterPageData, ClusterView } from "./cluster-view";
import { ACTION_SLOTS, type ActionSlots } from "./slots";

const live = vi.hoisted(() => ({
  objects: {} as Record<string, unknown[]>,
  calls: [] as { kind: string; opts: unknown }[],
  errors: {} as Record<string, string>,
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, opts: unknown, initial: unknown[]) => {
    live.calls.push({ kind, opts });
    return { items: live.objects[kind] ?? initial, error: live.errors[kind] };
  },
  useResource: (kind: string, ref: { name: string }, initial?: unknown) => {
    live.calls.push({ kind, opts: ref });
    const items = live.objects[kind] as
      { metadata: { name: string } }[] | undefined;
    return {
      item: items ? items.find((o) => o.metadata.name === ref.name) : initial,
    };
  },
}));

beforeEach(() => {
  live.objects = {};
  live.calls = [];
  live.errors = {};
  window.history.replaceState(null, "", "/fleet/clusters/team-a/orders");
});
afterEach(cleanup);

const now = "2026-10-18T12:00:00Z";

function brokerPod(i: number): Resource<"Pod"> {
  return {
    kind: "Pod",
    metadata: {
      name: `orders-zeebe-${i}`,
      namespace: "team-a",
      labels: { [COMPONENT_LABEL]: "zeebe" },
    },
    status: {
      phase: "Running",
      conditions: [{ type: "Ready", status: "True" }],
    },
  } as Resource<"Pod">;
}

function cluster(
  over: { labels?: Record<string, string>; spec?: object } = {},
): Resource<"CamundaCluster"> {
  return {
    kind: "CamundaCluster",
    metadata: {
      namespace: "team-a",
      name: "orders",
      labels: over.labels ?? { [CLAIM_LABEL]: "orders" },
    },
    spec: {
      externalUrl: "https://orders.example.com",
      presetRef: "medium",
      ...over.spec,
    },
    status: {
      conditions: [
        {
          type: "Ready",
          status: "True",
          reason: "Healthy",
          message: "",
          lastTransitionTime: now,
        },
      ],
      management: {
        version: "8.9.23",
        endpoint: "http://orders:9600",
        partitions: 3,
        auth: { method: "none" },
      },
    },
  } as Resource<"CamundaCluster">;
}

function claim(revision?: string): Resource<"CamundaClusterClaim"> {
  return {
    kind: "CamundaClusterClaim",
    metadata: {
      namespace: "team-a",
      name: "orders",
      annotations: revision ? { [REVISION_ANNOTATION]: revision } : {},
    },
    spec: {},
  } as unknown as Resource<"CamundaClusterClaim">;
}

function data(over: Partial<ClusterPageData> = {}): ClusterPageData {
  return {
    cluster: cluster(),
    namespace: {
      kind: "Namespace",
      metadata: { name: "team-a", labels: { [ENVIRONMENT_LABEL]: "prod" } },
    } as Resource<"Namespace">,
    environments: [
      {
        kind: "Environment",
        metadata: { name: "prod" },
        spec: { displayName: "Production" },
      } as unknown as Resource<"Environment">,
    ],
    claims: [claim()],
    presets: [
      {
        kind: "CamundaClusterPreset",
        metadata: { name: "medium" },
        spec: { cluster: {} },
      } as unknown as Resource<"CamundaClusterPreset">,
    ],
    releases: [],
    storage: [],
    accessRequests: [],
    pods: [brokerPod(0), brokerPod(1), brokerPod(2)],
    podSelector: "camunda.io/cluster=orders",
    backups: { elasticsearch: [], rdbms: [] },
    restores: { elasticsearch: [], rdbms: [], pointInTime: [] },
    ...over,
  };
}

// waitingLine returns the text of the Git waiting line, or undefined.
const waitingLine = () =>
  screen
    .queryAllByRole("status")
    .map((el) => el.textContent)
    .find((text) => text?.includes("waiting for GitOps"));

const view = (props: Partial<Parameters<typeof ClusterView>[0]> = {}) => (
  <ClusterView initial={data()} tab="overview" now={now} {...props} />
);

const filled: ActionSlots = Object.fromEntries(
  ACTION_SLOTS.map((name) => [name, <button key={name}>Slot {name}</button>]),
);

describe("ClusterView", () => {
  it("a managed cluster shows the header, the four tabs, and empty action slots", async () => {
    const { container } = render(view());
    expect(
      screen.getByRole("heading", { level: 1, name: "orders" }),
    ).toBeTruthy();
    for (const text of ["Production", "team-a", "8.9.23", "medium"]) {
      expect(screen.getByText(text)).toBeTruthy();
    }
    expect(screen.getAllByRole("tab").map((t) => t.textContent)).toEqual([
      "Overview",
      "Backups",
      "Endpoints",
      "Claim",
    ]);

    const slot = (name: string) =>
      container.querySelector(`[data-action-slot="${name}"]`);
    for (const name of ["cluster.header.actions", "cluster.topology.actions"]) {
      expect(slot(name)?.childNodes.length).toBe(0);
    }
    await userEvent.click(screen.getByRole("tab", { name: "Backups" }));
    expect(slot("cluster.backups.actions")?.childNodes.length).toBe(0);
    await userEvent.click(screen.getByRole("tab", { name: "Claim" }));
    expect(slot("cluster.claim.editor")?.childNodes.length).toBe(0);
  });

  it("an unmanaged cluster shows the banner and no actions", async () => {
    const pushedClaim = {
      path: "clusters/team-a/camundaclusterclaims/orders.yaml",
      revision: "bbb",
      pushedAt: now,
    };
    const { container } = render(
      view({
        initial: data({ cluster: cluster({ labels: {} }) }),
        slots: filled,
        pushedClaim,
      }),
    );
    expect(
      screen.getByText(
        "Not managed by the fleet manager. You can read this cluster but not change it here.",
      ),
    ).toBeTruthy();
    expect(waitingLine()).toBeUndefined();
    for (const tab of ["Overview", "Backups", "Endpoints", "Claim"]) {
      await userEvent.click(screen.getByRole("tab", { name: tab }));
      expect(container.querySelector("[data-action-slot]")).toBeNull();
      expect(screen.queryByRole("button", { name: /^Slot / })).toBeNull();
    }
  });

  it("a managed cluster mounts the nodes of the slots", () => {
    render(view({ slots: filled }));
    expect(
      screen.getByRole("button", { name: "Slot cluster.header.actions" }),
    ).toBeTruthy();
  });

  it("a running LogicalBackupRDBMS of the cluster shows the banner with a link to the Backups tab", async () => {
    const backup = {
      kind: "LogicalBackupRDBMS",
      metadata: { namespace: "team-a", name: "nightly-1" },
      spec: { clusterRef: { name: "orders" } },
      status: { phase: "Running" },
    } as unknown as Resource<"LogicalBackupRDBMS">;
    render(
      view({
        initial: data({ backups: { elasticsearch: [], rdbms: [backup] } }),
      }),
    );
    expect(screen.getByText(/Backup nightly-1 is running\./)).toBeTruthy();
    const link = screen.getByRole("link", { name: "Open the Backups tab" });
    expect(link.getAttribute("href")).toBe("?tab=backups");
    await userEvent.click(link);
    expect(
      screen
        .getByRole("tab", { name: "Backups" })
        .getAttribute("aria-selected"),
    ).toBe("true");
    expect(window.location.search).toBe("?tab=backups");
  });

  it("in Git mode, a claim whose live revision differs from the pushed one shows the waiting line", () => {
    const pushedClaim = {
      path: "clusters/team-a/camundaclusterclaims/orders.yaml",
      revision: "bbb",
      pushedAt: "2026-10-18T11:58:00Z",
    };
    const { rerender } = render(
      view({ initial: data({ claims: [claim("aaa")] }), pushedClaim }),
    );
    expect(waitingLine()).toBe(
      "1 change waiting for GitOps: clusters/team-a/camundaclusterclaims/orders.yaml, pushed 2m ago.",
    );

    live.objects.CamundaClusterClaim = [claim("bbb")];
    rerender(view({ initial: data({ claims: [claim("aaa")] }), pushedClaim }));
    expect(waitingLine()).toBeUndefined();
  });

  it("opens on the tab of the URL", () => {
    render(view({ tab: "claim" }));
    expect(
      screen.getByRole("tab", { name: "Claim" }).getAttribute("aria-selected"),
    ).toBe("true");
  });

  it("keeps the selected tab in the URL", async () => {
    render(view());
    await userEvent.click(screen.getByRole("tab", { name: "Endpoints" }));
    expect(window.location.search).toBe("?tab=endpoints");
  });

  it("the topology follows the merged spec of the preset", () => {
    const preset = {
      kind: "CamundaClusterPreset",
      metadata: { name: "medium" },
      spec: { cluster: { gateway: { mode: "Embedded" } } },
    } as unknown as Resource<"CamundaClusterPreset">;
    render(view({ initial: data({ presets: [preset] }) }));
    const topology = screen.getByRole("region", { name: "Topology" });
    expect(within(topology).getByText("Brokers")).toBeTruthy();
    expect(within(topology).queryByText("Gateway")).toBeNull();
    expect(within(topology).getByText("Embedded: Gateway")).toBeTruthy();
  });

  it("shows the launch links of the access request without externalUrl", () => {
    const request = (claimName: string, name: string, url: string) =>
      ({
        kind: "PluginRequest",
        metadata: {
          namespace: "team-a",
          name: `${claimName}-access`,
          labels: { [CLAIM_LABEL]: claimName },
        },
        spec: {},
        status: { links: [{ name, url }] },
      }) as unknown as Resource<"PluginRequest">;
    render(
      view({
        initial: data({
          cluster: cluster({ spec: { externalUrl: undefined } }),
          accessRequests: [
            request("payments", "tasklist", "https://t.example.com"),
            request("orders", "operate", "https://o.example.com"),
          ],
        }),
      }),
    );
    expect(
      screen
        .getAllByRole("link", { name: /Operate|Tasklist|Admin/ })
        .map((l) => [l.textContent, l.getAttribute("href")]),
    ).toEqual([["Operate", "https://o.example.com"]]);
  });

  it("shows a failed read with its kind and keeps the page", () => {
    live.errors.LogicalBackupRDBMS = "forbidden";
    render(view());
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read LogicalBackupRDBMS: forbidden",
    );
    expect(
      screen.getByRole("heading", { level: 1, name: "orders" }),
    ).toBeTruthy();
  });

  it("says the cluster no longer exists when it goes away", () => {
    live.objects.CamundaCluster = [];
    render(view());
    expect(screen.getByText("This cluster no longer exists.")).toBeTruthy();
    expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
    expect(screen.queryAllByRole("tab")).toEqual([]);
  });

  it("watches the pods with the selector of the page", () => {
    render(
      view({
        initial: data({ podSelector: "camunda.io/cluster=long-0123456789" }),
      }),
    );
    expect(live.calls.find((c) => c.kind === "Pod")?.opts).toEqual({
      namespace: "team-a",
      labelSelector: "camunda.io/cluster=long-0123456789",
    });
  });

  it("shows the pods of the cluster as dots", () => {
    render(view());
    expect(
      screen.getByRole("img", { name: "orders-zeebe-0: Ready" }),
    ).toBeTruthy();
  });
});
