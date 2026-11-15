// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { presetFixtures } from "./fixtures";
import { PresetSettings } from "./preset-settings";

afterEach(cleanup);

const medium = presetFixtures().clusterPresets[0];

describe("PresetSettings", () => {
  it("shows each field that the preset sets with its merge rule, in its part", () => {
    render(
      <PresetSettings kind="CamundaClusterPreset" body={medium.spec.cluster} />,
    );
    const brokers = screen.getByRole("region", { name: "Brokers" });
    const row = within(brokers).getByText("zeebe.extraEnv").closest("div");
    expect(row?.textContent).toContain("Merged by name");
    expect(row?.textContent).toContain("JAVA_TOOL_OPTIONS");
    expect(within(brokers).getByText("zeebe.replicas")).toBeTruthy();
    expect(
      within(screen.getByRole("region", { name: "Authentication" })).getByText(
        "auth.basic",
      ),
    ).toBeTruthy();
  });

  it("shows a scalar value as text", () => {
    render(
      <PresetSettings kind="DatabaseServerPreset" body={{ instances: 2 }} />,
    );
    const row = screen.getByText("instances").closest("div");
    expect(row?.textContent).toBe("instances2A resource value replaces it");
  });

  it("lists the fields that a preset cannot set", () => {
    render(
      <PresetSettings kind="DatabaseServerPreset" body={{ instances: 2 }} />,
    );
    const list = screen.getByRole("list", {
      name: "Fields a preset cannot set",
    });
    expect(
      within(list)
        .getAllByRole("listitem")
        .map((i) => i.textContent),
    ).toEqual([
      "presetRef",
      "releaseRef",
      "databaseServerConfig",
      "suspend",
      "version",
    ]);
  });

  it("says that an empty preset sets nothing", () => {
    render(<PresetSettings kind="ElasticsearchClusterPreset" body={{}} />);
    expect(
      screen.getByText("The preset sets no field. Each resource sets its own."),
    ).toBeTruthy();
  });
});
