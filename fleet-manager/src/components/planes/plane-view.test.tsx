// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import { type PlanePageData, PlaneView } from "./plane-view";

// live holds the objects that the mocked watches report. A kind that it
// does not name reports the snapshot of the server.
const live = vi.hoisted(() => ({
  objects: new Map<string, unknown>(),
}));
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(window.location.search),
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (_kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
  }),
  useResource: (kind: string, _ref: unknown, initial?: unknown) => ({
    item: live.objects.has(kind) ? live.objects.get(kind) : initial,
  }),
}));

const now = "2026-11-15T12:00:00Z";

beforeEach(() => {
  live.objects = new Map();
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
    window.history.replaceState(null, "", "?tab=overview");
    render(<PlaneView data={data()} />);
    expect(
      screen
        .getByRole("tab", { name: "Overview" })
        .getAttribute("aria-selected"),
    ).toBe("true");
  });
});
