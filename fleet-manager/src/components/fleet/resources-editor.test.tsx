// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  type AmountWrite,
  amountWrite,
  limitsBelowRequests,
  ResourcesEditor,
} from "./resources-editor";

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

  it("keeps quantities with a fraction or an exponent when another field changes", async () => {
    const onChange = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={{ requests: { cpu: ".5", memory: "1e9" } }}
        onChange={onChange}
      />,
    );
    const limit = screen.getByRole("group", { name: "CPU limit" });
    await userEvent.type(
      within(limit).getByRole("textbox", { name: "Amount" }),
      "1",
    );
    expect(onChange).toHaveBeenLastCalledWith(
      { requests: { cpu: ".5", memory: "1e9" }, limits: { cpu: "1" } },
      [],
    );
  });

  it("accepts a limit equal to its request in another unit", async () => {
    const onChange = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={{ requests: { cpu: "700m" } }}
        onChange={onChange}
      />,
    );
    const limit = screen.getByRole("group", { name: "CPU limit" });
    await userEvent.type(
      within(limit).getByRole("textbox", { name: "Amount" }),
      "0.7",
    );
    expect(onChange.mock.lastCall![1]).toEqual([]);
  });

  it("compares a limit with a request in a unit that the list does not hold", async () => {
    const onChange = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={{ requests: { cpu: "1000000u" } }}
        onChange={onChange}
      />,
    );
    const limit = screen.getByRole("group", { name: "CPU limit" });
    await userEvent.type(
      within(limit).getByRole("textbox", { name: "Amount" }),
      "500",
    );
    await userEvent.selectOptions(
      within(limit).getByRole("combobox", { name: "Unit" }),
      "m",
    );
    expect(onChange.mock.lastCall![1]).toEqual([
      { path: "limits.cpu", message: "The limit is below the request" },
    ]);
  });

  it("compares a limit with a request in exponent notation", async () => {
    const onChange = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={{ requests: { memory: "2e9" } }}
        onChange={onChange}
      />,
    );
    const limit = screen.getByRole("group", { name: "Memory limit" });
    await userEvent.type(
      within(limit).getByRole("textbox", { name: "Amount" }),
      "1",
    );
    expect(onChange.mock.lastCall![1]).toEqual([
      { path: "limits.memory", message: "The limit is below the request" },
    ]);
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

describe("ResourcesEditor quantities", () => {
  it("accepts an amount with a plus sign, which a quantity can have", () => {
    const onErrors = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={{ requests: { cpu: "+1" } }}
        onErrors={onErrors}
      />,
    );
    expect(onErrors).toHaveBeenLastCalledWith([]);
  });

  it("refuses a negative amount", () => {
    const onErrors = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={{ limits: { memory: "-1Gi" } }}
        onErrors={onErrors}
      />,
    );
    expect(onErrors).toHaveBeenLastCalledWith([
      { path: "limits.memory", message: "Enter 0 or more" },
    ]);
  });

  it("leaves a limit below its request to a caller that checks limits", async () => {
    const onErrors = vi.fn();
    const onChange = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={{ requests: { cpu: "2" }, limits: { cpu: "1" } }}
        limitsChecked
        onChange={onChange}
        onErrors={onErrors}
      />,
    );
    expect(screen.queryByText("The limit is below the request")).toBeNull();
    expect(onErrors).toHaveBeenLastCalledWith([]);
    const memory = within(
      screen.getByRole("group", { name: "Memory request" }),
    ).getByRole("textbox", { name: "Amount" });
    await userEvent.type(memory, "2");
    expect(onChange).toHaveBeenLastCalledWith(
      { requests: { cpu: "2", memory: "2Gi" }, limits: { cpu: "1" } },
      [],
    );
  });

  it("still refuses an amount that is not a number for a caller that checks limits", async () => {
    const onChange = vi.fn();
    render(
      <ResourcesEditor label="Resources" limitsChecked onChange={onChange} />,
    );
    await userEvent.type(
      within(screen.getByRole("group", { name: "CPU limit" })).getByRole(
        "textbox",
        { name: "Amount" },
      ),
      "x",
    );
    expect(onChange).toHaveBeenLastCalledWith({ limits: { cpu: "x" } }, [
      { path: "limits.cpu", message: "Enter a number" },
    ]);
  });
});

