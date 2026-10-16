// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SecretRefPicker } from "./secret-ref-picker";

afterEach(cleanup);

const secrets = [
  { name: "db", keys: ["password", "username"] },
  { name: "smtp", keys: ["token"] },
];

const pick = async (secret: string, key?: string) => {
  await userEvent.selectOptions(
    screen.getByRole("combobox", { name: "Secret" }),
    secret,
  );
  if (key)
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Key" }),
      key,
    );
};

describe("SecretRefPicker", () => {
  it("gives the reference once a Secret and a key are picked", async () => {
    const onChange = vi.fn();
    render(
      <SecretRefPicker
        label="Password"
        secrets={secrets}
        onChange={onChange}
      />,
    );

    await pick("db");
    expect(onChange).not.toHaveBeenCalled();

    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Key" }),
      "password",
    );
    expect(onChange).toHaveBeenLastCalledWith({ name: "db", key: "password" });
  });

  it("gives undefined when another Secret is picked after a reference", async () => {
    const onChange = vi.fn();
    render(
      <SecretRefPicker
        label="Password"
        secrets={secrets}
        onChange={onChange}
      />,
    );
    await pick("db", "password");

    await pick("smtp");

    expect(onChange).toHaveBeenCalledTimes(2);
    expect(onChange).toHaveBeenLastCalledWith(undefined);
  });

  it("gives undefined when the secrets no longer hold the picked Secret", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <SecretRefPicker
        label="Password"
        secrets={secrets}
        defaultValue={{ name: "db", key: "password" }}
        onChange={onChange}
      />,
    );
    expect(onChange).not.toHaveBeenCalled();

    rerender(
      <SecretRefPicker
        label="Password"
        secrets={[{ name: "smtp", keys: ["token"] }]}
        defaultValue={{ name: "db", key: "password" }}
        onChange={onChange}
      />,
    );

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith(undefined);
  });

  it("gives undefined when the picked Secret no longer holds the picked key", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <SecretRefPicker
        label="Password"
        secrets={secrets}
        defaultValue={{ name: "db", key: "password" }}
        onChange={onChange}
      />,
    );

    rerender(
      <SecretRefPicker
        label="Password"
        secrets={[{ name: "db", keys: ["username"] }]}
        defaultValue={{ name: "db", key: "password" }}
        onChange={onChange}
      />,
    );

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith(undefined);
  });

  it("does not report undefined again when it had no reference", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <SecretRefPicker
        label="Password"
        secrets={secrets}
        onChange={onChange}
      />,
    );

    rerender(
      <SecretRefPicker
        label="Password"
        secrets={[{ name: "smtp", keys: ["token"] }]}
        onChange={onChange}
      />,
    );

    expect(onChange).not.toHaveBeenCalled();
  });

  it("says so when the namespace has no Secret", () => {
    render(<SecretRefPicker label="Password" secrets={[]} />);
    expect(screen.getByText("No Secrets in this namespace")).toBeTruthy();
    expect(screen.queryByRole("combobox")).toBeNull();
  });
});
