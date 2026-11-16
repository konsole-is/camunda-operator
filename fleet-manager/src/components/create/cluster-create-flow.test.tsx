// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import {
  MANAGEMENT_AUTH_CONFIGS_UNREAD,
  NO_MANAGEMENT_AUTH_CONFIG,
} from "@/lib/clusters/attachments";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import {
  type ClusterCreateData,
  ClusterCreateFlow,
} from "./cluster-create-flow";

const live = vi.hoisted(() => ({
  objects: {} as Record<string, unknown[]>,
  errors: {} as Record<string, string>,
  asked: new Set<string>(),
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, _opts: unknown, initial: unknown[]) => {
    live.asked.add(kind);
    return { items: live.objects[kind] ?? initial, error: live.errors[kind] };
  },
}));

beforeEach(() => {
  live.objects = {};
  live.errors = {};
  live.asked = new Set();
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
      releases: {
        allowed: ["camunda-8-8", "camunda-8-9"],
        default: "camunda-8-9",
      },
      platformConfigRef: "basic",
      ...(namespaceCreation && { namespaceCreation }),
    },
    status: READY(),
  };
}

const release = (name: string, version: string) =>
  ({
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaRelease",
    metadata: { name },
    spec: { version },
  }) as unknown as Resource<"CamundaRelease">;

const platformConfig = (name: string) =>
  ({
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaPlatformConfig",
    metadata: { name },
    spec: {},
  }) as unknown as Resource<"CamundaPlatformConfig">;

const managementAuthConfig = (name: string) =>
  ({
    apiVersion: "core.camunda.io/v1",
    kind: "ManagementAuthConfig",
    metadata: { name },
    spec: {},
  }) as unknown as Resource<"ManagementAuthConfig">;

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
    releases: [
      release("camunda-8-8", "8.8.4"),
      release("camunda-8-9", "8.9.9"),
    ],
    platformConfigs: [platformConfig("basic"), platformConfig("oidc")],
    managementAuthConfigs: [managementAuthConfig("central")],
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

const heading = (name: string) =>
  screen.queryByRole("heading", { level: 2, name });

// toStep fills step 1 and clicks Next until the step of label shows.
async function toStep(label: string, name = "search") {
  await fillBasics(name);
  for (let i = 0; i < 4 && !heading(label); i++) {
    if (next().disabled) break;
    await userEvent.click(next());
  }
  expect(heading(label)).toBeTruthy();
}

const stepButton = (label: string) =>
  screen.getByRole("button", { name: new RegExp(label) });

