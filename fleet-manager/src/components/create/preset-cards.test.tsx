// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import { PresetCards, presetFacts } from "./preset-cards";

afterEach(cleanup);

function preset(
  name: string,
  zeebe?: CamundaClusterPreset["spec"]["cluster"]["zeebe"],
  annotations?: Record<string, string>,
): CamundaClusterPreset {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaClusterPreset",
    metadata: { name, ...(annotations && { annotations }) },
    spec: { cluster: { ...(zeebe && { zeebe }) } },
  };
}

const small = preset("small", {
  replicas: 3,
  partitions: 6,
  resources: { requests: { cpu: "500m", memory: "2Gi" } },
});
const large = preset("large");
const described = preset(
  "medium",
  { replicas: 3 },
  { "kubernetes.io/description": "For production workloads." },
);

describe("presetFacts", () => {
  it("gives the facts that the preset sets, in order", () => {
    expect(presetFacts(small)).toEqual([
      { label: "Brokers", value: "3" },
      { label: "Partitions", value: "6" },
      { label: "CPU", value: "500m" },
      { label: "Memory", value: "2Gi" },
    ]);
  });

  it("gives no facts for a preset that sets none", () => {
    expect(presetFacts(large)).toEqual([]);
  });
});

describe("PresetCards", () => {
  it("shows the facts and the usage of each preset", () => {
    render(
      <PresetCards
        presets={[small, large]}
        usage={new Map([["small", ["team-a/orders", "team-b/billing"]]])}
      />,
    );
    const card = (name: string) =>
      screen
        .getByRole("radio", { name: new RegExp(`^${name}`) })
        .closest("label")?.textContent;
    expect(card("small")).toContain("Brokers3");
    expect(card("small")).toContain("Partitions6");
    expect(card("small")).toContain("Used by 2 clusters");
    expect(card("large")).toContain("Not used yet");
  });

  it("says one cluster in the singular", () => {
    render(
      <PresetCards presets={[small]} usage={new Map([["small", ["a/b"]]])} />,
    );
    expect(screen.getByText("Used by 1 cluster")).toBeTruthy();
  });

  it("picks a preset on a click", async () => {
    const onChange = vi.fn();
    render(
      <PresetCards
        presets={[small, large]}
        usage={new Map()}
        onChange={onChange}
      />,
    );
    await userEvent.click(screen.getByRole("radio", { name: /^large/ }));
    expect(onChange).toHaveBeenCalledWith("large");
  });

  it("shows the description between the name and the facts", () => {
    render(<PresetCards presets={[described]} usage={new Map()} />);
    const card = screen
      .getByRole("radio", { name: /^medium/ })
      .closest("label");
    expect(card?.textContent).toMatch(
      /^mediumFor production workloads\.Brokers3/,
    );
  });

  it("shows no description line for a preset without one", () => {
    const blank = preset("blank", undefined, {
      "kubernetes.io/description": " ",
    });
    render(<PresetCards presets={[large, blank]} usage={new Map()} />);
    // Without facts, the text column holds the name and the usage line only.
    for (const name of ["large", "blank"]) {
      const column = screen.getByRole("radio", {
        name: new RegExp(`^${name}`),
      }).nextElementSibling;
      expect(column?.children).toHaveLength(2);
    }
  });
});
