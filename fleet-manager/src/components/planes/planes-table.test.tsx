// @vitest-environment jsdom
import {
  cleanup,
  render,
  renderHook,
  screen,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import { type PlaneSources, PlanesTable, usePlanes } from "./planes-table";

const lists = vi.hoisted(() => ({
  current: {} as Record<string, { items: unknown[]; error?: string }>,
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, _opts: unknown, initial: unknown[]) =>
    lists.current[kind] ?? { items: initial },
}));

afterEach(() => {
  cleanup();
  lists.current = {};
});

function cluster(
  namespace: string,
  name: string,
  {
    labels,
    spec = {},
    status,
  }: {
    labels?: Record<string, string>;
    spec?: Record<string, unknown>;
    status?: Record<string, unknown>;
  } = {},
) {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: { namespace, name, labels },
    spec: { identityProvider: { keycloak: {} }, ...spec },
    status,
  } as never;
}

function claim(namespace: string, name: string, environments: string[]) {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { namespace, name },
    spec: { environments, management: { identityProvider: { keycloak: {} } } },
  } as never;
}

function Harness({
  environment,
  initial,
}: {
  environment: string;
  initial: PlaneSources;
}) {
  const { planes } = usePlanes(initial);
  return <PlanesTable planes={planes} environment={environment} />;
}

const managed = (name: string, environments: string[], status?: object) => ({
  cluster: cluster("platform", name, {
    labels: { [CLAIM_LABEL]: name },
    status: status as Record<string, unknown>,
  }),
  claim: claim("platform", name, environments),
});

const a = managed("plane-a", ["production", "staging"], {
  conditions: [{ type: "Ready", status: "True", reason: "Healthy" }],
});
const b = managed("plane-b", ["staging"]);
const initial: PlaneSources = {
  clusters: [a.cluster, b.cluster],
  claims: [a.claim, b.claim],
  namespaces: [],
};

describe("PlanesTable", () => {
  it("has the six columns of the fleet table in order", () => {
    render(<Harness environment="production" initial={initial} />);
    expect(
      screen.getAllByRole("columnheader").map((h) => h.textContent),
    ).toEqual([
      "Management plane",
      "Status",
      "Identity",
      "Serves",
      "Clusters",
      "Warnings",
    ]);
  });

  it("shows a plane under each environment it serves", () => {
    const { rerender } = render(
      <Harness environment="production" initial={initial} />,
    );
    expect(screen.getByRole("link", { name: /plane-a/ })).toBeTruthy();
    expect(screen.queryByRole("link", { name: /plane-b/ })).toBeNull();
    rerender(<Harness environment="staging" initial={initial} />);
    expect(screen.getByRole("link", { name: /plane-a/ })).toBeTruthy();
    expect(screen.getByRole("link", { name: /plane-b/ })).toBeTruthy();
  });

  it("links each plane to its page", () => {
    render(<Harness environment="production" initial={initial} />);
    expect(
      screen.getByRole("link", { name: /plane-a/ }).getAttribute("href"),
    ).toBe("/fleet/planes/platform/plane-a");
  });

  it("shows nothing for an environment that no plane serves", () => {
    const { container } = render(
      <Harness environment="sandbox" initial={initial} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("follows the live lists of the hook", () => {
    const { rerender } = render(
      <Harness environment="production" initial={initial} />,
    );
    expect(
      within(screen.getByRole("row", { name: /plane-a/ })).getAllByRole(
        "cell",
      )[1].textContent,
    ).toBe("Ready");
    lists.current.CamundaManagementCluster = {
      items: [
        cluster("platform", "plane-a", {
          labels: { [CLAIM_LABEL]: "plane-a" },
          status: {
            conditions: [
              {
                type: "Ready",
                status: "False",
                reason: "UnsupportedVersion",
              },
            ],
          },
        }),
      ],
    };
    rerender(<Harness environment="production" initial={initial} />);
    expect(screen.getByRole("row", { name: /plane-a/ }).textContent).toContain(
      "UnsupportedVersion",
    );
  });

  it("shows the reason of a Ready plane, such as Suspended", () => {
    const stopped = managed("stopped", ["production"], {
      conditions: [{ type: "Ready", status: "True", reason: "Suspended" }],
    });
    render(
      <Harness
        environment="production"
        initial={{
          clusters: [stopped.cluster],
          claims: [stopped.claim],
          namespaces: [],
        }}
      />,
    );
    expect(screen.getByRole("row", { name: /stopped/ }).textContent).toContain(
      "Ready · Suspended",
    );
  });

  it("marks a plane without the claim label as not managed and shows its selector", () => {
    const legacy: PlaneSources = {
      clusters: [
        cluster("platform", "legacy", {
          spec: {
            namespaceSelector: {
              matchExpressions: [
                {
                  key: ENVIRONMENT_LABEL,
                  operator: "In",
                  values: ["production"],
                },
              ],
            },
          },
        }),
      ],
      claims: [],
      namespaces: [
        {
          apiVersion: "v1",
          kind: "Namespace",
          metadata: {
            name: "team-a",
            labels: { [ENVIRONMENT_LABEL]: "production" },
          },
        } as never,
      ],
    };
    render(<Harness environment="production" initial={legacy} />);
    const row = screen.getByRole("row", { name: /legacy/ });
    expect(row.textContent).toContain("Not managed by the fleet manager");
    expect(row.textContent).toContain(
      "fleet.camunda.io/environment in (production)",
    );
  });

  it("shows the attached count and the warning chip of a plane", () => {
    const c = managed("plane-c", ["production"], {
      clusters: [
        { namespace: "team-a", name: "orders", attached: true },
        { namespace: "team-a", name: "search", attached: true },
        {
          namespace: "team-b",
          name: "billing",
          attached: false,
          reason: "ClaimedElsewhere",
        },
      ],
    });
    render(
      <Harness
        environment="production"
        initial={{ clusters: [c.cluster], claims: [c.claim], namespaces: [] }}
      />,
    );
    const row = screen.getByRole("row", { name: /plane-c/ });
    expect(row.textContent).toContain("2 attached · 1 not attached");
    expect(screen.getByRole("button", { name: "1 not attached" })).toBeTruthy();
  });
});

describe("usePlanes", () => {
  it("returns the error of each failed watch with its kind", () => {
    lists.current.CamundaManagementClusterClaim = {
      items: [],
      error: "claims: forbidden",
    };
    lists.current.Namespace = { items: [], error: "namespaces: forbidden" };
    const { result } = renderHook(() => usePlanes(initial));
    expect(result.current.errors).toEqual([
      {
        kind: "CamundaManagementClusterClaim",
        message: "claims: forbidden",
      },
      { kind: "Namespace", message: "namespaces: forbidden" },
    ]);
    expect(result.current.planes.map((p) => p.name)).toEqual([
      "plane-a",
      "plane-b",
    ]);
  });

  it("has no error while every watch is healthy", () => {
    const { result } = renderHook(() => usePlanes(initial));
    expect(result.current.errors).toEqual([]);
  });
});
