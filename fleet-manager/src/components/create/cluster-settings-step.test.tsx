// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import { createReleaseChoices } from "@/components/cluster/edit/release-options";
import { type ClusterDraft, draftCluster } from "@/lib/clusters/draft";
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
) {
  const draft = over.draft ?? draftOf();
  const drafted = draftCluster(draft, [preset], releases, environment);
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

const select = (name: string) =>
  screen.getByRole<HTMLSelectElement>("combobox", { name });

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
});
