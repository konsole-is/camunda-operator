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
): CamundaClusterPreset {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaClusterPreset",
    metadata: { name },
    spec: { cluster: { ...(zeebe && { zeebe }) } },
  };
}

const small = preset("small", {
  replicas: 3,
  partitions: 6,
  resources: { requests: { cpu: "500m", memory: "2Gi" } },
});
const large = preset("large");

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
});
