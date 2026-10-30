// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import {
  type ClusterCreateData,
  ClusterCreateFlow,
} from "./cluster-create-flow";

const live = vi.hoisted(() => ({
  objects: {} as Record<string, unknown[]>,
  errors: {} as Record<string, string>,
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, _opts: unknown, initial: unknown[]) => ({
    items: live.objects[kind] ?? initial,
    error: live.errors[kind],
  }),
}));

beforeEach(() => {
  live.objects = {};
  live.errors = {};
});
afterEach(cleanup);

const READY = (status: "True" | "False" = "True", message = "") => ({
  conditions: [
    {
      type: "Ready",
      status,
      reason: "Test",
      message,
      lastTransitionTime: "2026-10-30T00:00:00Z",
    },
  ],
});

function prod(
  namespaceCreation?: Resource<"Environment">["spec"]["namespaceCreation"],
): Resource<"Environment"> {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "prod" },
    spec: {
      displayName: "Production",
      sections: { access: { allowed: ["gateway"], default: "gateway" } },
      presets: { camundaCluster: ["small"] },
      ...(namespaceCreation && { namespaceCreation }),
    },
    status: READY(),
  };
}

const accessSection: Resource<"PluginSection"> = {
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginSection",
  metadata: { name: "access" },
  spec: {
    appliesTo: ["CamundaClusterClaim"],
    label: "Access",
    selection: "One",
  },
};

const gateway: Resource<"PluginRegistration"> = {
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginRegistration",
  metadata: { name: "gateway" },
  spec: {
    sections: ["access"],
    mode: "Optional",
    label: "Gateway",
    deletionPolicies: ["Delete"],
    serviceAccount: { name: "gateway", namespace: "plugins" },
    parameterValues: { hostnamePattern: "{name}.{namespace}.example.com" },
  },
  status: {
    conditions: ["Accepted", "Ready"].map((type) => ({
      type,
      status: "True" as const,
      reason: "Test",
      message: "",
      lastTransitionTime: "2026-10-30T00:00:00Z",
    })),
  },
};

const billingAccess = {
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginRequest",
  metadata: { namespace: "team-b", name: "billing-access" },
  spec: {
    claimRef: { kind: "CamundaClusterClaim", name: "billing" },
    registration: "gateway",
    section: "access",
  },
  status: { outputs: { externalUrl: "https://orders.team-a.example.com" } },
} as unknown as Resource<"PluginRequest">;

function data(over: Partial<ClusterCreateData> = {}): ClusterCreateData {
  return {
    environments: [prod()],
    namespaces: [
      { metadata: { name: "team-a", labels: { [ENVIRONMENT_LABEL]: "prod" } } },
    ],
    claims: [],
    clusters: [],
    presets: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaClusterPreset",
        metadata: { name: "small" },
        spec: { cluster: {} },
      },
    ],
    sections: [accessSection],
    registrations: [gateway],
    accessRequests: [billingAccess],
    ...over,
  };
}

const next = () =>
  screen.getByRole("button", { name: "Next" }) as HTMLButtonElement;

async function fillBasics(name: string) {
  await userEvent.click(screen.getByRole("radio", { name: /Production/ }));
  await userEvent.type(screen.getByRole("textbox", { name: "Name" }), name);
  await userEvent.selectOptions(screen.getByRole("combobox"), "team-a");
  await userEvent.click(screen.getByRole("radio", { name: /^small/ }));
}

