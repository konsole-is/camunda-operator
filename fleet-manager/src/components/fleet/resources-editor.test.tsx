// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { limitsBelowRequests, ResourcesEditor } from "./resources-editor";

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
