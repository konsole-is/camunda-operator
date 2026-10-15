// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ResourcesEditor } from "./resources-editor";
import { SchedulingEditor } from "./scheduling-editor";
import { YamlEditor } from "./yaml-editor";

afterEach(cleanup);

describe("ResourcesEditor", () => {
  it("writes CPU and memory with their units", async () => {
    const onChange = vi.fn();
    render(<ResourcesEditor label="Resources" onChange={onChange} />);
    const cpu = screen.getByRole("group", { name: "CPU request" });
    await userEvent.type(
      within(cpu).getByRole("textbox", { name: "Amount" }),
      "500",
    );
    await userEvent.selectOptions(
      within(cpu).getByRole("combobox", { name: "Unit" }),
      "m",
    );
    const memory = screen.getByRole("group", { name: "Memory limit" });
    await userEvent.type(
      within(memory).getByRole("textbox", { name: "Amount" }),
      "2",
    );
    expect(onChange).toHaveBeenLastCalledWith(
      { requests: { cpu: "500m" }, limits: { memory: "2Gi" } },
      [],
    );
  });

  it("keeps the resources and units that it has no field for", async () => {
    const onChange = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={{
          requests: { cpu: "1", memory: "512Ki", "ephemeral-storage": "1Gi" },
          limits: { "nvidia.com/gpu": 1 },
        }}
        onChange={onChange}
      />,
    );
    const cpu = screen.getByRole("group", { name: "CPU request" });
    await userEvent.clear(within(cpu).getByRole("textbox", { name: "Amount" }));
    await userEvent.type(
      within(cpu).getByRole("textbox", { name: "Amount" }),
      "2",
    );
    expect(onChange).toHaveBeenLastCalledWith(
      {
        requests: { cpu: "2", memory: "512Ki", "ephemeral-storage": "1Gi" },
        limits: { "nvidia.com/gpu": 1 },
      },
      [],
    );
  });

  it("refuses a limit below its request and an amount that is not a number", async () => {
    const onChange = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={{ requests: { memory: "2Gi" } }}
        onChange={onChange}
      />,
    );
    const limit = screen.getByRole("group", { name: "Memory limit" });
    await userEvent.type(
      within(limit).getByRole("textbox", { name: "Amount" }),
      "512",
    );
    await userEvent.selectOptions(
      within(limit).getByRole("combobox", { name: "Unit" }),
      "Mi",
    );
    expect(onChange.mock.lastCall![1]).toEqual([
      { path: "limits.memory", message: "The limit is below the request" },
    ]);
    const cpu = screen.getByRole("group", { name: "CPU request" });
    await userEvent.type(
      within(cpu).getByRole("textbox", { name: "Amount" }),
      "abc",
    );
    expect(onChange.mock.lastCall![1]).toContainEqual({
      path: "requests.cpu",
      message: "Enter a number",
    });
  });
});

describe("SchedulingEditor", () => {
  it("writes a toleration and drops the fields that its operator and effect do not use", async () => {
    const onChange = vi.fn();
    render(<SchedulingEditor label="Tolerations" onChange={onChange} />);
    await userEvent.click(
      screen.getByRole("button", { name: "Add a toleration" }),
    );
    const row = screen.getByRole("group", { name: "Toleration 1" });
    await userEvent.type(
      within(row).getByRole("textbox", { name: "Key" }),
      "dedicated",
    );
    await userEvent.type(
      within(row).getByRole("textbox", { name: "Value" }),
      "camunda",
    );
    await userEvent.selectOptions(
      within(row).getByRole("combobox", { name: "Effect" }),
      "NoExecute",
    );
    await userEvent.type(
      within(row).getByRole("spinbutton", { name: "Seconds" }),
      "300",
    );
    expect(onChange).toHaveBeenLastCalledWith(
      [
        {
          key: "dedicated",
          operator: "Equal",
          value: "camunda",
          effect: "NoExecute",
          tolerationSeconds: 300,
        },
      ],
      [],
    );
    await userEvent.selectOptions(
      within(row).getByRole("combobox", { name: "Operator" }),
      "Exists",
    );
    expect(within(row).queryByRole("textbox", { name: "Value" })).toBeNull();
    expect(onChange.mock.lastCall![0]).toEqual([
      {
        key: "dedicated",
        operator: "Exists",
        effect: "NoExecute",
        tolerationSeconds: 300,
      },
    ]);
  });

  it("refuses Equal without a key", async () => {
    const onChange = vi.fn();
    render(<SchedulingEditor label="Tolerations" onChange={onChange} />);
    await userEvent.click(
      screen.getByRole("button", { name: "Add a toleration" }),
    );
    expect(onChange.mock.lastCall![1]).toEqual([
      { path: "0.key", message: "Enter a key, or use the operator Exists" },
    ]);
  });
});

describe("YamlEditor", () => {
  it("returns the parsed value", async () => {
    const onChange = vi.fn();
    render(<YamlEditor label="Node affinity" onChange={onChange} />);
    await userEvent.type(
      screen.getByRole("textbox", { name: "Node affinity" }),
      "a: 1",
    );
    expect(onChange).toHaveBeenLastCalledWith({ a: 1 }, []);
  });

  it("names the line of a YAML error", async () => {
    const onChange = vi.fn();
    render(<YamlEditor label="Node affinity" onChange={onChange} />);
    // user-event reads "[" as the start of a key name, so "[[" types one "[".
    await userEvent.type(
      screen.getByRole("textbox", { name: "Node affinity" }),
      "a: 1{enter}b: [[",
    );
    const [value, errors] = onChange.mock.lastCall!;
    expect(value).toBeUndefined();
    expect(errors[0].message).toMatch(/^Line 2: /);
    expect(
      screen
        .getByRole("textbox", { name: "Node affinity" })
        .getAttribute("aria-invalid"),
    ).toBe("true");
  });

  it("shows the schema errors with their paths", async () => {
    const onChange = vi.fn();
    const schema = {
      type: "object",
      properties: { replicas: { type: "integer" } },
    };
    render(
      <YamlEditor label="Overrides" schema={schema} onChange={onChange} />,
    );
    await userEvent.type(
      screen.getByRole("textbox", { name: "Overrides" }),
      "replicas: two",
    );
    expect(onChange.mock.lastCall![1]).toEqual([
      { path: "replicas", message: "must be integer" },
    ]);
    expect(screen.getByText("replicas: must be integer")).toBeTruthy();
  });

  it("treats empty text as no value", async () => {
    const onChange = vi.fn();
    render(
      <YamlEditor
        label="Node affinity"
        defaultValue={{ a: 1 }}
        onChange={onChange}
      />,
    );
    await userEvent.clear(
      screen.getByRole("textbox", { name: "Node affinity" }),
    );
    expect(onChange).toHaveBeenLastCalledWith(undefined, []);
  });
});
