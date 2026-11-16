// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import { type PlanePageData, PlaneView } from "./plane-view";

// live holds what the mocked watches report. useResource reports the
// snapshot of the server for a kind that live.objects does not name.
const live = vi.hoisted(() => ({
  objects: new Map<string, unknown>(),
  errors: new Map<string, string>(),
  fresh: new Set<string>(),
}));
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(window.location.search),
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
    ...(live.errors.has(kind) && { error: live.errors.get(kind) }),
  }),
  useResource: (kind: string, _ref: unknown, initial?: unknown) => ({
    item: live.objects.has(kind) ? live.objects.get(kind) : initial,
    ...(live.errors.has(kind) && { error: live.errors.get(kind) }),
  }),
  useWatchesFresh: (lists: { kind: string }[]) =>
    lists.map((l) => live.fresh.has(l.kind)),
}));
// The page lists one tab today. A second one shows that the view takes the
// tab from the URL and not from its default.
vi.mock("./plane-tabs", () => {
  const tabs = [
    { id: "overview", label: "Overview" },
    { id: "clusters", label: "Clusters" },
  ];
  return {
    PLANE_TABS: tabs,
    planeTabOf: (value: string | undefined) =>
      tabs.find((t) => t.id === value)?.id ?? "overview",
  };
});

const now = "2026-11-15T12:00:00Z";

beforeEach(() => {
  live.objects = new Map();
  live.errors = new Map();
  live.fresh = new Set();
  window.history.replaceState(null, "", "/fleet/planes/platform/plane-a");
});
afterEach(cleanup);

const condition = (type: string, status = "True", reason = "Healthy") => ({
  type,
  status,
  reason,
  message: "",
  lastTransitionTime: now,
});

function planeOf(
  labels: Record<string, string> = { [CLAIM_LABEL]: "plane-a" },
): Resource<"CamundaManagementCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: { namespace: "platform", name: "plane-a", labels },
    spec: {
      platformConfigRef: "oidc-platform",
      identityProvider: { oidc: {} },
      identity: {},
      namespaceSelector: { matchLabels: { team: "a" } },
      clusterSelector: { matchLabels: { fleet: "a" } },
    },
    status: {
      conditions: [
        condition("Ready"),
        condition("SecretsReady", "True", "Disabled"),
        condition("ManagementAuthReady"),
        condition("IdentityReady", "False", "Down"),
      ],
    },
  } as unknown as Resource<"CamundaManagementCluster">;
}

function claimOf(
  ready = condition("Ready"),
): Resource<"CamundaManagementClusterClaim"> {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { namespace: "platform", name: "plane-a", resourceVersion: "7" },
    spec: {
      environments: ["platform"],
      management: { identityProvider: { oidc: {} }, identity: {} },
    },
    status: {
      conditions: [
        ready,
        condition("RequestsReady"),
        condition("ManagementClusterReady", "False", "Creating"),
      ],
    },
  } as unknown as Resource<"CamundaManagementClusterClaim">;
}

function data(page: Partial<PlanePageData> = {}): PlanePageData {
  return {
    namespace: "platform",
    name: "plane-a",
    now,
    plane: planeOf(),
    claim: claimOf(),
    namespaces: [
      {
        metadata: {
          name: "platform",
          labels: { [ENVIRONMENT_LABEL]: "platform" },
        },
      } as unknown as Resource<"Namespace">,
    ],
    environments: [
      {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "Environment",
        metadata: { name: "platform" },
        spec: { platformConfigRef: "env-platform" },
      } as unknown as Resource<"Environment">,
    ],
    ...page,
  };
}

const table = () =>
  screen.getByRole("region", { name: "Conditions outside the components" });

// fact returns the description of the header term of that name.
function fact(term: string): string | null {
  const dt = screen.getByText(term, { selector: "dt" });
  return dt.nextElementSibling?.textContent ?? null;
}

