// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { EnvEditor } from "./env-editor";

afterEach(cleanup);

const secrets = [
  { name: "db", keys: ["password", "username"] },
  { name: "smtp", keys: ["token"] },
];

describe("EnvEditor", () => {
  it("returns a Secret key reference for a Secret entry and has no field for a Secret value", async () => {
    const onChange = vi.fn();
    render(
      <EnvEditor
        label="Environment variables"
        secrets={secrets}
        onChange={onChange}
      />,
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Add a variable" }),
    );
    const row = screen.getByRole("group", { name: "Variable 1" });
    await userEvent.type(
      within(row).getByRole("textbox", { name: "Name" }),
      "DB_PASSWORD",
    );
    await userEvent.selectOptions(
      within(row).getByRole("combobox", { name: "Source" }),
      "secret",
    );

    expect(within(row).queryByRole("textbox", { name: "Value" })).toBeNull();
    expect(within(row).getAllByRole("textbox")).toHaveLength(1);

    await userEvent.selectOptions(
      within(row).getByRole("combobox", { name: "Secret" }),
      "db",
    );
    await userEvent.selectOptions(
      within(row).getByRole("combobox", { name: "Key" }),
      "password",
    );
    expect(onChange).toHaveBeenLastCalledWith(
      [
        {
          name: "DB_PASSWORD",
          valueFrom: { secretKeyRef: { name: "db", key: "password" } },
        },
      ],
      [],
    );
  });

  it("drops the typed value when an entry switches to a Secret", async () => {
    const onChange = vi.fn();
    render(
      <EnvEditor
        label="Env"
        secrets={secrets}
        defaultValue={[{ name: "TOKEN", value: "abc" }]}
        onChange={onChange}
      />,
    );
    const row = screen.getByRole("group", { name: "Variable 1" });
    await userEvent.selectOptions(
      within(row).getByRole("combobox", { name: "Source" }),
      "secret",
    );
    const [env] = onChange.mock.lastCall!;
    expect(env).toEqual([{ name: "TOKEN" }]);
    expect(JSON.stringify(env)).not.toContain("abc");
  });

  it("keeps an entry from another source unchanged and read only", async () => {
    const onChange = vi.fn();
    const fromConfigMap = {
      name: "MODE",
      valueFrom: { configMapKeyRef: { name: "settings", key: "mode" } },
    };
    render(
      <EnvEditor
        label="Env"
        secrets={secrets}
        defaultValue={[fromConfigMap, { name: "A", value: "1" }]}
        onChange={onChange}
      />,
    );
    const second = screen.getByRole("group", { name: "Variable 2" });
    await userEvent.type(
      within(second).getByRole("textbox", { name: "Value" }),
      "2",
    );
    expect(onChange).toHaveBeenLastCalledWith(
      [fromConfigMap, { name: "A", value: "12" }],
      [],
    );
    const first = screen.getByRole("group", { name: "Variable 1" });
    expect(within(first).queryAllByRole("textbox")).toHaveLength(0);
    expect(first.textContent).toContain("From a ConfigMap key");
  });

  it.each([
    ["the Secret", [{ name: "smtp", keys: ["token"] }]],
    ["the key", [{ name: "db", keys: ["username"] }]],
  ])("drops a Secret reference when %s goes away", (_, after) => {
    const onChange = vi.fn();
    const entry = {
      name: "DB_PASSWORD",
      valueFrom: { secretKeyRef: { name: "db", key: "password" } },
    };
    const { rerender } = render(
      <EnvEditor
        label="Env"
        secrets={secrets}
        defaultValue={[entry]}
        onChange={onChange}
      />,
    );
    rerender(
      <EnvEditor
        label="Env"
        secrets={after}
        defaultValue={[entry]}
        onChange={onChange}
      />,
    );
    expect(onChange).toHaveBeenLastCalledWith(
      [{ name: "DB_PASSWORD" }],
      [{ path: "0.secret", message: "Pick a Secret and a key" }],
    );
  });

  it("drops the references of two rows that lose the same Secret", () => {
    const onChange = vi.fn();
    const ref = (name: string) => ({
      name,
      valueFrom: { secretKeyRef: { name: "db", key: "password" } },
    });
    const entries = [ref("A"), ref("B")];
    const { rerender } = render(
      <EnvEditor
        label="Env"
        secrets={secrets}
        defaultValue={entries}
        onChange={onChange}
      />,
    );
    rerender(
      <EnvEditor
        label="Env"
        secrets={[{ name: "smtp", keys: ["token"] }]}
        defaultValue={entries}
        onChange={onChange}
      />,
    );
    expect(onChange).toHaveBeenLastCalledWith(
      [{ name: "A" }, { name: "B" }],
      [
        { path: "0.secret", message: "Pick a Secret and a key" },
        { path: "1.secret", message: "Pick a Secret and a key" },
      ],
    );
  });

  it("reports an empty name, a repeated name, and a Secret entry without a key", async () => {
    const onChange = vi.fn();
    render(
      <EnvEditor
        label="Env"
        secrets={secrets}
        defaultValue={[
          { name: "A", value: "1" },
          { name: "A", value: "2" },
        ]}
        onChange={onChange}
      />,
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Add a variable" }),
    );
    const third = screen.getByRole("group", { name: "Variable 3" });
    await userEvent.selectOptions(
      within(third).getByRole("combobox", { name: "Source" }),
      "secret",
    );
    expect(onChange.mock.lastCall![1]).toEqual([
      { path: "1.name", message: "A is set twice" },
      { path: "2.name", message: "Enter a name" },
      { path: "2.secret", message: "Pick a Secret and a key" },
    ]);
  });
});
