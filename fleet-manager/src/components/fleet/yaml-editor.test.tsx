// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { YamlEditor } from "./yaml-editor";

afterEach(cleanup);

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

  it("reports an alias that refers to its own value as an error", async () => {
    const onChange = vi.fn();
    render(<YamlEditor label="Overrides" onChange={onChange} />);
    // user-event reads "[" as the start of a key name, so "[[" types one "[".
    await userEvent.type(
      screen.getByRole("textbox", { name: "Overrides" }),
      "&a [[*a]",
    );
    const [value, errors] = onChange.mock.lastCall!;
    expect(value).toBeUndefined();
    expect(errors).toEqual([
      { path: "", message: "An alias refers to the value that holds it" },
    ]);
  });

  it.each([[".nan"], [".inf"], ["-.inf"]])(
    "reports a nested %s as an error",
    async (number) => {
      const onChange = vi.fn();
      render(
        <YamlEditor
          label="Overrides"
          defaultValue={{ a: { weight: null } }}
          onChange={onChange}
        />,
      );
      const box = screen.getByRole("textbox", { name: "Overrides" });
      await userEvent.clear(box);
      await userEvent.type(box, `a:{enter}  weight: ${number}`);
      expect(onChange).toHaveBeenLastCalledWith(undefined, [
        { path: "", message: "Use a finite number, not .nan or .inf" },
      ]);
    },
  );

  it("reports a cleared null value as no value", async () => {
    const onChange = vi.fn();
    render(
      <YamlEditor label="Overrides" defaultValue={null} onChange={onChange} />,
    );
    await userEvent.clear(screen.getByRole("textbox", { name: "Overrides" }));
    expect(onChange).toHaveBeenLastCalledWith(undefined, []);
  });

  it("reports the errors of a new schema to onChange", async () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <YamlEditor
        label="Overrides"
        schema={{ type: "object" }}
        onChange={onChange}
      />,
    );
    await userEvent.type(
      screen.getByRole("textbox", { name: "Overrides" }),
      "replicas: two",
    );
    expect(onChange).toHaveBeenLastCalledWith({ replicas: "two" }, []);
    const calls = onChange.mock.calls.length;
    rerender(
      <YamlEditor
        label="Overrides"
        schema={{
          type: "object",
          properties: { replicas: { type: "integer" } },
        }}
        onChange={onChange}
      />,
    );
    expect(onChange).toHaveBeenLastCalledWith({ replicas: "two" }, [
      { path: "replicas", message: "must be integer" },
    ]);
    expect(onChange.mock.calls.length).toBe(calls + 1);
  });

  it("checks the text again when the schema changes", () => {
    const { rerender } = render(
      <YamlEditor
        label="Overrides"
        defaultValue={{ replicas: "two" }}
        schema={{ type: "object" }}
      />,
    );
    expect(screen.queryByText(/must be integer/)).toBeNull();
    rerender(
      <YamlEditor
        label="Overrides"
        defaultValue={{ replicas: "two" }}
        schema={{
          type: "object",
          properties: { replicas: { type: "integer" } },
        }}
      />,
    );
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