const footer = () =>
  document.getElementById(next().getAttribute("aria-describedby") ?? "")
    ?.textContent ?? "";

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

  it("puts the section headings of a step under the step heading", () => {
    render(<ClusterCreateFlow initial={data()} />);
    const texts = (level: number) =>
      screen.getAllByRole("heading", { level }).map((h) => h.textContent);
    expect(texts(2)).toEqual(["Environment and name"]);
    expect(texts(3)).toEqual(
      expect.arrayContaining(["Environment", "Namespace", "Preset"]),
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

  it("blocks Next on step 2 while step 1 has errors, and names step 1", async () => {
    const { rerender } = render(
      <ClusterCreateFlow initial={data({ sections: [] })} />,
    );
    await fillBasics("orders");
    await userEvent.click(next());
    live.objects.Namespace = [];
    rerender(<ClusterCreateFlow initial={data({ sections: [] })} />);
    expect(next().disabled).toBe(true);
    expect(
      document.getElementById(next().getAttribute("aria-describedby")!)
        ?.textContent,
    ).toContain("Environment and name: Choose a namespace");
  });

  it("offers Back on the review step", async () => {
    render(<ClusterCreateFlow initial={data({ sections: [] })} />);
    await fillBasics("orders");
    for (let i = 0; i < 4; i++) await userEvent.click(next());
    expect(screen.queryByRole("button", { name: "Next" })).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(document.activeElement).toBe(
      screen.getByRole("heading", { level: 2, name: "Attachments" }),
    );
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

  it("drops the pick of a section that goes away, and blocks the step while the policy still requires that section", async () => {
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
    // The claim controller counts a required section of the policy that no
    // PluginSection holds as one that applies to every claim.
    expect(
      screen.getByText(
        'section "backup-storage" is required, and the claim selects nothing in it',
      ),
    ).toBeTruthy();
    expect(next().disabled).toBe(true);
  });

  it("opens step 3 with the release and platform config of the environment", async () => {
    render(<ClusterCreateFlow initial={data({ accessRequests: [] })} />);
    await toStep("Cluster settings");
    expect(
      (screen.getByRole("combobox", { name: "Release" }) as HTMLSelectElement)
        .value,
    ).toBe("camunda-8-9");
    expect(
      (
        screen.getByRole("combobox", {
          name: "Platform config",
        }) as HTMLSelectElement
      ).value,
    ).toBe("basic");
    expect(next().disabled).toBe(false);
  });

  it("blocks step 3 without a platform config", async () => {
    const without = prod();
    delete without.spec.platformConfigRef;
    render(
      <ClusterCreateFlow
        initial={data({ environments: [without], accessRequests: [] })}
      />,
    );
    await toStep("Cluster settings");
    expect(next().disabled).toBe(true);
    expect(footer()).toContain("Cluster settings: Pick a platform config.");
  });

  it("blocks step 3, not step 1, while the releases cannot be read", async () => {
    live.objects.CamundaRelease = [];
    live.errors.CamundaRelease = "forbidden";
    render(<ClusterCreateFlow initial={data({ accessRequests: [] })} />);
    await toStep("Cluster settings");
    expect(next().disabled).toBe(true);
    expect(footer()).toContain(
      "Cluster settings: The fleet manager could not read the CamundaRelease objects: forbidden",
    );
    expect(footer()).not.toContain("does not exist");
    expect(footer()).not.toContain("Environment and name:");
    expect(
      screen.getByRole("option", { name: "Could not read the releases" }),
    ).toBeTruthy();
  });

  it("blocks step 3 while the platform configs cannot be read, and names no platform config as missing", async () => {
    live.objects.CamundaPlatformConfig = [];
    live.errors.CamundaPlatformConfig = "forbidden";
    render(<ClusterCreateFlow initial={data({ accessRequests: [] })} />);
    await toStep("Cluster settings");
    expect(next().disabled).toBe(true);
    expect(footer()).toContain(
      "Cluster settings: The fleet manager could not read the CamundaPlatformConfig objects: forbidden",
    );
    expect(footer()).not.toContain("does not exist");
    expect(
      screen.getByRole("option", {
        name: "Could not read the platform configs",
      }),
    ).toBeTruthy();
  });

  it("blocks step 4, not step 3, while the ManagementAuthConfigs cannot be read", async () => {
    live.objects.ManagementAuthConfig = [];
    live.errors.ManagementAuthConfig = "forbidden";
    render(<ClusterCreateFlow initial={attachmentData()} />);
    await toStep("Attachments");
    expect(next().disabled).toBe(true);
    expect(footer()).toContain(
      "Attachments: The fleet manager could not read the ManagementAuthConfig objects: forbidden",
    );
    expect(footer()).not.toContain("Cluster settings:");
    expect(screen.queryByText(NO_MANAGEMENT_AUTH_CONFIG)).toBeNull();
    expect(screen.getByText(MANAGEMENT_AUTH_CONFIGS_UNREAD)).toBeTruthy();
  });

  it("keeps the step 3 and 4 picks after Back and Next", async () => {
    render(<ClusterCreateFlow initial={attachmentData()} />);
    await toStep("Cluster settings");
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Release" }),
      "camunda-8-8",
    );
    await userEvent.click(next());
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Back up on a schedule" }),
    );

    await userEvent.click(stepButton("Storage and access"));
    await userEvent.click(next());
    expect(
      (screen.getByRole("combobox", { name: "Release" }) as HTMLSelectElement)
        .value,
    ).toBe("camunda-8-8");
    await userEvent.click(next());
    expect(
      screen
        .getByRole("checkbox", { name: "Back up on a schedule" })
        .getAttribute("aria-checked"),
    ).toBe("true");
    expect(
      (
        screen.getByRole("textbox", {
          name: "Schedule (cron, UTC)",
        }) as HTMLInputElement
      ).value,
    ).toBe("0 2 * * *");
  });

  it("drops the typed Optimize URL once an access option fills it", async () => {
    render(<ClusterCreateFlow initial={attachmentData()} />);
    await toStep("Storage and access");
    await userEvent.click(radio(/No access from outside/));
    await userEvent.click(next());
    await userEvent.click(next());
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Attach Optimize" }),
    );
    await userEvent.type(
      screen.getByRole("textbox", { name: "External URL" }),
      "https://optimize.example.com",
    );

    await userEvent.click(stepButton("Storage and access"));
    await userEvent.click(radio(/^Gateway/));
    await userEvent.click(next());
    await userEvent.click(next());

    // The locked row shows the URL of the draft first, so the typed URL
    // would show here while the draft still held it.
    const group = screen.getByRole("group", { name: "External URL" });
    expect(within(group).getByText("Filled by Access")).toBeTruthy();
    expect(
      within(group).getByText("Known once the access option is ready"),
    ).toBeTruthy();
    expect(
      within(group).queryByText("https://optimize.example.com"),
    ).toBeNull();
  });

  it("drops a whole cluster value once an access option fills it", async () => {
    render(<ClusterCreateFlow initial={attachmentData()} />);
    await toStep("Storage and access");
    await userEvent.click(radio(/No access from outside/));
    await userEvent.click(next());
    await userEvent.type(
      screen.getByRole("textbox", { name: "externalUrl" }),
      "https://orders.example.com",
    );

    await userEvent.click(stepButton("Storage and access"));
    await userEvent.click(radio(/^Gateway/));
    await userEvent.click(next());
    const url = screen.getAllByRole("group", { name: "externalUrl" })[0];
    expect(within(url).getByText("Filled by Access")).toBeTruthy();
    expect(next().disabled).toBe(false);

    await userEvent.click(stepButton("Storage and access"));
    await userEvent.click(radio(/No access from outside/));
    await userEvent.click(next());
    expect(
      (screen.getByRole("textbox", { name: "externalUrl" }) as HTMLInputElement)
        .value,
    ).toBe("");
  });

  it("drops service account annotations once an identity option fills them", async () => {
    render(<ClusterCreateFlow initial={identityData()} />);
    await toStep("Storage and access");
    await userEvent.click(radio(/No identity/));
    await userEvent.click(next());
    const row = () =>
      screen.getAllByRole("group", { name: "serviceAccount.annotations" })[0];
    await userEvent.click(
      within(row()).getByRole("button", { name: "Add an entry" }),
    );
    await userEvent.type(
      within(row()).getByRole("textbox", { name: "Key" }),
      "role",
    );
    await userEvent.type(
      within(row()).getByRole("textbox", { name: "Value" }),
      "orders",
    );

    await userEvent.click(stepButton("Storage and access"));
    await userEvent.click(radio(/^IRSA/));
    await userEvent.click(next());
    expect(within(row()).getByText("Filled by Identity")).toBeTruthy();
    expect(next().disabled).toBe(false);

    await userEvent.click(stepButton("Storage and access"));
    await userEvent.click(radio(/No identity/));
    await userEvent.click(next());
    expect(within(row()).queryByRole("textbox", { name: "Key" })).toBeNull();
  });

  it("stops on a step 2 error that a release pick causes", async () => {
    const pinned: Resource<"PluginRegistration"> = {
      ...gateway,
      metadata: { name: "pinned" },
      spec: {
        ...gateway.spec,
        label: "Pinned",
        parameters: {
          type: "object",
          properties: {
            releaseRef: { type: "string", enum: ["camunda-8-9"] },
          },
        },
      },
    };
    const environment = prod();
    environment.spec.sections = {
      access: { allowed: ["pinned"], default: "pinned", required: true },
    };
    render(
      <ClusterCreateFlow
        initial={data({
          environments: [environment],
          registrations: [pinned],
          accessRequests: [],
        })}
      />,
    );
    await toStep("Cluster settings");
    expect(next().disabled).toBe(false);
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Release" }),
      "camunda-8-8",
    );
    expect(next().disabled).toBe(true);
    expect(footer()).toContain("Storage and access:");
  });

  it("blocks step 4 on a count with an error until the schedule is turned off", async () => {
    render(<ClusterCreateFlow initial={attachmentData()} />);
    await toStep("Attachments");
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Back up on a schedule" }),
    );
    expect(next().disabled).toBe(false);
    await userEvent.type(
      screen.getByRole("spinbutton", { name: "Completed backups to keep" }),
      "1.5",
    );
    expect(next().disabled).toBe(true);
    expect(footer()).toContain(
      "Attachments: Completed backups to keep: Enter a whole number",
    );
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Back up on a schedule" }),
    );
    expect(next().disabled).toBe(false);
  });

  it("asks for nothing on a new namespace with no Secrets to read", async () => {
    render(
      <ClusterCreateFlow
        initial={data({
          sections: [],
          environments: [prod({ allowed: true })],
        })}
      />,
    );
    await userEvent.click(screen.getByRole("radio", { name: /Production/ }));
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "c1");
    await userEvent.click(
      screen.getByRole("radio", { name: "Create a namespace" }),
    );
    await userEvent.type(
      screen.getAllByRole("textbox", { name: "Name" })[1],
      "team-new",
    );
    await userEvent.click(screen.getByRole("radio", { name: /^small/ }));
    await userEvent.click(next());
    await userEvent.click(next());
    expect(heading("Cluster settings")).toBeTruthy();
    expect(live.asked.has("Secret")).toBe(false);
  });

  it("names a Secret read error and does not block step 3", async () => {
    live.errors.Secret = "forbidden";
    render(<ClusterCreateFlow initial={data({ accessRequests: [] })} />);
    await toStep("Cluster settings");
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read Secret: forbidden",
    );
    expect(next().disabled).toBe(false);
  });
});