describe("ResourcesEditor with onAmounts", () => {
  const amount = (label: string) =>
    within(screen.getByRole("group", { name: label })).getByRole("textbox", {
      name: "Amount",
    });
  const PRESET = {
    requests: { cpu: "1", memory: "1Gi" },
    limits: { cpu: "2", memory: "2Gi" },
  };

  it("reports only the amount that the user changed", async () => {
    const onAmounts = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={PRESET}
        limitsChecked
        onAmounts={onAmounts}
      />,
    );
    await userEvent.clear(amount("Memory request"));
    expect(onAmounts).toHaveBeenLastCalledWith({ "requests.memory": "" }, []);
    await userEvent.type(amount("Memory request"), "2");
    expect(onAmounts).toHaveBeenLastCalledWith(
      { "requests.memory": "2Gi" },
      [],
    );
  });

  it("reports a unit that the user picked as a change of its amount", async () => {
    const onAmounts = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={PRESET}
        limitsChecked
        onAmounts={onAmounts}
      />,
    );
    await userEvent.selectOptions(
      within(screen.getByRole("group", { name: "Memory limit" })).getByRole(
        "combobox",
        { name: "Unit" },
      ),
      "Mi",
    );
    expect(onAmounts).toHaveBeenLastCalledWith({ "limits.memory": "2Mi" }, []);
  });

  it("reports each amount typed while an amount had an error with the first edit without one", async () => {
    const onAmounts = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={PRESET}
        limitsChecked
        onAmounts={onAmounts}
      />,
    );
    await userEvent.type(amount("CPU limit"), "x");
    await userEvent.clear(amount("Memory request"));
    await userEvent.type(amount("Memory request"), "3");
    expect(onAmounts).toHaveBeenLastCalledWith(
      { "limits.cpu": "2x", "requests.memory": "3Gi" },
      [{ path: "limits.cpu", message: "Enter a number" }],
    );
    await userEvent.type(amount("CPU limit"), "{Backspace}");
    expect(onAmounts).toHaveBeenLastCalledWith(
      { "limits.cpu": "2", "requests.memory": "3Gi" },
      [],
    );
    await userEvent.type(amount("CPU limit"), "0");
    expect(onAmounts).toHaveBeenLastCalledWith({ "limits.cpu": "20" }, []);
  });

  it("writes the whole value without onAmounts, as on the preset page", async () => {
    const onChange = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={PRESET}
        onChange={onChange}
      />,
    );
    await userEvent.clear(amount("Memory request"));
    await userEvent.type(amount("Memory request"), "2");
    expect(onChange).toHaveBeenLastCalledWith(
      {
        requests: { cpu: "1", memory: "2Gi" },
        limits: { cpu: "2", memory: "2Gi" },
      },
      [],
    );
  });

  it("writes both amounts after a refused limit below its request, as on the preset page", async () => {
    const onChange = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={PRESET}
        onChange={onChange}
      />,
    );
    await userEvent.clear(amount("CPU request"));
    await userEvent.type(amount("CPU request"), "3");
    expect(onChange).toHaveBeenLastCalledWith(
      {
        requests: { cpu: "3", memory: "1Gi" },
        limits: { cpu: "2", memory: "2Gi" },
      },
      [{ path: "limits.cpu", message: "The limit is below the request" }],
    );
    await userEvent.clear(amount("CPU limit"));
    await userEvent.type(amount("CPU limit"), "4");
    expect(onChange).toHaveBeenLastCalledWith(
      {
        requests: { cpu: "3", memory: "1Gi" },
        limits: { cpu: "4", memory: "2Gi" },
      },
      [],
    );
  });
});

