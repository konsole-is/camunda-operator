// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  type ClaimLayers,
  claimResourcesOf,
  limitsBelowRequests,
  type Resources,
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

describe("ResourcesEditor over a claim", () => {
  const amount = (label: string) =>
    within(screen.getByRole("group", { name: label })).getByRole("textbox", {
      name: "Amount",
    });

  it("writes only the amounts that the user typed in", async () => {
    const onChange = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={PRESET}
        limitsChecked
        layers={{
          claim: undefined,
          before: { claim: undefined, merged: PRESET },
        }}
        onChange={onChange}
      />,
    );
    await userEvent.clear(amount("Memory request"));
    await userEvent.type(amount("Memory request"), "2");
    expect(onChange).toHaveBeenLastCalledWith(
      { requests: { memory: "2Gi" } },
      [],
    );
  });

  it("writes a unit that the user picked as an edit of its field", async () => {
    const onChange = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={PRESET}
        limitsChecked
        layers={{
          claim: undefined,
          before: { claim: undefined, merged: PRESET },
        }}
        onChange={onChange}
      />,
    );
    await userEvent.selectOptions(
      within(screen.getByRole("group", { name: "Memory limit" })).getByRole(
        "combobox",
        { name: "Unit" },
      ),
      "Mi",
    );
    expect(onChange).toHaveBeenLastCalledWith(
      { limits: { memory: "2Mi" } },
      [],
    );
  });

  it("writes an amount that it refused before with the next valid edit", async () => {
    const onChange = vi.fn();
    render(
      <ResourcesEditor
        label="Resources"
        defaultValue={PRESET}
        limitsChecked
        layers={{
          claim: undefined,
          before: { claim: undefined, merged: PRESET },
        }}
        onChange={onChange}
      />,
    );
    await userEvent.type(amount("CPU limit"), "x");
    expect(onChange).toHaveBeenLastCalledWith({ limits: { cpu: "2x" } }, [
      { path: "limits.cpu", message: "Enter a number" },
    ]);
    await userEvent.clear(amount("CPU limit"));
    await userEvent.type(amount("CPU limit"), "4");
    expect(onChange).toHaveBeenLastCalledWith({ limits: { cpu: "4" } }, []);
  });

  it("keeps the newer claim amount of a field that the user did not type in", async () => {
    const onChange = vi.fn();
    const claim = { requests: { cpu: "1" }, limits: { cpu: "2" } };
    const { rerender } = render(
      <ResourcesEditor
        label="Resources"
        defaultValue={claim}
        limitsChecked
        layers={{ claim, before: { claim, merged: claim } }}
        onChange={onChange}
      />,
    );
    await userEvent.type(amount("CPU request"), "0");
    const newer = { requests: { cpu: "10" }, limits: { cpu: "40" } };
    rerender(
      <ResourcesEditor
        label="Resources"
        defaultValue={newer}
        limitsChecked
        layers={{ claim: newer, before: { claim: newer, merged: newer } }}
        onChange={onChange}
      />,
    );
    expect(amount("CPU limit")).toHaveProperty("value", "2");
    await userEvent.type(amount("CPU request"), "0");
    expect(onChange).toHaveBeenLastCalledWith(
      { requests: { cpu: "100" }, limits: { cpu: "40" } },
      [],
    );
  });

  it("compares with the claim and the value at its mount when nothing is pending before it", async () => {
    const onChange = vi.fn();
    const merged = { requests: { cpu: "1", memory: "3Gi" } };
    const claim = { requests: { memory: "3Gi" } };
    const { rerender } = render(
      <ResourcesEditor
        label="Resources"
        defaultValue={merged}
        layers={{ claim }}
        onChange={onChange}
      />,
    );
    await userEvent.type(amount("CPU request"), "0");
    expect(onChange).toHaveBeenLastCalledWith(
      { requests: { cpu: "10", memory: "3Gi" } },
      [],
    );
    rerender(
      <ResourcesEditor
        label="Resources"
        defaultValue={{ requests: { cpu: "10", memory: "3Gi" } }}
        layers={{ claim: { requests: { cpu: "10", memory: "3Gi" } } }}
        onChange={onChange}
      />,
    );
    await userEvent.type(amount("CPU request"), "{Backspace}");
    expect(onChange).toHaveBeenLastCalledWith(
      { requests: { memory: "3Gi" } },
      [],
    );
  });

  it("writes the whole value without a claim, as on the preset page", async () => {
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

// The preset of the rows below: it gives every amount that the claim leaves out.
const PRESET = {
  requests: { cpu: "1", memory: "1Gi" },
  limits: { cpu: "2", memory: "2Gi" },
};

describe("claimResourcesOf", () => {
  // Each row gives the claim before the pending changes (base), the merged
  // value of base (the preset over base when it is left out), the claim
  // with the pending changes at the mount of the editor (atMount, base when
  // it is left out), the fields that the user typed in, and the block that
  // the claim gets. baseAtMount, base at the mount, is base unless a row
  // gives it.
  const rows: {
    name: string;
    base: Resources | undefined;
    merged?: Resources;
    atMount?: Resources;
    baseAtMount?: Resources;
    edits: Record<string, string>;
    want: Resources;
  }[] = [
    {
      name: "writes only the changed memory request over a preset",
      base: undefined,
      edits: { "requests.memory": "2Gi" },
      want: { requests: { memory: "2Gi" } },
    },
    {
      name: "removes a claim key whose field the user emptied",
      base: { requests: { cpu: "3", memory: "3Gi" } },
      edits: { "requests.cpu": "" },
      want: { requests: { memory: "3Gi" } },
    },
    {
      name: "writes nothing for an emptied field that only the preset gives",
      base: { requests: { memory: "3Gi" } },
      edits: { "requests.cpu": "" },
      want: { requests: { memory: "3Gi" } },
    },
    {
      name: "leaves no change after a delete and a retype of the claim amount",
      base: { requests: { cpu: "1" } },
      edits: { "requests.cpu": "1" },
      want: { requests: { cpu: "1" } },
    },
    {
      name: "leaves no change after a delete and a retype of the preset amount",
      base: undefined,
      edits: { "requests.cpu": "1" },
      want: {},
    },
    {
      name: "compares amounts by value, not by spelling",
      base: { requests: { cpu: 2 } },
      atMount: { requests: { cpu: "4" } },
      edits: { "requests.cpu": "2000m" },
      want: { requests: { cpu: 2 } },
    },
    {
      name: "keeps a number of the claim when another amount changes",
      base: { requests: { cpu: 1 } },
      edits: { "limits.memory": "4Gi" },
      want: { requests: { cpu: 1 }, limits: { memory: "4Gi" } },
    },
    {
      name: "keeps a number of the preset out of the claim",
      base: undefined,
      merged: { requests: { cpu: 1, memory: "1Gi" } },
      edits: { "requests.memory": "2Gi" },
      want: { requests: { memory: "2Gi" } },
    },
    {
      name: "keeps a pending change of a field that the user did not type in",
      base: { requests: { cpu: "3" } },
      atMount: { requests: { cpu: "3", memory: "5Gi" } },
      edits: { "requests.cpu": "4" },
      want: { requests: { cpu: "4", memory: "5Gi" } },
    },
    {
      name: "takes back a pending change from before the mount",
      base: { requests: { cpu: "3" } },
      atMount: { requests: { cpu: "5" } },
      edits: { "requests.cpu": "3" },
      want: { requests: { cpu: "3" } },
    },
    {
      name: "keeps the other amounts on the preset after a reset of the block",
      base: { requests: { cpu: "3", memory: "3Gi" } },
      atMount: undefined,
      edits: { "requests.memory": "4Gi" },
      want: { requests: { memory: "4Gi" } },
    },
    {
      name: "writes the claim amount again after a reset of the block",
      base: { requests: { cpu: "3", memory: "3Gi" } },
      atMount: undefined,
      edits: { "requests.cpu": "3" },
      want: { requests: { cpu: "3" } },
    },
    {
      name: "takes the amount of another writer that the editor did not load",
      base: { requests: { cpu: "1" }, limits: { cpu: "40" } },
      atMount: { requests: { cpu: "1" }, limits: { cpu: "2" } },
      baseAtMount: { requests: { cpu: "1" }, limits: { cpu: "2" } },
      edits: { "requests.cpu": "100" },
      want: { requests: { cpu: "100" }, limits: { cpu: "40" } },
    },
    {
      name: "keeps a pending change over a change of another writer",
      base: { limits: { cpu: "40" } },
      atMount: { limits: { cpu: "5" } },
      baseAtMount: { limits: { cpu: "2" } },
      edits: { "requests.cpu": "6" },
      want: { requests: { cpu: "6" }, limits: { cpu: "5" } },
    },
    {
      name: "writes each amount that the user typed in, also one refused before",
      base: undefined,
      edits: { "requests.cpu": "3", "limits.cpu": "4" },
      want: { requests: { cpu: "3" }, limits: { cpu: "4" } },
    },
    {
      name: "keeps the keys that the editor has no field for",
      base: {
        requests: { "ephemeral-storage": "1Gi" },
        limits: { "nvidia.com/gpu": 1 },
        claims: [{ name: "gpu" }],
      },
      edits: { "requests.cpu": "2" },
      want: {
        requests: { "ephemeral-storage": "1Gi", cpu: "2" },
        limits: { "nvidia.com/gpu": 1 },
        claims: [{ name: "gpu" }],
      },
    },
    {
      name: "keeps a pending change of a key without a field",
      base: { requests: { "ephemeral-storage": "1Gi" } },
      atMount: {
        requests: { "ephemeral-storage": "2Gi" },
        claims: [{ name: "gpu" }],
      },
      edits: { "requests.cpu": "2" },
      want: {
        requests: { "ephemeral-storage": "2Gi", cpu: "2" },
        claims: [{ name: "gpu" }],
      },
    },
    {
      name: "leaves out a requests or limits that the edit empties",
      base: { requests: { cpu: "3" } },
      edits: { "requests.cpu": "" },
      want: {},
    },
  ];

  it.each(rows)("$name", ({ base, edits, want, ...rest }) => {
    const merged = rest.merged ?? {
      requests: { ...PRESET.requests, ...base?.requests },
      limits: { ...PRESET.limits, ...base?.limits },
    };
    const layers: ClaimLayers = {
      base,
      merged,
      atMount: {
        claim: "atMount" in rest ? rest.atMount : base,
        base: "baseAtMount" in rest ? rest.baseAtMount : base,
      },
    };
    expect(claimResourcesOf(layers, edits)).toEqual(want);
  });

  it("changes none of its layers", () => {
    const base = { requests: { cpu: "3" } };
    claimResourcesOf(
      { base, merged: base, atMount: { claim: base, base } },
      { "requests.cpu": "" },
    );
    expect(base).toEqual({ requests: { cpu: "3" } });
  });
});
