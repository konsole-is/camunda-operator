// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { type ClusterField, parameterFields } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import type { ResolvedPolicy } from "@/lib/policy";
import type { OpenAPISchema } from "@/lib/schema";
import { SelectionSettingsEditor } from "./selection-settings-editor";

afterEach(cleanup);

const SHRINK = "Volumes cannot shrink. The operator keeps the current size.";

// The parameters of the elasticsearch registration, as
// internal/pluginstartup/schemas.go writes them.
const SCHEMA: OpenAPISchema = {
  type: "object",
  required: ["preset", "releaseRef"],
  properties: {
    preset: { type: "string", minLength: 1, title: "Preset" },
    releaseRef: { type: "string", minLength: 1, title: "Release" },
    overrides: {
      type: "object",
      title: "Overrides",
      properties: {
        replicas: { type: "integer", format: "int32", minimum: 1 },
        resources: {
          type: "object",
          "x-kubernetes-preserve-unknown-fields": true,
        },
        storageSize: { type: "string" },
        scheduling: {
          type: "object",
          "x-kubernetes-preserve-unknown-fields": true,
        },
        podLabels: {
          type: "object",
          additionalProperties: { type: "string" },
        },
      },
    },
  },
};

const overridesSteps = [
  { field: "spec" },
  { field: "plugins" },
  { item: { section: "secondary-storage", registration: "elasticsearch" } },
  { field: "parameters" },
  { field: "overrides" },
];

// fieldsOf returns the parameter fields of the elasticsearch selection of
// a managed claim that sets overrides.replicas to 3. presetEditable makes
// the preset claim-editable.
function fieldsOf(presetEditable = false) {
  return parameterFields({
    cluster: {
      kind: "CamundaCluster",
      metadata: { name: "orders", labels: { [CLAIM_LABEL]: "orders" } },
      spec: {},
    } as unknown as Resource<"CamundaCluster">,
    merged: mergeCluster(undefined, undefined, {}),
    claim: {
      kind: "CamundaClusterClaim",
      metadata: { name: "orders" },
      spec: {
        presetRef: "medium",
        plugins: [
          {
            section: "secondary-storage",
            registration: "elasticsearch",
            parameters: { overrides: { replicas: 3, storageSize: "50Gi" } },
          },
        ],
      },
    } as unknown as Resource<"CamundaClusterClaim">,
    policy: {
      parameters: {
        elasticsearch: {
          preset: { claimEditable: presetEditable, value: "small" },
          overrides: {
            claimEditable: true,
            value: { replicas: 1, storageSize: "50Gi" },
          },
        },
      },
    } as unknown as ResolvedPolicy,
    sections: [],
    registrations: [
      {
        kind: "PluginRegistration",
        metadata: { name: "elasticsearch" },
        spec: { parameters: SCHEMA },
      } as unknown as Resource<"PluginRegistration">,
    ],
  });
}

function renderEditor(
  props: Partial<Parameters<typeof SelectionSettingsEditor>[0]> = {},
) {
  const set = vi.fn();
  const reset = vi.fn();
  render(
    <SelectionSettingsEditor
      fields={fieldsOf()}
      schema={SCHEMA}
      set={set}
      reset={reset}
      {...props}
    />,
  );
  return { set, reset };
}

function settingRow(label: string): HTMLElement {
  return screen.getAllByRole("group", { name: label })[0];
}

