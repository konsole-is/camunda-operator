// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { type ComponentProps, useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import { createReleaseChoices } from "@/components/cluster/edit/release-options";
import {
  type ClusterDraft,
  draftCluster,
  withClaimValue,
} from "@/lib/clusters/draft";
import { clusterFields } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { ClusterSettingsStep } from "./cluster-settings-step";

afterEach(cleanup);

const release = (name: string, version: string) =>
  ({
    kind: "CamundaRelease",
    metadata: { name },
    spec: { version },
  }) as unknown as Resource<"CamundaRelease">;

const releases = [
  release("camunda-8-8", "8.8.4"),
  release("camunda-8-9", "8.9.9"),
];

function prod(over: Partial<Environment["spec"]> = {}): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "prod" },
    spec: {
      displayName: "Production",
      releases: {
        allowed: ["camunda-8-8", "camunda-8-9"],
        default: "camunda-8-9",
      },
      platformConfigRef: "basic",
      ...over,
    },
  };
}

const accessSection = {
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginSection",
  metadata: { name: "access" },
  spec: {
    appliesTo: ["CamundaClusterClaim"],
    label: "Access",
    selection: "One",
    slot: "externalAccess",
  },
} as unknown as Resource<"PluginSection">;

const preset = {
  apiVersion: "core.camunda.io/v1",
  kind: "CamundaClusterPreset",
  metadata: { name: "small" },
  spec: { cluster: {} },
} as unknown as Resource<"CamundaClusterPreset">;

function draftOf(
  spec: ClusterDraft["claim"]["spec"] = {},
  access = true,
): ClusterDraft {
  return {
    environment: "prod",
    namespace: { kind: "existing", name: "team-a" },
    claim: {
      metadata: { name: "orders" },
      spec: { presetRef: "small", ...spec },
    },
    ...(access && {
      storage: { access: [{ section: "access", registration: "gateway" }] },
    }),
  };
}

function renderStep(
  over: Partial<ComponentProps<typeof ClusterSettingsStep>> = {},
  environment: Environment = prod(),
  presets = [preset],
) {
  const draft = over.draft ?? draftOf();
  const drafted = draftCluster(draft, presets, releases, environment);
  const fields = clusterFields({
    cluster: drafted.cluster,
    merged: drafted.merged,
    claim: drafted.claim as unknown as Resource<"CamundaClusterClaim">,
    sections: [accessSection],
    registrations: [],
  });
  const props: ComponentProps<typeof ClusterSettingsStep> = {
    draft,
    environment,
    releases: createReleaseChoices(releases, environment),
    platformConfigs: ["basic", "oidc"],
    presets: presets.map((p) => p.metadata?.name ?? ""),
    fields,
    merged: drafted.merged,
    secrets: [],
    pick: vi.fn(),
    set: vi.fn(),
    reset: vi.fn(),
    invalid: vi.fn(),
    ...over,
  };
  render(<ClusterSettingsStep {...props} />);
  return props;
}

// LiveStep holds the draft as the create flow does, so that a change
// reaches the fields of the step.
function LiveStep({
  initial,
  set,
}: {
  initial: ClusterDraft;
  set: ComponentProps<typeof ClusterSettingsStep>["set"];
}) {
  const [draft, setDraft] = useState(initial);
  const drafted = draftCluster(draft, [preset], releases, prod());
  const fields = clusterFields({
    cluster: drafted.cluster,
    merged: drafted.merged,
    claim: drafted.claim as unknown as Resource<"CamundaClusterClaim">,
    sections: [accessSection],
    registrations: [],
  });
  return (
    <ClusterSettingsStep
      draft={draft}
      environment={prod()}
      releases={createReleaseChoices(releases, prod())}
      platformConfigs={["basic"]}
      presets={["small"]}
      fields={fields}
      merged={drafted.merged}
      secrets={[]}
      pick={vi.fn()}
      set={(steps, value) => {
        set(steps, value);
        setDraft((d) => withClaimValue(d, steps, value));
      }}
      reset={vi.fn()}
      invalid={vi.fn()}
    />
  );
}