// attachmentData gives step 2 a backup storage option, Elasticsearch as
// secondary storage, and an access section whose Gateway option gives the
// Optimize URL. Each section starts with its option picked.
function attachmentData(): ClusterCreateData {
  const slotted = (
    name: string,
    label: string,
    slot: string,
    order: number,
    emptyOptionLabel?: string,
  ) =>
    ({
      apiVersion: "plugins.camunda.io/v1alpha1",
      kind: "PluginSection",
      metadata: { name },
      spec: {
        appliesTo: ["CamundaClusterClaim"],
        label,
        selection: "One",
        order,
        slot,
        ...(emptyOptionLabel && { emptyOptionLabel }),
      },
    }) as unknown as Resource<"PluginSection">;
  const bucket: Resource<"PluginRegistration"> = {
    ...gateway,
    metadata: { name: "bucket" },
    spec: {
      ...gateway.spec,
      sections: ["backup-storage"],
      label: "Bucket",
      parameterValues: {},
    },
  };
  const elasticsearch: Resource<"PluginRegistration"> = {
    ...gateway,
    metadata: { name: "elasticsearch" },
    spec: {
      ...gateway.spec,
      sections: ["secondary-storage"],
      label: "New Elasticsearch cluster",
      controller: { name: "camunda-operator", version: "0.0.0" },
      parameters: {
        type: "object",
        properties: { preset: { type: "string", title: "Preset" } },
      },
      parameterValues: {},
    },
  };
  const access: Resource<"PluginRegistration"> = {
    ...gateway,
    spec: {
      ...gateway.spec,
      outputs: [{ key: "externalUrl" }, { key: "optimizeUrl" }],
    },
  };
  const environment = prod();
  environment.spec.sections = {
    access: { allowed: ["gateway"], default: "gateway" },
    "backup-storage": { allowed: ["bucket"], default: "bucket" },
    "secondary-storage": {
      allowed: ["elasticsearch"],
      default: "elasticsearch",
      required: true,
    },
  };
  environment.spec.presets = {
    ...environment.spec.presets,
    elasticsearchCluster: ["search-small"],
  };
  environment.spec.parameters = {
    elasticsearch: { preset: { value: "search-small" } },
  };
  return data({
    environments: [environment],
    sections: [
      slotted(
        "backup-storage",
        "Backup storage",
        "backupStorage",
        40,
        "No backups",
      ),
      slotted("secondary-storage", "Secondary storage", "secondaryStorage", 50),
      slotted(
        "access",
        "Access",
        "externalAccess",
        60,
        "No access from outside",
      ),
    ],
    registrations: [access, bucket, elasticsearch],
    accessRequests: [],
    elasticsearchPresets: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "ElasticsearchClusterPreset",
        metadata: { name: "search-small" },
        spec: { cluster: { replicas: 1 } },
      } as unknown as Resource<"ElasticsearchClusterPreset">,
    ],
  });
}

// identityData is attachmentData with an identity section that fills the
// service account annotations and offers no option by default.
function identityData(): ClusterCreateData {
  const base = attachmentData();
  const identity = {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name: "identity" },
    spec: {
      appliesTo: ["CamundaClusterClaim"],
      label: "Identity",
      selection: "One",
      order: 70,
      slot: "workloadIdentity",
      emptyOptionLabel: "No identity",
    },
  } as unknown as Resource<"PluginSection">;
  const irsa: Resource<"PluginRegistration"> = {
    ...gateway,
    metadata: { name: "irsa" },
    spec: {
      ...gateway.spec,
      sections: ["identity"],
      label: "IRSA",
      parameterValues: {},
    },
  };
  const [environment] = base.environments;
  environment.spec.sections = {
    ...environment.spec.sections,
    identity: { allowed: ["irsa"] },
  };
  return {
    ...base,
    sections: [...base.sections, identity],
    registrations: [...base.registrations, irsa],
  };
}
