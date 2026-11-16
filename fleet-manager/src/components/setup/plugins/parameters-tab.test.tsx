// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { fleetEnvironment, pluginRegistration } from "./fixtures";
import { parameterRows } from "./parameters";
import { ParametersTab } from "./parameters-tab";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const kms = pluginRegistration("aws-kms", {
  parameters: {
    type: "object",
    properties: {
      deletionWindowDays: { type: "integer", minimum: 7, maximum: 30 },
      multiRegion: { type: "boolean" },
      keySpec: { type: "string", enum: ["SYMMETRIC_DEFAULT", "RSA_2048"] },
      alias: { type: "string", minimum: 1 },
    },
  },
  parameterValues: { deletionWindowDays: 30, multiRegion: false },
  claimEditable: ["multiRegion"],
});
const production = fleetEnvironment("production", {
  displayName: "Production",
  parameters: {
    "aws-kms": {
      deletionWindowDays: { value: 14 },
      keySpec: { claimEditable: true },
    },
  },
});

const termOf = (panel: string, label: string) =>
  within(screen.getByRole("region", { name: panel }))
    .queryAllByText(label, { selector: "dt" })
    .map((dt) => dt.nextElementSibling?.textContent);

describe("ParametersTab", () => {
  it("shows one panel per parameter with its type, range or values, and default", () => {
    render(<ParametersTab rows={parameterRows(kms, [], [])} unread={[]} />);
    expect(
      screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent),
    ).toEqual(["alias", "deletionWindowDays", "keySpec", "multiRegion"]);
    expect(termOf("deletionWindowDays", "Type")).toEqual(["integer"]);
    expect(termOf("deletionWindowDays", "Range")).toEqual(["7 to 30"]);
    expect(termOf("deletionWindowDays", "Default")).toEqual(["30"]);
    expect(termOf("keySpec", "Values")).toEqual([
      "SYMMETRIC_DEFAULT, RSA_2048",
    ]);
    expect(termOf("keySpec", "Default")).toEqual(["None"]);
    expect(termOf("multiRegion", "A claim can change it")).toEqual(["Yes"]);
    expect(termOf("deletionWindowDays", "A claim can change it")).toEqual([
      "No",
    ]);
  });

  it("shows the value of each environment and whether a claim can change it there", () => {
    render(
      <ParametersTab rows={parameterRows(kms, [production], [])} unread={[]} />,
    );
    expect(termOf("deletionWindowDays", "Production")).toEqual([
      "14A claim can change it: No",
    ]);
    expect(termOf("keySpec", "Production")).toEqual([
      "No valueA claim can change it: Yes",
    ]);
  });

  it("says where the default and the values of the environments hold", () => {
    render(<ParametersTab rows={parameterRows(kms, [], [])} unread={[]} />);
    expect(
      screen.getByText(
        /apply in a namespace without an environment\. Each environment shows what a new claim in it gets\. A claim that exists keeps the values that it resolved\./,
      ),
    ).toBeTruthy();
  });

  it("shows two environments of the same display name each with its value", () => {
    const twin = (name: string, value: number) =>
      fleetEnvironment(name, {
        displayName: "Production",
        parameters: { "aws-kms": { deletionWindowDays: { value } } },
      });
    const error = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <ParametersTab
        rows={parameterRows(kms, [twin("prod-a", 14), twin("prod-b", 21)], [])}
        unread={[]}
      />,
    );
    expect(termOf("deletionWindowDays", "Production")).toEqual([
      "14A claim can change it: No",
      "21A claim can change it: No",
    ]);
    expect(
      error.mock.calls.filter((call) => /same key/.test(String(call[0]))),
    ).toEqual([]);
  });

  it("marks a parameter that only an environment sets", () => {
    render(
      <ParametersTab
        rows={parameterRows(
          pluginRegistration("x", {}),
          [
            fleetEnvironment("production", {
              parameters: { x: { region: { value: "eu-west-1" } } },
            }),
          ],
          [],
        )}
        unread={[]}
      />,
    );
    expect(
      within(screen.getByRole("region", { name: "region" })).getByText(
        "Only an environment sets this parameter.",
      ),
    ).toBeTruthy();
  });

  it("says that the values of the environments are not known while Environment is not read", () => {
    render(
      <ParametersTab
        rows={parameterRows(kms, [production], ["Environment"])}
        unread={["Environment"]}
      />,
    );
    expect(
      screen.getByText(
        "Values of the environments: Not known: Environment is not read.",
      ),
    ).toBeTruthy();
    expect(termOf("deletionWindowDays", "Production")).toEqual([]);
  });

  it("shows an error and no rows for parameter values that are not an object", () => {
    render(
      <ParametersTab
        rows={parameterRows(
          pluginRegistration("bad", { parameterValues: [1] }),
          [],
          [],
        )}
        unread={[]}
      />,
    );
    expect(screen.getByRole("alert").textContent).toMatch(/parameterValues/);
    expect(screen.queryAllByRole("heading", { level: 2 })).toHaveLength(0);
  });

  it("shows the error of one parameter in its panel", () => {
    render(
      <ParametersTab
        rows={parameterRows(
          pluginRegistration("bad", {
            parameters: { properties: { size: "big" } },
          }),
          [],
          [],
        )}
        unread={[]}
      />,
    );
    expect(
      within(screen.getByRole("region", { name: "size" })).getByRole("alert")
        .textContent,
    ).toBe("The schema of this parameter is not an object.");
  });

  it("does not say that the option takes no parameters while Environment is not read", () => {
    render(
      <ParametersTab
        rows={parameterRows(pluginRegistration("x", {}), [], ["Environment"])}
        unread={["Environment"]}
      />,
    );
    expect(screen.queryByText("This option takes no parameters.")).toBeNull();
    expect(
      screen.getByText("Not known: Environment is not read."),
    ).toBeTruthy();
  });

  it("says when the option takes no parameters", () => {
    render(
      <ParametersTab
        rows={parameterRows(pluginRegistration("x", {}), [], [])}
        unread={[]}
      />,
    );
    expect(screen.getByText("This option takes no parameters.")).toBeTruthy();
  });

  it("says plugin and chart in no label", () => {
    render(
      <ParametersTab rows={parameterRows(kms, [production], [])} unread={[]} />,
    );
    const labels = [
      ...screen.getAllByRole("heading").map((h) => h.textContent),
      ...document.querySelectorAll("dt"),
    ].map((l) => (typeof l === "string" ? l : l?.textContent));
    expect(labels.filter((l) => /plugin|chart/i.test(l ?? ""))).toEqual([]);
  });
});