const select = (name: string) =>
  screen.getByRole<HTMLSelectElement>("combobox", { name });

const section = (label: string) =>
  screen.getByRole("region", { name: `${label} settings` });

// rowOf is the button in the heading of the settings group of label, which
// opens and closes the group.
const rowOf = (label: string) =>
  within(
    within(section(label)).getByRole("heading", { level: 3, name: label }),
  ).getByRole("button");

const isOpen = (label: string) =>
  rowOf(label).getAttribute("aria-expanded") === "true";

const toggle = (label: string) => userEvent.click(rowOf(label));

describe("ClusterSettingsStep", () => {
  it("preselects the default release and platform config of the environment", () => {
    renderStep();
    expect(select("Release").value).toBe("camunda-8-9");
    expect(
      screen.getByRole("option", { name: "8.9.9 (camunda-8-9), Default" }),
    ).toBeTruthy();
    expect(select("Platform config").value).toBe("basic");
    expect(
      screen.getByText("Default from the Production environment."),
    ).toBeTruthy();
  });

  it("sets an overridden release", async () => {
    const { pick } = renderStep();
    await userEvent.selectOptions(select("Release"), "camunda-8-8");
    expect(pick).toHaveBeenCalledWith("releaseRef", "camunda-8-8");
  });

  it("sets an overridden platform config", async () => {
    const { pick } = renderStep();
    await userEvent.selectOptions(select("Platform config"), "oidc");
    expect(pick).toHaveBeenCalledWith("platformConfigRef", "oidc");
  });

  it("shows a picked platform config without the default note", () => {
    renderStep({ draft: draftOf({ platformConfigRef: "oidc" }) });
    expect(select("Platform config").value).toBe("oidc");
    expect(
      screen.queryByText("Default from the Production environment."),
    ).toBeNull();
  });

  it("asks for a release when none is known", () => {
    renderStep(
      {},
      prod({ releases: { allowed: ["camunda-8-8", "camunda-8-9"] } }),
    );
    expect(select("Release").value).toBe("");
    expect(
      (
        screen.getByRole("option", {
          name: "Pick a release",
        }) as HTMLOptionElement
      ).selected,
    ).toBe(true);
  });

  it("asks for a platform config when none is known", () => {
    renderStep({}, prod({ platformConfigRef: undefined }));
    expect(select("Platform config").value).toBe("");
    expect(
      screen.getByRole("option", { name: "Pick a platform config" }),
    ).toBeTruthy();
  });

  it("says which lists could not be read, not that a pick is missing", () => {
    renderStep({
      releases: [],
      platformConfigs: [],
      unread: ["CamundaRelease", "CamundaPlatformConfig"],
    });
    expect(
      screen.getByRole("option", { name: "Could not read the releases" }),
    ).toBeTruthy();
    expect(
      screen.getByRole("option", {
        name: "Could not read the platform configs",
      }),
    ).toBeTruthy();
    expect(screen.queryByRole("option", { name: /^Pick a/ })).toBeNull();
  });

  it("leaves the preset, release, and platform config out of the whole cluster settings", () => {
    renderStep();
    const whole = screen.getByRole("region", {
      name: "Whole cluster settings",
    });
    for (const name of ["presetRef", "releaseRef", "platformConfigRef"])
      expect(within(whole).queryByRole("group", { name })).toBeNull();
    expect(within(whole).queryByText("Change it in the Claim tab.")).toBeNull();
  });

  it("does not say that the volumes of a new cluster cannot shrink", async () => {
    const large = {
      ...preset,
      spec: { cluster: { zeebe: { storageSize: "32Gi" } } },
    } as unknown as Resource<"CamundaClusterPreset">;
    renderStep({}, prod(), [large]);
    await toggle("Brokers");
    const brokers = screen.getByRole("region", { name: "Brokers settings" });
    const amount = within(
      within(brokers).getAllByRole("group", { name: "storageSize" })[0],
    ).getByRole("textbox", { name: "Amount" });
    await userEvent.clear(amount);
    await userEvent.type(amount, "16");
    expect(screen.queryByText(/Volumes cannot shrink/)).toBeNull();
  });

  it("keeps a storage size that the preset gives as a number a number", async () => {
    const numeric = {
      ...preset,
      spec: { cluster: { zeebe: { storageSize: 10737418240 } } },
    } as unknown as Resource<"CamundaClusterPreset">;
    const { set } = renderStep({}, prod(), [numeric]);
    await toggle("Brokers");
    const brokers = screen.getByRole("region", { name: "Brokers settings" });
    const amount = within(
      within(brokers).getAllByRole("group", { name: "storageSize" })[0],
    ).getByRole("textbox", { name: "Amount" });
    await userEvent.clear(amount);
    await userEvent.type(amount, "21474836480");
    expect(set).toHaveBeenLastCalledWith(
      [
        { field: "spec" },
        { field: "cluster" },
        { field: "zeebe" },
        { field: "storageSize" },
      ],
      21474836480,
    );
  });

  it("turns on Connectors that the preset leaves off", async () => {
    const { set } = renderStep();
    await toggle("Connectors");
    const connectors = screen.getByRole("region", {
      name: "Connectors settings",
    });
    await userEvent.click(
      within(connectors).getByRole("checkbox", { name: "enabled" }),
    );
    expect(set).toHaveBeenLastCalledWith(
      [
        { field: "spec" },
        { field: "cluster" },
        { field: "connectors" },
        { field: "enabled" },
      ],
      true,
    );
  });

  it("keeps the Connectors settings while the draft turns them off", () => {
    const connectors = {
      ...preset,
      spec: { cluster: { connectors: { enabled: true } } },
    } as unknown as Resource<"CamundaClusterPreset">;
    renderStep(
      { draft: draftOf({ cluster: { connectors: { enabled: false } } }) },
      prod(),
      [connectors],
    );
    expect(
      screen.getByRole("region", { name: "Connectors settings" }),
    ).toBeTruthy();
  });

  it("shows the whole cluster settings first", () => {
    renderStep();
    const groups = screen
      .getAllByRole("region", { name: / settings$/ })
      .map((r) => r.getAttribute("aria-label"));
    expect(groups[0]).toBe("Whole cluster settings");
    expect(groups).toContain("Brokers settings");
  });

  it("locks a field that the access option fills", () => {
    renderStep();
    const whole = screen.getByRole("region", {
      name: "Whole cluster settings",
    });
    expect(within(whole).getByText("Filled by Access")).toBeTruthy();
  });

  it("leaves the field open without an access option", () => {
    renderStep({ draft: draftOf({}, false) });
    const whole = screen.getByRole("region", {
      name: "Whole cluster settings",
    });
    expect(within(whole).queryByText("Filled by Access")).toBeNull();
  });

  it("sends a settings change with steps from the root of the claim", async () => {
    const set = vi.fn();
    renderStep({ draft: draftOf({}, false), set });
    await toggle("Whole cluster");
    const whole = screen.getByRole("region", {
      name: "Whole cluster settings",
    });
    await userEvent.type(
      within(whole).getByRole("textbox", { name: "externalUrl" }),
      "h",
    );
    expect(set).toHaveBeenCalled();
    expect(set.mock.calls[0][0].slice(0, 2)).toEqual([
      { field: "spec" },
      { field: "cluster" },
    ]);
  });

  it("starts each settings group closed under the release and platform config", () => {
    renderStep();
    const groups = screen.getAllByRole("region", { name: / settings$/ });
    expect(groups.length).toBeGreaterThan(1);
    for (const group of groups)
      expect(
        within(group)
          .getByRole("heading", { level: 3 })
          .querySelector("button")
          ?.getAttribute("aria-expanded"),
      ).toBe("false");
    expect(select("Release")).toBeTruthy();
    expect(select("Platform config")).toBeTruthy();
  });

  it("says first in the body of a group where the values of its settings come from", async () => {
    const large = {
      ...preset,
      spec: { cluster: { zeebe: { replicas: 3, partitions: 3 } } },
    } as unknown as Resource<"CamundaClusterPreset">;
    renderStep({}, prod(), [large]);
    await toggle("Brokers");
    const body = document.getElementById(
      rowOf("Brokers").getAttribute("aria-controls")!,
    );
    expect(body?.querySelector("p")?.textContent).toMatch(
      /^\d+ settings: 2 from the preset, /,
    );
  });

  it("names the preset that sets the values and says that a change stays with this cluster", () => {
    renderStep();
    expect(
      screen.getByText(
        (_, element) =>
          element?.tagName === "P" &&
          element.textContent ===
            "Your admin made the preset small. It sets each value that shows “From preset”. A change here applies to this cluster only. It does not change the preset.",
      ),
    ).toBeTruthy();
  });

  it("shows no preset note without a preset", () => {
    renderStep({ draft: draftOf({ presetRef: undefined }) });
    expect(screen.queryByText(/Your admin made the preset/)).toBeNull();
  });

  it("shows no preset note for a preset that the live list does not hold", () => {
    renderStep({}, prod(), []);
    expect(screen.queryByText(/Your admin made the preset/)).toBeNull();
  });

  it("names each settings group as a heading", () => {
    renderStep();
    for (const name of ["Whole cluster", "Brokers", "Gateway", "Connectors"])
      expect(screen.getByRole("heading", { level: 3, name })).toBeTruthy();
  });

  it("counts a changed setting on its group at once", async () => {
    render(<LiveStep initial={draftOf({}, false)} set={vi.fn()} />);
    expect(rowOf("Brokers")).toBe(
      screen.getByRole("button", { name: "Brokers 0 changed in the claim" }),
    );
    await toggle("Brokers");
    await userEvent.type(
      within(
        within(section("Brokers")).getAllByRole("group", {
          name: "replicas",
        })[0],
      ).getByRole("spinbutton"),
      "5",
    );
    expect(rowOf("Brokers")).toBe(
      screen.getByRole("button", { name: "Brokers 1 changed in the claim" }),
    );
  });

  it("keeps a typed value through a close and an open, and sends it to the claim", async () => {
    const set = vi.fn();
    render(<LiveStep initial={draftOf({}, false)} set={set} />);
    const url = () =>
      within(section("Whole cluster")).getByRole<HTMLInputElement>("textbox", {
        name: "externalUrl",
      });
    await toggle("Whole cluster");
    await userEvent.type(url(), "https://orders.example.com");
    await toggle("Whole cluster");
    expect(isOpen("Whole cluster")).toBe(false);
    await toggle("Whole cluster");
    expect(url().value).toBe("https://orders.example.com");
    expect(set).toHaveBeenLastCalledWith(
      [{ field: "spec" }, { field: "cluster" }, { field: "externalUrl" }],
      "https://orders.example.com",
    );
  });

  it("opens the group of a setting with a field error and shows the error at its row", () => {
    renderStep({
      draft: draftOf({}, false),
      errors: [
        { path: "spec.cluster.externalUrl", message: "The URL is taken." },
      ],
    });
    const whole = section("Whole cluster");
    expect(isOpen("Whole cluster")).toBe(true);
    expect(
      within(
        within(whole).getAllByRole("group", { name: "externalUrl" })[0],
      ).getByText("The URL is taken."),
    ).toBeTruthy();
    expect(isOpen("Brokers")).toBe(false);
  });
});