describe("PlaneView", () => {
  it("a managed plane shows the header and the Conditions section", () => {
    render(<PlaneView data={data()} />);
    expect(
      screen.getByRole("heading", { level: 1, name: "plane-a" }),
    ).toBeTruthy();
    expect(fact("Identity provider")).toBe("OIDC");
    expect(fact("Platform config")).toBe("oidc-platform");
    expect(fact("Serves")).toBe("platform");
    expect(within(table()).getByText("SecretsReady")).toBeTruthy();
    expect(within(table()).getByText("RequestsReady")).toBeTruthy();
    expect(within(table()).queryByText("IdentityReady")).toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("a claim-only plane shows the claim Ready in the header and its conditions", () => {
    render(
      <PlaneView
        data={data({
          plane: undefined,
          claim: claimOf(condition("Ready", "False", "Creating")),
        })}
      />,
    );
    expect(screen.getByText("Not ready · Creating")).toBeTruthy();
    expect(
      screen.getByText(
        "The claim controller has not created the management plane yet.",
      ),
    ).toBeTruthy();
    expect(within(table()).getByText("ManagementClusterReady")).toBeTruthy();
    expect(fact("Platform config")).toBe("env-platform");
  });

  it("an unlabelled plane shows the banner and no claim group", () => {
    render(<PlaneView data={data({ plane: planeOf({}) })} />);
    expect(screen.getByRole("status").textContent).toContain(
      "You can read this management plane but not change it here.",
    );
    expect(within(table()).queryByText("Claim", { exact: true })).toBeNull();
    expect(within(table()).queryByText("RequestsReady")).toBeNull();
    expect(fact("Serves")).toBe("team=a");
  });

  it("an unlabelled plane without a cluster selector serves no cluster", () => {
    const plane = planeOf({});
    delete (plane.spec as { clusterSelector?: unknown }).clusterSelector;
    render(<PlaneView data={data({ plane })} />);
    expect(fact("Serves")).toBe(
      "No cluster, because it has no cluster selector",
    );
  });

  it("shows the plane once it exists", () => {
    live.objects.set("CamundaManagementCluster", undefined);
    const page = data({ plane: undefined });
    const { rerender } = render(<PlaneView data={page} />);
    expect(
      screen.getByText(
        "The claim controller has not created the management plane yet.",
      ),
    ).toBeTruthy();
    live.objects.set("CamundaManagementCluster", planeOf());
    rerender(<PlaneView data={page} />);
    expect(
      screen.queryByText(
        "The claim controller has not created the management plane yet.",
      ),
    ).toBeNull();
    expect(within(table()).getByText("SecretsReady")).toBeTruthy();
  });

  it("says the plane no longer exists when both objects are gone", () => {
    live.objects.set("CamundaManagementCluster", undefined);
    live.objects.set("CamundaManagementClusterClaim", undefined);
    render(<PlaneView data={data()} />);
    expect(
      screen.getByText("This management plane no longer exists."),
    ).toBeTruthy();
    expect(
      screen
        .getByRole("link", { name: "All management planes" })
        .getAttribute("href"),
    ).toBe("/fleet/planes");
    expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
  });

  it("selects the tab of the tab query parameter", () => {
    window.history.replaceState(null, "", "?tab=clusters");
    render(<PlaneView data={data()} />);
    expect(
      screen
        .getByRole("tab", { name: "Clusters" })
        .getAttribute("aria-selected"),
    ).toBe("true");
    expect(
      screen
        .getByRole("tab", { name: "Overview" })
        .getAttribute("aria-selected"),
    ).toBe("false");
  });

  const notCreated =
    "The claim controller has not created the management plane yet.";
  const planeError = {
    kind: "CamundaManagementCluster",
    message: "CamundaManagementCluster is forbidden",
  };
  const claimError = {
    kind: "CamundaManagementClusterClaim",
    message: "CamundaManagementClusterClaim is forbidden",
  };
  const alerts = () => screen.queryAllByRole("alert").map((a) => a.textContent);

  it("names a failed plane read beside the claim and does not say the plane is missing", () => {
    render(
      <PlaneView data={data({ plane: undefined, readErrors: [planeError] })} />,
    );
    expect(alerts()).toEqual([
      "Could not read CamundaManagementCluster: CamundaManagementCluster is forbidden",
    ]);
    expect(screen.queryByText(notCreated)).toBeNull();
    expect(within(table()).getByText("RequestsReady")).toBeTruthy();
  });

  it("names a failed plane read without a claim and does not say the plane is gone", () => {
    render(
      <PlaneView
        data={data({
          plane: undefined,
          claim: undefined,
          readErrors: [planeError],
        })}
      />,
    );
    expect(alerts()).toEqual([
      "Could not read CamundaManagementCluster: CamundaManagementCluster is forbidden",
    ]);
    expect(
      screen.queryByText("This management plane no longer exists."),
    ).toBeNull();
    expect(
      screen.getByRole("link", { name: "All management planes" }),
    ).toBeTruthy();
  });

  it("names both failed reads when neither object could be read", () => {
    render(
      <PlaneView
        data={data({
          plane: undefined,
          claim: undefined,
          readErrors: [planeError, claimError],
        })}
      />,
    );
    expect(alerts()).toHaveLength(2);
    expect(
      screen.queryByText("This management plane no longer exists."),
    ).toBeNull();
  });

  it("drops a failed first read once the watch of its kind got a snapshot", () => {
    live.fresh.add("CamundaManagementCluster");
    live.objects.set("CamundaManagementCluster", planeOf());
    render(
      <PlaneView data={data({ plane: undefined, readErrors: [planeError] })} />,
    );
    expect(alerts()).toEqual([]);
    expect(within(table()).getByText("SecretsReady")).toBeTruthy();
  });

  it("names a watch error after a good first read and does not say the plane is missing", () => {
    live.errors.set("CamundaManagementCluster", "The API server is down");
    render(<PlaneView data={data({ plane: undefined })} />);
    expect(alerts()).toEqual([
      "Could not read CamundaManagementCluster: The API server is down",
    ]);
    expect(screen.queryByText(notCreated)).toBeNull();
  });

  it("names a watch error and does not say the plane is gone when the watches report no object", () => {
    live.objects.set("CamundaManagementCluster", undefined);
    live.objects.set("CamundaManagementClusterClaim", undefined);
    live.errors.set("CamundaManagementCluster", "The API server is down");
    render(<PlaneView data={data()} />);
    expect(alerts()).toEqual([
      "Could not read CamundaManagementCluster: The API server is down",
    ]);
    expect(
      screen.queryByText("This management plane no longer exists."),
    ).toBeNull();
  });
});