describe("SelectionSettingsEditor", () => {
  it("edits the claim-editable parameters only", () => {
    renderEditor();
    const preset = settingRow("preset");
    expect(within(preset).getByText("Set by your admin")).toBeDefined();
    expect(within(preset).queryByRole("textbox")).toBeNull();
    expect(within(preset).queryByRole("combobox")).toBeNull();
    expect(
      within(settingRow("overrides.replicas")).getByRole("spinbutton"),
    ).toBeDefined();
    expect(
      within(settingRow("overrides.resources")).getByRole("group", {
        name: "CPU request",
      }),
    ).toBeDefined();
    expect(
      within(settingRow("overrides.storageSize")).getByRole("textbox", {
        name: "Amount",
      }),
    ).toBeDefined();
    expect(
      within(settingRow("overrides.scheduling")).getByRole("textbox"),
    ).toBeDefined();
    expect(
      within(settingRow("overrides.podLabels")).getByRole("button", {
        name: "Add an entry",
      }),
    ).toBeDefined();
    expect(screen.queryAllByRole("group", { name: "overrides" })).toEqual([]);
  });

  it("a change of one override writes the whole parameter", async () => {
    const { set } = renderEditor();
    const replicas = within(settingRow("overrides.replicas")).getByRole(
      "spinbutton",
    );
    await userEvent.clear(replicas);
    await userEvent.type(replicas, "5");
    expect(set).toHaveBeenLastCalledWith(overridesSteps, {
      replicas: 5,
      storageSize: "50Gi",
    });
  });

  it("each part of a whole parameter reports its own error", async () => {
    const invalid = vi.fn();
    renderEditor({ invalid });
    const replicas = within(settingRow("overrides.replicas")).getByRole(
      "spinbutton",
    );
    await userEvent.clear(replicas);
    await userEvent.type(replicas, "0");
    await userEvent.type(
      within(settingRow("overrides.storageSize")).getByRole("textbox", {
        name: "Amount",
      }),
      "0",
    );
    const replicasSteps = [...overridesSteps, { field: "replicas" }];
    const reports = invalid.mock.calls.filter(
      ([steps]) => JSON.stringify(steps) === JSON.stringify(replicasSteps),
    );
    expect(reports.at(-1)).toEqual([
      replicasSteps,
      "overrides.replicas: Enter 1 or more",
    ]);
  });

  it("a parameter resets to the environment", async () => {
    const { reset } = renderEditor();
    const replicas = settingRow("overrides.replicas");
    expect(
      within(replicas).getByText(
        "Every overrides field of the claim goes back to the environment.",
      ),
    ).toBeDefined();
    await userEvent.click(
      within(replicas).getByRole("button", {
        name: "Reset overrides to environment",
      }),
    );
    expect(reset).toHaveBeenCalledWith(overridesSteps);
  });

  it("a smaller storage size of an override says volumes cannot shrink", async () => {
    renderEditor();
    const amount = within(settingRow("overrides.storageSize")).getByRole(
      "textbox",
      { name: "Amount" },
    );
    await userEvent.clear(amount);
    await userEvent.type(amount, "20");
    expect(screen.getByText(SHRINK)).toBeDefined();
  });

  it("choices make a select also for a parameter whose schema is an object", async () => {
    const { set } = renderEditor({
      choices: { overrides: ["small", "large"] },
    });
    const select = within(settingRow("overrides")).getByRole("combobox");
    expect(
      screen.queryAllByRole("group", { name: "overrides.replicas" }),
    ).toEqual([]);
    await userEvent.selectOptions(select, "large");
    expect(set).toHaveBeenLastCalledWith(overridesSteps, "large");
  });

  it("choices make a select", async () => {
    const { set } = renderEditor({
      fields: fieldsOf(true),
      choices: { preset: ["small", "large"] },
    });
    const select = within(settingRow("preset")).getByRole("combobox");
    expect(
      within(select)
        .getAllByRole("option")
        .map((o) => o.textContent),
    ).toEqual(["Not set", "small", "large"]);
    await userEvent.selectOptions(select, "large");
    expect(set).toHaveBeenLastCalledWith(
      [...overridesSteps.slice(0, 4), { field: "preset" }],
      "large",
    );
  });

  describe("allowed", () => {
    const CHOICES: OpenAPISchema = {
      type: "object",
      properties: {
        tier: { type: "object" },
        policies: { type: "array", items: { type: "string" } },
      },
    };
    const stepsOf = (name: string) => [
      ...overridesSteps.slice(0, 4),
      { field: name },
    ];
    // choiceField returns an editable parameter that the environment sets
    // to value.
    const choiceField = (label: string, value: unknown) =>
      ({
        key: label,
        label,
        value,
        state: { kind: "environment" },
        claimSteps: stepsOf(label),
      }) as unknown as ClusterField;
    const wholeLists = {
      policies: [
        { name: "standard", value: ["a", "b"] },
        { name: "wide", value: ["a", "b", "c"] },
      ],
    };

    it("picks a choice by name and shows the name of the value", async () => {
      const { set } = renderEditor({
        fields: [choiceField("tier", { size: 2 })],
        schema: CHOICES,
        allowed: {
          tier: [
            { name: "silver", value: { size: 1 } },
            { name: "gold", value: { size: 2 } },
          ],
        },
      });
      const select = within(settingRow("tier")).getByRole<HTMLSelectElement>(
        "combobox",
      );
      expect(select.value).toBe("gold");
      await userEvent.selectOptions(select, "silver");
      expect(set).toHaveBeenLastCalledWith(stepsOf("tier"), "silver");
    });

    it("shows the name that claimed holds when two choices have one value", () => {
      renderEditor({
        fields: [choiceField("tier", { size: 2 })],
        schema: CHOICES,
        allowed: {
          tier: [
            { name: "silver", value: { size: 2 } },
            { name: "gold", value: { size: 2 } },
          ],
        },
        claimed: { tier: "gold" },
      });
      expect(
        within(settingRow("tier")).getByRole<HTMLSelectElement>("combobox")
          .value,
      ).toBe("gold");
    });

    it("a list parameter picks its choices with a checkbox for each name", async () => {
      const { set } = renderEditor({
        fields: [choiceField("policies", ["arn:sqs"])],
        schema: CHOICES,
        allowed: {
          policies: [
            { name: "sqs", value: "arn:sqs" },
            { name: "sns", value: "arn:sns" },
          ],
        },
      });
      const box = (name: string) =>
        within(settingRow("policies")).getByRole("checkbox", { name });
      expect(box("sqs").getAttribute("aria-checked")).toBe("true");
      expect(box("sns").getAttribute("aria-checked")).toBe("false");
      await userEvent.click(box("sns"));
      expect(set).toHaveBeenLastCalledWith(stepsOf("policies"), ["sqs", "sns"]);
    });

    it("a list parameter whose choices are whole lists picks one name", async () => {
      const { set } = renderEditor({
        fields: [choiceField("policies", ["a", "b"])],
        schema: CHOICES,
        allowed: wholeLists,
        claimed: { policies: "standard" },
      });
      const select = within(
        settingRow("policies"),
      ).getByRole<HTMLSelectElement>("combobox");
      expect(select.value).toBe("standard");
      await userEvent.selectOptions(select, "wide");
      expect(set).toHaveBeenLastCalledWith(stepsOf("policies"), "wide");
    });

    it("a list parameter whose choices are whole lists picks one name also from an empty list", async () => {
      const { set } = renderEditor({
        fields: [choiceField("policies", [])],
        schema: CHOICES,
        allowed: wholeLists,
        claimed: { policies: [] },
      });
      await userEvent.selectOptions(
        within(settingRow("policies")).getByRole("combobox"),
        "standard",
      );
      expect(set).toHaveBeenLastCalledWith(stepsOf("policies"), "standard");
    });
  });
});
