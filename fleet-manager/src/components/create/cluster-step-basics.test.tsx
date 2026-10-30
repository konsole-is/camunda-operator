// @vitest-environment jsdom
import type { V1Namespace } from "@kubernetes/client-node";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { type ClusterDraft, offeredPresets } from "./cluster-draft";
import { ClusterStepBasics } from "./cluster-step-basics";
import { type DraftPath, useCreateFlow } from "./use-create-flow";

afterEach(cleanup);

function environment(
  name: string,
  spec: Partial<Environment["spec"]> = {},
  notReady?: string,
): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name },
    spec,
    ...(notReady && {
      status: {
        conditions: [
          {
            type: "Ready",
            status: "False" as const,
            reason: "InvalidReference",
            message: notReady,
            lastTransitionTime: "2026-10-30T00:00:00Z",
          },
        ],
      },
    }),
  };
}

function namespace(name: string, env?: string): V1Namespace {
  return {
    metadata: {
      name,
      ...(env && { labels: { [ENVIRONMENT_LABEL]: env } }),
    },
  };
}

function preset(name: string): CamundaClusterPreset {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaClusterPreset",
    metadata: { name },
    spec: { cluster: {} },
  };
}

const presets = [preset("small"), preset("large")];
const namespaces = [namespace("team-a", "prod"), namespace("sandbox", "dev")];

function Harness({
  environments,
  spy,
}: {
  environments: Environment[];
  spy: (path: DraftPath, value: unknown) => void;
}) {
  const flow = useCreateFlow<ClusterDraft>({
    steps: [{ id: "basics", label: "Basics" }],
    initial: { claim: { metadata: { name: "" }, spec: {} } },
  });
  const picked = environments.find(
    (e) => e.metadata.name === flow.draft.environment,
  );
  return (
    <ClusterStepBasics
      draft={flow.draft}
      set={(path, value) => {
        spy(path, value);
        flow.set(path, value);
      }}
      environments={environments}
      counts={{}}
      namespaces={namespaces}
      presets={offeredPresets(presets, picked)}
      usage={new Map()}
      nameErrors={[]}
      hosts={[]}
    />
  );
}

describe("ClusterStepBasics", () => {
  it("offers to create a namespace with two required label keys", async () => {
    const prod = environment("prod", {
      namespaceCreation: {
        allowed: true,
        labels: { "cost-center": "4711" },
        requiredLabelKeys: ["team", "owner"],
      },
      presets: { camundaCluster: ["small"] },
    });
    render(<Harness environments={[prod]} spy={vi.fn()} />);
    await userEvent.click(screen.getByRole("radio", { name: /prod/ }));
    await userEvent.click(
      screen.getByRole("radio", { name: "Create a namespace" }),
    );
    expect(screen.getByRole("textbox", { name: "team" })).toBeTruthy();
    expect(screen.getByRole("textbox", { name: "owner" })).toBeTruthy();
    expect(screen.getByText("cost-center=4711")).toBeTruthy();
    expect(screen.getByText(`${ENVIRONMENT_LABEL}=prod`)).toBeTruthy();
  });

  it("offers only the namespaces of an environment that allows no creation", async () => {
    render(
      <Harness
        environments={[environment("prod"), environment("dev")]}
        spy={vi.fn()}
      />,
    );
    await userEvent.click(screen.getByRole("radio", { name: /prod/ }));
    expect(
      screen.queryByRole("radio", { name: "Create a namespace" }),
    ).toBeNull();
    const options = within(screen.getByRole("combobox"))
      .getAllByRole("option")
      .map((o) => o.textContent);
    expect(options).toContain("team-a");
    expect(options).not.toContain("sandbox");
  });

  it("disables an environment that is not ready", async () => {
    const spy = vi.fn();
    render(
      <Harness
        environments={[environment("prod", {}, "Release 8.9-old is missing")]}
        spy={spy}
      />,
    );
    const card = screen.getByRole("radio", { name: /prod/ });
    expect(card.getAttribute("aria-disabled")).toBe("true");
    expect(
      screen.getByText("Not ready: Release 8.9-old is missing"),
    ).toBeTruthy();
    await userEvent.click(card);
    expect(spy).not.toHaveBeenCalled();
  });

  it("waits for an environment before the namespace and the preset", () => {
    render(<Harness environments={[environment("prod")]} spy={vi.fn()} />);
    expect(screen.getAllByText("Choose an environment first.")).toHaveLength(2);
  });

  it("says that the environment allows no preset", async () => {
    render(
      <Harness
        environments={[environment("prod", { displayName: "Production" })]}
        spy={vi.fn()}
      />,
    );
    await userEvent.click(screen.getByRole("radio", { name: /Production/ }));
    expect(
      screen.getByText(
        "The environment Production allows no cluster preset. Ask your admin to add one.",
      ),
    ).toBeTruthy();
  });

  it("clears the namespace and a preset that the new environment does not offer", async () => {
    const spy = vi.fn();
    render(
      <Harness
        environments={[
          environment("prod", { presets: { camundaCluster: ["small"] } }),
          environment("dev", { presets: { camundaCluster: ["large"] } }),
        ]}
        spy={spy}
      />,
    );
    await userEvent.click(screen.getByRole("radio", { name: /prod/ }));
    await userEvent.click(screen.getByRole("radio", { name: /^small/ }));
    spy.mockClear();
    await userEvent.click(screen.getByRole("radio", { name: /dev/ }));
    expect(spy.mock.calls).toEqual([
      [["environment"], "dev"],
      [["namespace"], undefined],
      [["claim", "spec", "presetRef"], undefined],
    ]);
  });

  it("shows no environment choice without any Environment", () => {
    render(<Harness environments={[]} spy={vi.fn()} />);
    expect(screen.queryByRole("radiogroup", { name: "Environment" })).toBeNull();
    const options = within(screen.getByRole("combobox"))
      .getAllByRole("option")
      .map((o) => o.textContent);
    expect(options).toEqual(["Pick a namespace", "team-a", "sandbox"]);
    expect(screen.getByRole("radio", { name: /^small/ })).toBeTruthy();
    expect(screen.getByRole("radio", { name: /^large/ })).toBeTruthy();
  });

  it("sets the name and the preset of the claim", async () => {
    const spy = vi.fn();
    render(<Harness environments={[]} spy={spy} />);
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "ab");
    await userEvent.click(screen.getByRole("radio", { name: /^large/ }));
    expect(spy).toHaveBeenCalledWith(["claim", "metadata", "name"], "ab");
    expect(spy).toHaveBeenLastCalledWith(
      ["claim", "spec", "presetRef"],
      "large",
    );
  });
});