describe("limitsBelowRequests", () => {
  it("names a limit below its request in another unit", () => {
    expect(
      limitsBelowRequests({
        requests: { cpu: "1" },
        limits: { cpu: "500m" },
      }),
    ).toEqual([{ resource: "cpu", request: "1", limit: "500m" }]);
  });

  it("names the memory after the CPU", () => {
    expect(
      limitsBelowRequests({
        requests: { cpu: 2, memory: "2Gi" },
        limits: { cpu: "1", memory: "1024Mi" },
      }),
    ).toEqual([
      { resource: "cpu", request: "2", limit: "1" },
      { resource: "memory", request: "2Gi", limit: "1024Mi" },
    ]);
  });

  it("accepts a limit equal to its request in another unit", () => {
    expect(
      limitsBelowRequests({
        requests: { cpu: "0.7", memory: "1Gi" },
        limits: { cpu: "700m", memory: "1024Mi" },
      }),
    ).toEqual([]);
  });

  it("compares nothing without a request or a limit", () => {
    expect(limitsBelowRequests({ limits: { cpu: "1" } })).toEqual([]);
    expect(limitsBelowRequests({ requests: { cpu: "2" } })).toEqual([]);
    expect(limitsBelowRequests({})).toEqual([]);
  });

  it("compares every resource that the requests and the limits both hold", () => {
    expect(
      limitsBelowRequests({
        requests: { "ephemeral-storage": "2Gi", "nvidia.com/gpu": "1" },
        limits: { "ephemeral-storage": "1Gi" },
      }),
    ).toEqual([
      { resource: "ephemeral-storage", request: "2Gi", limit: "1Gi" },
    ]);
  });

  it("reads a quantity with a sign or an exponent", () => {
    expect(
      limitsBelowRequests({
        requests: { cpu: "+2", memory: "2e9" },
        limits: { cpu: "1", memory: "1E+9" },
      }),
    ).toEqual([
      { resource: "cpu", request: "+2", limit: "1" },
      { resource: "memory", request: "2e9", limit: "1E+9" },
    ]);
  });

  it("compares nothing with a quantity that is not a number", () => {
    expect(
      limitsBelowRequests({ requests: { cpu: "two" }, limits: { cpu: "1" } }),
    ).toEqual([]);
  });
});

describe("amountWrite", () => {
  const rows: {
    name: string;
    quantity: string;
    saved?: string | number;
    below?: string | number;
    held?: true;
    want: AmountWrite;
  }[] = [
    {
      name: "resets an emptied field",
      quantity: "",
      saved: "3",
      below: "1",
      want: undefined,
    },
    {
      name: "resets an emptied field under a held change",
      quantity: "",
      saved: "3",
      held: true,
      want: undefined,
    },
    {
      name: "writes the saved value back as it is",
      quantity: "1",
      saved: 1,
      below: "2",
      want: { value: 1 },
    },
    {
      name: "writes the saved value back when it is equal to the preset",
      quantity: "1000m",
      saved: "1",
      below: "1",
      want: { value: "1" },
    },
    {
      name: "writes the preset amount when the saved claim sets another one",
      quantity: "1Gi",
      saved: "3Gi",
      below: "1Gi",
      want: { value: "1Gi" },
    },
    {
      name: "resets the preset amount where the saved claim sets none",
      quantity: "1024Mi",
      below: "1Gi",
      want: undefined,
    },
    {
      name: "writes an amount that is neither saved nor below",
      quantity: "2Gi",
      saved: "3Gi",
      below: "1Gi",
      want: { value: "2Gi" },
    },
    {
      name: "writes an amount where no layer gives a value",
      quantity: "5",
      want: { value: "5" },
    },
    {
      name: "resets the preset amount under a held change",
      quantity: "500m",
      saved: "2",
      below: "500m",
      held: true,
      want: undefined,
    },
    {
      name: "resets the preset amount under a held change also when it is saved",
      quantity: "500m",
      saved: "500m",
      below: "500m",
      held: true,
      want: undefined,
    },
    {
      name: "resets the preset amount under a held change where the saved claim sets none",
      quantity: "512Mi",
      below: "512Mi",
      held: true,
      want: undefined,
    },
    {
      name: "writes the saved value as it is under a held change",
      quantity: "1500m",
      saved: "1.5",
      below: "500m",
      held: true,
      want: { value: "1.5" },
    },
    {
      name: "writes any other amount under a held change",
      quantity: "750m",
      saved: "2",
      below: "500m",
      held: true,
      want: { value: "750m" },
    },
    {
      name: "writes an amount that is not a number as it is",
      quantity: "2x",
      below: "2",
      want: { value: "2x" },
    },
  ];

  it.each(rows)("$name", ({ quantity, saved, below, held, want }) => {
    expect(
      amountWrite(quantity, { saved, below, held: held ?? false }),
    ).toEqual(want);
  });
});
