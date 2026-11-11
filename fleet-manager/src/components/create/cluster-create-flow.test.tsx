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
      sections: {
        access: { allowed: ["gateway", "internal"], default: "gateway" },
      },
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
    order: 60,
    emptyOptionLabel: "No access from outside",
  },
};

const backupSection: Resource<"PluginSection"> = {
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginSection",
  metadata: { name: "backup-storage" },
  spec: {
    appliesTo: ["CamundaClusterClaim"],
    label: "Backup storage",
    selection: "One",
    order: 40,
    emptyOptionLabel: "No backups",
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

const internal: Resource<"PluginRegistration"> = {
  ...gateway,
  metadata: { name: "internal" },
  spec: {
    ...gateway.spec,
    label: "Internal",
    parameterValues: {
      hostnamePattern: "{name}.{namespace}.internal.example.com",
    },
  },
};

function accessRequest(
  namespace: string,
  claim: string,
  url: string,
): Resource<"PluginRequest"> {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRequest",
    metadata: { namespace, name: `${claim}-access` },
    spec: {
      claimRef: { kind: "CamundaClusterClaim", name: claim },
      registration: "gateway",
      section: "access",
    },
    status: { outputs: { externalUrl: url } },
  } as unknown as Resource<"PluginRequest">;
}

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
    registrations: [gateway, internal],
    accessRequests: [billingAccess],
    elasticsearchPresets: [],
    databaseServerPresets: [],
    secondaryStorageConfigs: [],
    objectStorageConfigs: [],
    ...over,
  };
}

