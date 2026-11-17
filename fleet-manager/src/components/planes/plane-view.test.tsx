// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
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
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));
vi.mock("@/lib/observe/use-observe", () => ({
  usePodUsage: () => undefined,
}));
// The detail of a component is tested on its own. Here it shows its title.
vi.mock("@/components/cluster/component-detail", () => ({
  ComponentDetail: ({ title }: { title: string }) => (
    <section aria-label={title}>
      <h2 tabIndex={-1}>{title}</h2>
    </section>
  ),
}));
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
      management: {
        identityProvider: { oidc: {} },
        identity: {
          admin: { claimName: "email", claimValue: "a@example.com" },
        },
      },
      versions: { identity: "8.9.14" },
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
    optimizes: [],
    managementAuthConfigs: [],
    pods: [],
    platformConfigs: [],
    registrations: [],
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
        .getByRole("tab", { name: "Clusters served" })
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

  const planeUnknown =
    "The page could not read the management plane, so it cannot tell if the claim plane-a made it.";

  it("shows a claim of the same name as a claim, not as the plane's, while the plane read fails", () => {
    render(
      <PlaneView data={data({ plane: undefined, readErrors: [planeError] })} />,
    );
    expect(alerts()).toEqual([
      "Could not read CamundaManagementCluster: CamundaManagementCluster is forbidden",
    ]);
    expect(screen.queryByText(notCreated)).toBeNull();
    expect(screen.getByText(planeUnknown)).toBeTruthy();
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.getByText("Not known")).toBeTruthy();
    expect(fact("Identity provider")).toBe("-");
    expect(fact("Platform config")).toBe("-");
    expect(fact("Serves")).toBe("-");
    expect(within(table()).getByText("RequestsReady")).toBeTruthy();
  });

  it("keeps a failed first read while the watch of its kind has not answered", () => {
    const page = data({ plane: undefined, readErrors: [planeError] });
    const { rerender } = render(<PlaneView data={page} />);
    expect(alerts()).toEqual([
      "Could not read CamundaManagementCluster: CamundaManagementCluster is forbidden",
    ]);
    live.fresh.add("CamundaManagementCluster");
    live.objects.set("CamundaManagementCluster", planeOf());
    rerender(<PlaneView data={page} />);
    expect(alerts()).toEqual([]);
    expect(screen.queryByText(planeUnknown)).toBeNull();
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
    expect(screen.getByText(planeUnknown)).toBeTruthy();
    expect(fact("Serves")).toBe("-");
  });

  it("does not say that the claim controller has not created the plane while the claim watch fails", () => {
    live.errors.set("CamundaManagementClusterClaim", "The API server is down");
    render(<PlaneView data={data({ plane: undefined })} />);
    expect(alerts()).toEqual([
      "Could not read CamundaManagementClusterClaim: The API server is down",
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

  describe("the read tabs", () => {
    const empties: [string, string][] = [
      ["clusters", "This plane lists no clusters yet."],
      [
        "optimize",
        "No Optimize with an external URL reads the contract of this plane.",
      ],
      ["contract", "This plane has not written its contract yet."],
      ["endpoints", "This plane reports no endpoints yet."],
    ];
    const unreadPlane =
      "The page could not read CamundaManagementCluster, so this tab can be incomplete or out of date.";

    it("lists Overview, Clusters served, Optimize, Contract, and Endpoints in that order", () => {
      render(<PlaneView data={data()} />);
      expect(
        screen
          .getAllByRole("tab")
          .map((t) => t.textContent)
          .slice(0, 5),
      ).toEqual([
        "Overview",
        "Clusters served",
        "Optimize",
        "Contract",
        "Endpoints",
      ]);
    });

    it.each(empties)(
      "a claim-only plane shows the empty text of the %s tab",
      (tab, text) => {
        window.history.replaceState(null, "", `?tab=${tab}`);
        render(<PlaneView data={data({ plane: undefined })} />);
        expect(screen.getByText(text)).toBeTruthy();
      },
    );

    it.each(empties)(
      "the %s tab does not show its empty text while the plane read fails, and names the kind",
      (tab, text) => {
        window.history.replaceState(null, "", `?tab=${tab}`);
        render(
          <PlaneView
            data={data({ plane: undefined, readErrors: [planeError] })}
          />,
        );
        expect(screen.queryByText(text)).toBeNull();
        expect(screen.getByText(unreadPlane)).toBeTruthy();
      },
    );

    it("shows the routes of a plane on the Endpoints tab", () => {
      window.history.replaceState(null, "", "?tab=endpoints");
      const plane = planeOf();
      plane.spec.identity = {
        externalUrl: "https://management.example.com/identity",
      } as typeof plane.spec.identity;
      plane.status = {
        ...plane.status,
        endpoints: [
          {
            name: "identity",
            pathPrefix: "/identity",
            port: 80,
            protocol: "HTTP",
            service: "plane-a-identity",
          },
        ],
      };
      render(<PlaneView data={data({ plane })} />);
      const table = screen.getByRole("table", { name: "Endpoints" });
      expect(within(table).getByText("identity")).toBeTruthy();
      expect(
        within(table).getByText("https://management.example.com/identity"),
      ).toBeTruthy();
    });

    it("shows the Web Modeler routes with their URLs and apps on the Endpoints tab", () => {
      window.history.replaceState(null, "", "?tab=endpoints");
      const plane = planeOf();
      const host = "https://management.example.com";
      (plane.spec as Record<string, unknown>).webModeler = {
        externalUrl: `${host}/modeler`,
        websocketsExternalUrl: `${host}/modeler-ws`,
      };
      const route = (name: string, pathPrefix: string) => ({
        name,
        pathPrefix,
        port: 80,
        protocol: "HTTP" as const,
        service: `plane-a-${name}`,
      });
      plane.status = {
        ...plane.status,
        endpoints: [
          route("web-modeler", "/modeler"),
          route("web-modeler-websockets", "/modeler-ws"),
        ],
      };
      render(<PlaneView data={data({ plane })} />);
      const table = screen.getByRole("table", { name: "Endpoints" });
      const rowOf = (name: string) =>
        within(table).getByRole("row", { name: new RegExp(`^${name} `) });
      expect(
        within(rowOf("web-modeler")).getByRole("link", {
          name: `${host}/modeler`,
        }),
      ).toBeTruthy();
      expect(
        within(rowOf("web-modeler")).getByText("Web Modeler"),
      ).toBeTruthy();
      expect(
        within(rowOf("web-modeler-websockets")).getByRole("link", {
          name: `${host}/modeler-ws`,
        }),
      ).toBeTruthy();
      expect(
        within(rowOf("web-modeler-websockets")).getByText(
          "Web Modeler websockets",
        ),
      ).toBeTruthy();
    });

    it("does not say that no Optimize reads the contract while the Optimize watch fails", () => {
      window.history.replaceState(null, "", "?tab=contract");
      live.errors.set("CamundaOptimize", "The API server is down");
      const plane = planeOf();
      plane.status = { ...plane.status, managementAuthConfig: "plane-a" };
      render(<PlaneView data={data({ plane })} />);
      expect(screen.queryByText("No Optimize reads this contract.")).toBeNull();
      expect(
        screen.getByText(
          "The page could not read CamundaOptimize, so this tab can be incomplete or out of date.",
        ),
      ).toBeTruthy();
    });

    it("names a list whose watch answered once and is not current now", () => {
      window.history.replaceState(null, "", "?tab=contract");
      const plane = planeOf();
      plane.status = { ...plane.status, managementAuthConfig: "plane-a" };
      const page = data({ plane });
      live.fresh.add("ManagementAuthConfig");
      const { rerender } = render(<PlaneView data={page} />);
      expect(
        screen.getByText("The contract plane-a does not exist."),
      ).toBeTruthy();
      live.fresh.delete("ManagementAuthConfig");
      rerender(<PlaneView data={page} />);
      expect(
        screen.queryByText("The contract plane-a does not exist."),
      ).toBeNull();
      expect(
        screen.getByText(
          "The page could not read ManagementAuthConfig, so this tab can be incomplete or out of date.",
        ),
      ).toBeTruthy();
    });
  });

  describe("the component cards", () => {
    const cardsRegion = () =>
      screen.getByRole("region", { name: "Components" });

    it("a managed plane shows the cards before the Conditions section", () => {
      render(<PlaneView data={data()} />);
      const cards = cardsRegion();
      expect(within(cards).getByText("Management Identity")).toBeTruthy();
      expect(within(cards).getByText("Identity provider")).toBeTruthy();
      expect(
        cards.compareDocumentPosition(table()) &
          Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();
    });

    it("IdentityReady False turns the Identity dot red and stays out of the Conditions section", () => {
      render(<PlaneView data={data()} />);
      const dot = within(cardsRegion()).getByRole("img", {
        name: "Management Identity: Not ready (Down)",
      });
      expect(dot.className).toContain("bg-bad");
      expect(within(table()).queryByText("IdentityReady")).toBeNull();
    });

    it("a click on a card opens its detail and All components goes back", async () => {
      render(<PlaneView data={data()} />);
      await userEvent.click(
        within(cardsRegion()).getByRole("link", {
          name: "Management Identity",
        }),
      );
      expect(window.location.search).toBe("?tab=overview&component=identity");
      expect(
        screen.getByRole("region", { name: "Management Identity" }),
      ).toBeTruthy();
      expect(screen.queryByRole("region", { name: "Components" })).toBeNull();
      await userEvent.click(
        screen.getByRole("link", { name: "All components" }),
      );
      expect(cardsRegion()).toBeTruthy();
    });

    it("opens the detail that the component query parameter names", () => {
      window.history.replaceState(null, "", "?tab=overview&component=identity");
      render(<PlaneView data={data()} />);
      expect(
        screen.getByRole("region", { name: "Management Identity" }),
      ).toBeTruthy();
    });

    it("the identity provider card is no link and opens no detail", () => {
      window.history.replaceState(
        null,
        "",
        "?tab=overview&component=identityProvider",
      );
      render(<PlaneView data={data()} />);
      expect(
        within(cardsRegion()).queryByRole("link", {
          name: "Identity provider",
        }),
      ).toBeNull();
    });

    it("a claim-only plane shows no cards", () => {
      render(<PlaneView data={data({ plane: undefined })} />);
      expect(screen.queryByRole("region", { name: "Components" })).toBeNull();
    });

    it("says Not known on each dot and names the kind while the plane watch is out of date", () => {
      live.fresh = new Set(["CamundaManagementCluster"]);
      const { rerender } = render(<PlaneView data={data()} />);
      live.fresh = new Set();
      rerender(<PlaneView data={data()} />);
      expect(
        within(cardsRegion()).getByRole("img", {
          name: "Management Identity: Not known",
        }),
      ).toBeTruthy();
      expect(
        screen.getByText(/could not read CamundaManagementCluster/),
      ).toBeTruthy();
    });
  });

  describe("the actions", () => {
    const claimActions = {
      preview: vi.fn(),
      apply: vi.fn(async () => ({
        ok: true as const,
        mode: "direct" as const,
      })),
    };
    const withProviders = (node: ReactNode) => (
      <QueryClientProvider client={new QueryClient()}>
        <GitPushesProvider>{node}</GitPushesProvider>
      </QueryClientProvider>
    );
    const header = () =>
      screen.getByRole("heading", { level: 1 }).closest("header")!;

    it("a managed plane shows Suspend and Delete in the header", () => {
      render(
        withProviders(<PlaneView data={data()} claimActions={claimActions} />),
      );
      expect(
        within(header()).getByRole("button", { name: "Suspend" }),
      ).toBeTruthy();
      expect(
        within(header())
          .getByRole("link", { name: "Delete" })
          .getAttribute("href"),
      ).toBe("/fleet/planes/platform/plane-a/delete");
    });

    it("an unlabelled plane has no Suspend and no Delete", () => {
      render(
        withProviders(
          <PlaneView
            data={data({ plane: planeOf({}) })}
            claimActions={claimActions}
          />,
        ),
      );
      expect(screen.queryByRole("button", { name: "Suspend" })).toBeNull();
      expect(screen.queryByRole("link", { name: "Delete" })).toBeNull();
    });

    it("a plane without claimActions shows no actions", () => {
      render(<PlaneView data={data()} />);
      expect(screen.queryByRole("button", { name: "Suspend" })).toBeNull();
      expect(screen.queryByRole("link", { name: "Delete" })).toBeNull();
    });

    it("Ready with reason Suspended shows Suspended and Resume", () => {
      const plane = planeOf();
      plane.status!.conditions = [
        condition("Ready", "True", "Suspended"),
      ] as never;
      const claim = claimOf();
      (claim.spec as { suspend?: boolean }).suspend = true;
      render(
        withProviders(
          <PlaneView
            data={data({ plane, claim })}
            claimActions={claimActions}
          />,
        ),
      );
      expect(within(header()).getByText("Ready · Suspended")).toBeTruthy();
      expect(
        within(header()).getByRole("button", { name: "Resume" }),
      ).toBeTruthy();
    });

    it("the Suspend confirm of a plane with its own Keycloak names the served OIDC clusters", async () => {
      const plane = planeOf();
      plane.spec.identityProvider = { keycloak: {} } as never;
      plane.status!.clusters = [
        { namespace: "team-orders", name: "orders", attached: true },
      ] as never;
      render(
        withProviders(
          <PlaneView
            data={data({ plane })}
            writeMode="direct"
            claimActions={claimActions}
          />,
        ),
      );
      await userEvent.click(
        within(header()).getByRole("button", { name: "Suspend" }),
      );
      expect(
        screen.getByRole("group", { name: "Suspend" }).textContent,
      ).toContain(
        "Each cluster that uses the Keycloak of this plane as its OIDC issuer loses sign-in",
      );
    });

    it("turns Suspend off while the write mode is not known", () => {
      render(
        withProviders(<PlaneView data={data()} claimActions={claimActions} />),
      );
      expect(
        within(header()).getByRole("button", { name: "Suspend" }),
      ).toHaveProperty("disabled", true);
    });

    it("turns Suspend and Delete off while the plane read fails", () => {
      live.errors.set("CamundaManagementCluster", "connection lost");
      render(
        withProviders(<PlaneView data={data()} claimActions={claimActions} />),
      );
      expect(
        within(header()).getByRole("button", { name: "Suspend" }),
      ).toHaveProperty("disabled", true);
      expect(
        within(header()).queryByRole("link", { name: "Delete" }),
      ).toBeNull();
      expect(within(header()).getByText("Not known")).toBeTruthy();
    });
  });
});