describe("ClusterCreateFlow", () => {
  it("shows five steps", () => {
    render(<ClusterCreateFlow initial={data()} />);
    const nav = screen.getByRole("navigation", { name: "Steps" });
    expect(
      within(nav)
        .getAllByRole("listitem")
        .map((li) => li.textContent),
    ).toEqual(
      [
        "Environment and name",
        "Storage and access",
        "Cluster settings",
        "Attachments",
        "Review",
      ].map((label) => expect.stringContaining(label)),
    );
  });

  it("shows the hosts of the default access option under the name", async () => {
    render(<ClusterCreateFlow initial={data({ accessRequests: [] })} />);
    await fillBasics("search");
    expect(screen.getByText("search.team-a.example.com")).toBeTruthy();
    expect(next().disabled).toBe(false);
  });

  it("blocks step 1 on a taken host", async () => {
    render(<ClusterCreateFlow initial={data()} />);
    await fillBasics("orders");
    expect(
      screen.getAllByText(
        "The host orders.team-a.example.com is taken by team-b/billing. Choose another name.",
      ).length,
    ).toBeGreaterThan(0);
    expect(next().disabled).toBe(true);
  });

  it("shows no host line without an access section", async () => {
    render(<ClusterCreateFlow initial={data({ sections: [] })} />);
    await fillBasics("orders");
    expect(screen.queryByText(/example\.com/)).toBeNull();
    expect(next().disabled).toBe(false);
  });

  it("refuses a name that a claim has in the namespace", async () => {
    render(
      <ClusterCreateFlow
        initial={data({
          sections: [],
          claims: [
            {
              apiVersion: "fleet.camunda.io/v1alpha1",
              kind: "CamundaClusterClaim",
              metadata: { namespace: "team-a", name: "orders" },
              spec: {},
            } as Resource<"CamundaClusterClaim">,
          ],
        })}
      />,
    );
    await fillBasics("orders");
    expect(
      screen.getAllByText("The name orders is taken in this namespace.").length,
    ).toBeGreaterThan(0);
    expect(next().disabled).toBe(true);
  });

  it("refuses an environment that turns not ready after the pick", async () => {
    const { rerender } = render(<ClusterCreateFlow initial={data()} />);
    await fillBasics("search");
    live.objects.Environment = [{ ...prod(), status: READY("False", "gone") }];
    rerender(<ClusterCreateFlow initial={data()} />);
    expect(
      screen.getByText("The environment Production is not ready: gone"),
    ).toBeTruthy();
    expect(next().disabled).toBe(true);
  });

  it("blocks step 1 while a list cannot be read", async () => {
    live.errors.CamundaClusterClaim = "forbidden";
    render(<ClusterCreateFlow initial={data({ sections: [] })} />);
    await fillBasics("orders");
    expect(
      screen.getByText(
        "The fleet manager could not read the CamundaClusterClaim objects: forbidden",
      ),
    ).toBeTruthy();
    expect(next().disabled).toBe(true);
  });

  it("refuses a namespace that went away while step 1 was not shown", async () => {
    const { rerender } = render(
      <ClusterCreateFlow initial={data({ sections: [] })} />,
    );
    await fillBasics("orders");
    await userEvent.click(next());
    live.objects.Namespace = [];
    rerender(<ClusterCreateFlow initial={data({ sections: [] })} />);
    await userEvent.click(
      screen.getByRole("button", { name: /Environment and name/ }),
    );
    expect(screen.getAllByText("Choose a namespace").length).toBeGreaterThan(0);
    expect(next().disabled).toBe(true);
  });

  it("keeps the step 1 choices after Next and Back", async () => {
    render(
      <ClusterCreateFlow
        initial={data({
          sections: [],
          environments: [prod({ allowed: true, requiredLabelKeys: ["team"] })],
        })}
      />,
    );
    await userEvent.click(screen.getByRole("radio", { name: /Production/ }));
    await userEvent.type(
      screen.getByRole("textbox", { name: "Name" }),
      "search",
    );
    await userEvent.click(
      screen.getByRole("radio", { name: "Create a namespace" }),
    );
    // The claim name comes first; the name of the new namespace second.
    await userEvent.type(
      screen.getAllByRole("textbox", { name: "Name" })[1],
      "team-search",
    );
    await userEvent.type(
      screen.getByRole("textbox", { name: "team" }),
      "search",
    );
    await userEvent.click(screen.getByRole("radio", { name: /^small/ }));
    await userEvent.click(next());
    expect(
      screen.getByText(
        "This step comes in a later version of the fleet manager.",
      ),
    ).toBeTruthy();
    await userEvent.click(
      screen.getByRole("button", { name: /Environment and name/ }),
    );
    const names = screen
      .getAllByRole("textbox", { name: "Name" })
      .map((i) => (i as HTMLInputElement).value);
    expect(names).toEqual(["search", "team-search"]);
    expect(
      (screen.getByRole("textbox", { name: "team" }) as HTMLInputElement).value,
    ).toBe("search");
    expect(
      (screen.getByRole("radio", { name: /^small/ }) as HTMLInputElement)
        .checked,
    ).toBe(true);
    expect(
      (screen.getByRole("radio", { name: /Production/ }) as HTMLInputElement)
        .checked,
    ).toBe(true);
    expect(next().disabled).toBe(false);
  });
});