const radio = (name: RegExp) =>
  screen.getByRole("radio", { name }) as HTMLInputElement;

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
    expect(screen.queryByRole("textbox", { name: "Name" })).toBeNull();
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

  it("shows the sections of step 2 after step 1", async () => {
    render(
      <ClusterCreateFlow
        initial={data({ sections: [accessSection, backupSection] })}
      />,
    );
    await fillBasics("search");
    await userEvent.click(next());

    const legends = screen
      .getAllByRole("group")
      .map((g) => g.querySelector("legend")?.textContent);
    expect(legends).toEqual(
      expect.arrayContaining(["Backup storage", "Access"]),
    );
    expect(radio(/^Gateway/).checked).toBe(true);
  });

  it("shows the deletion default again once the pick that cannot keep goes", async () => {
    const keeps = prod();
    keeps.spec.deletionPolicy = "Orphan";
    render(<ClusterCreateFlow initial={data({ environments: [keeps] })} />);
    await fillBasics("search");
    await userEvent.click(next());

    expect(
      screen.getByRole("group", { name: "When the cluster is deleted" }),
    ).toBeTruthy();
    expect(radio(/Keep the resources/).disabled).toBe(true);
    expect(radio(/Delete the resources/).checked).toBe(true);
    expect(screen.getByText("Gateway (Access)")).toBeTruthy();

    await userEvent.click(radio(/No access from outside/));

    expect(radio(/Keep the resources/).checked).toBe(true);
  });

  it("checks the hosts of the picked access option", async () => {
    const taken =
      "The host c1.team-a.internal.example.com is taken by team-b/x. Choose another name.";
    const requests = [
      accessRequest("team-b", "x", "https://c1.team-a.internal.example.com"),
    ];
    const { rerender } = render(
      <ClusterCreateFlow initial={data({ accessRequests: requests })} />,
    );
    await fillBasics("c1");
    await userEvent.click(next());

    await userEvent.click(radio(/^Internal/));
    expect(screen.getAllByText(taken).length).toBeGreaterThan(0);
    expect(next().disabled).toBe(true);

    await userEvent.click(radio(/No access from outside/));
    expect(screen.queryByText(taken)).toBeNull();
    expect(next().disabled).toBe(false);

    live.objects.PluginRequest = [
      ...requests,
      accessRequest("team-c", "y", "https://c1.team-a.example.com"),
    ];
    rerender(
      <ClusterCreateFlow initial={data({ accessRequests: requests })} />,
    );
    await userEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.queryByText(/is taken by/)).toBeNull();
    expect(next().disabled).toBe(false);
  });

  it("blocks step 1 on the taken host of the default option before step 2", async () => {
    render(
      <ClusterCreateFlow
        initial={data({
          accessRequests: [
            accessRequest("team-c", "y", "https://c1.team-a.example.com"),
          ],
        })}
      />,
    );
    await fillBasics("c1");

    expect(next().disabled).toBe(true);
  });

  it("keeps the step 2 picks after Back and Next", async () => {
    render(<ClusterCreateFlow initial={data({ accessRequests: [] })} />);
    await fillBasics("search");
    await userEvent.click(next());
    await userEvent.click(radio(/No access from outside/));

    await userEvent.click(screen.getByRole("button", { name: "Back" }));
    await userEvent.click(next());

    expect(radio(/No access from outside/).checked).toBe(true);
    expect(radio(/^Gateway/).checked).toBe(false);
  });

  it("clears the step 2 picks when the environment changes", async () => {
    const staging = {
      ...prod(),
      metadata: { name: "staging" },
      spec: { ...prod().spec, displayName: "Staging" },
    };
    render(
      <ClusterCreateFlow
        initial={data({ environments: [prod(), staging], accessRequests: [] })}
      />,
    );
    await fillBasics("search");
    await userEvent.click(next());
    await userEvent.click(radio(/No access from outside/));
    await userEvent.click(screen.getByRole("button", { name: "Back" }));

    await userEvent.click(screen.getByRole("radio", { name: /Staging/ }));
    await userEvent.click(screen.getByRole("radio", { name: /Production/ }));
    await userEvent.selectOptions(screen.getByRole("combobox"), "team-a");
    await userEvent.click(next());

    expect(radio(/^Gateway/).checked).toBe(true);
  });

  it("blocks step 2 when a storage config list cannot be read", async () => {
    live.errors.SecondaryStorageConfig = "forbidden";
    render(<ClusterCreateFlow initial={data({ accessRequests: [] })} />);
    await fillBasics("search");
    expect(next().disabled).toBe(false);

    await userEvent.click(next());

    expect(
      screen.getAllByText(
        "The fleet manager could not read the SecondaryStorageConfig objects: forbidden",
      ).length,
    ).toBeGreaterThan(0);
    expect(next().disabled).toBe(true);
  });

  it("preselects the Default registration without an Environment", async () => {
    const fallback = {
      ...gateway,
      spec: { ...gateway.spec, mode: "Default" as const },
    };
    render(
      <ClusterCreateFlow
        initial={data({
          environments: [],
          registrations: [fallback, internal],
          accessRequests: [],
        })}
      />,
    );
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "c1");
    await userEvent.selectOptions(screen.getByRole("combobox"), "team-a");
    await userEvent.click(screen.getByRole("radio", { name: /^small/ }));
    await userEvent.click(next());

    expect(radio(/^Gateway/).checked).toBe(true);
    expect(radio(/^Internal/).checked).toBe(false);
  });

  it("blocks step 2 when a list that both steps read cannot be read", async () => {
    render(<ClusterCreateFlow initial={data({ accessRequests: [] })} />);
    await fillBasics("search");
    await userEvent.click(next());
    expect(next().disabled).toBe(false);

    live.errors.PluginRegistration = "forbidden";
    await userEvent.click(radio(/No access from outside/));

    expect(
      screen.getAllByText(
        "The fleet manager could not read the PluginRegistration objects: forbidden",
      ).length,
    ).toBeGreaterThan(0);
    expect(next().disabled).toBe(true);
  });

  it("drops the pick of a section that goes away", async () => {
    const initial = data({
      sections: [accessSection, backupSection],
      environments: [
        {
          ...prod(),
          spec: {
            ...prod().spec,
            sections: {
              ...prod().spec.sections,
              "backup-storage": { allowed: ["internal"], required: true },
            },
          },
        },
      ],
      registrations: [
        gateway,
        {
          ...internal,
          spec: {
            ...internal.spec,
            sections: ["access", "backup-storage"],
          },
        },
      ],
      accessRequests: [],
    });
    const { rerender } = render(<ClusterCreateFlow initial={initial} />);
    await fillBasics("search");
    await userEvent.click(next());
    await userEvent.click(
      within(screen.getByRole("group", { name: /Backup storage/ })).getByRole(
        "radio",
        { name: /^Internal/ },
      ),
    );
    expect(next().disabled).toBe(false);

    live.objects.PluginSection = [accessSection];
    rerender(<ClusterCreateFlow initial={initial} />);

    expect(screen.queryByText(/Choose another option/)).toBeNull();
    expect(next().disabled).toBe(false);
  });
});
