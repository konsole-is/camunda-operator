// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SchedulingEditor } from "./scheduling-editor";

afterEach(cleanup);

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

  it("keeps the operators Lt, Gt, and one that it does not know", async () => {
    const onChange = vi.fn();
    const kept = [
      { key: "gpu-count", operator: "Lt", value: "4" },
      { key: "zone", operator: "Gt", value: "2" },
      { key: "new", operator: "Matches", value: "x" },
    ];
    render(
      <SchedulingEditor
        label="Tolerations"
        defaultValue={[...kept, { key: "a", operator: "Equal", value: "1" }]}
        onChange={onChange}
      />,
    );
    const last = screen.getByRole("group", { name: "Toleration 4" });
    await userEvent.type(
      within(last).getByRole("textbox", { name: "Value" }),
      "2",
    );
    expect(onChange).toHaveBeenLastCalledWith(
      [...kept, { key: "a", operator: "Equal", value: "12" }],
      [],
    );
    const first = screen.getByRole("group", { name: "Toleration 1" });
    expect(
      (
        within(first).getByRole("combobox", {
          name: "Operator",
        }) as HTMLSelectElement
      ).value,
    ).toBe("Lt");
  });

  it("keeps a negative number of seconds, which evicts at once", async () => {
    const onChange = vi.fn();
    const evict = {
      key: "unreachable",
      operator: "Exists",
      effect: "NoExecute",
      tolerationSeconds: -1,
    };
    render(
      <SchedulingEditor
        label="Tolerations"
        defaultValue={[evict, { key: "a", operator: "Equal", value: "1" }]}
        onChange={onChange}
      />,
    );
    const second = screen.getByRole("group", { name: "Toleration 2" });
    await userEvent.type(
      within(second).getByRole("textbox", { name: "Value" }),
      "2",
    );
    expect(onChange).toHaveBeenLastCalledWith(
      [evict, { key: "a", operator: "Equal", value: "12" }],
      [],
    );
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
