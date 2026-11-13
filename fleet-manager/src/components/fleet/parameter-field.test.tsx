// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ParameterRule } from "@/lib/policy/types";
import { ParameterField } from "./parameter-field";

afterEach(cleanup);

const editable = (
  schema: ParameterRule["schema"],
  extra: Partial<ParameterRule> = {},
): ParameterRule =>
  ({
    name: "p",
    label: "Replicas",
    claimEditable: true,
    schema,
    ...extra,
  }) as ParameterRule;

describe("ParameterField", () => {
  it("shows a number field with its range", () => {
    render(
      <ParameterField
        id="p"
        rule={editable({ type: "integer", minimum: 1, maximum: 9 })}
        value={3}
        onValue={() => {}}
      />,
    );
    expect(screen.getByLabelText("Replicas")).toHaveProperty("type", "number");
    expect(screen.getByText("From 1 to 9")).toBeTruthy();
  });

  it("gives a typed number and undefined for a cleared field", async () => {
    const onValue = vi.fn();
    render(
      <ParameterField
        id="p"
        rule={editable({ type: "integer" })}
        value={undefined}
        onValue={onValue}
      />,
    );
    await userEvent.type(screen.getByLabelText("Replicas"), "4");
    expect(onValue).toHaveBeenLastCalledWith(4);
  });

  it("shows a select for a string enum", async () => {
    const onValue = vi.fn();
    render(
      <ParameterField
        id="p"
        rule={editable({ type: "string", enum: ["gp3", "io2"] })}
        value="gp3"
        onValue={onValue}
      />,
    );
    const select = screen.getByLabelText("Replicas");
    expect(select.tagName).toBe("SELECT");
    await userEvent.selectOptions(select, "io2");
    expect(onValue).toHaveBeenLastCalledWith("io2");
  });

  it("shows a select of the allowed names by their labels", async () => {
    const onValue = vi.fn();
    render(
      <ParameterField
        id="p"
        rule={editable(
          { type: "string" },
          {
            allowed: [
              { name: "sqs", label: "SQS access" },
              { name: "s3", label: "S3 read" },
            ],
          },
        )}
        value={undefined}
        onValue={onValue}
      />,
    );
    await userEvent.selectOptions(screen.getByLabelText("Replicas"), "S3 read");
    expect(onValue).toHaveBeenLastCalledWith("s3");
  });

  it("picks allowed names of a list with a checkbox for each", async () => {
    const onValue = vi.fn();
    render(
      <ParameterField
        id="p"
        rule={editable(
          { type: "array", items: { type: "string" } },
          {
            allowed: [
              { name: "sqs", label: "SQS access" },
              { name: "s3", label: "S3 read" },
            ],
          },
        )}
        value={["s3"]}
        onValue={onValue}
      />,
    );
    await userEvent.click(screen.getByRole("checkbox", { name: "SQS access" }));
    expect(onValue).toHaveBeenLastCalledWith(["sqs", "s3"]);
  });

  it("picks one name for a list whose choices are whole lists", () => {
    render(
      <ParameterField
        id="p"
        rule={editable(
          { type: "array", items: { type: "string" } },
          {
            allowed: [{ name: "base", label: "Base policies" }],
            allowedLists: true,
          },
        )}
        value="base"
        onValue={() => {}}
      />,
    );
    expect(screen.getByLabelText("Replicas").tagName).toBe("SELECT");
  });

  describe("a map of strings", () => {
    const map = { type: "object", additionalProperties: { type: "string" } };

    it("shows a key and value row for each entry", () => {
      render(
        <ParameterField
          id="p"
          rule={editable(map)}
          value={{ team: "orders" }}
          onValue={() => {}}
        />,
      );
      expect(screen.getByRole("textbox", { name: "Key" })).toHaveProperty(
        "value",
        "team",
      );
      expect(screen.getByRole("textbox", { name: "Value" })).toHaveProperty(
        "value",
        "orders",
      );
    });

    it("gives the map after an edit", async () => {
      const onValue = vi.fn();
      render(
        <ParameterField
          id="p"
          rule={editable(map)}
          value={undefined}
          onValue={onValue}
        />,
      );
      await userEvent.click(
        screen.getByRole("button", { name: "Add an entry" }),
      );
      await userEvent.type(
        screen.getByRole("textbox", { name: "Key" }),
        "tier",
      );
      await userEvent.type(
        screen.getByRole("textbox", { name: "Value" }),
        "gold",
      );
      expect(onValue).toHaveBeenLastCalledWith({ tier: "gold" }, []);
    });

    it("gives undefined once no entry is left", async () => {
      const onValue = vi.fn();
      render(
        <ParameterField
          id="p"
          rule={editable(map)}
          value={{ team: "orders" }}
          onValue={onValue}
        />,
      );
      await userEvent.click(
        screen.getByRole("button", { name: "Remove entry 1" }),
      );
      expect(onValue).toHaveBeenLastCalledWith(undefined, []);
    });

    it("gives the errors of the rows and hides the error of the parent", async () => {
      const onValue = vi.fn();
      render(
        <ParameterField
          id="p"
          rule={editable(map)}
          value={{ team: "orders" }}
          error="is required"
          onValue={onValue}
        />,
      );
      await userEvent.click(
        screen.getByRole("button", { name: "Add an entry" }),
      );
      await userEvent.type(
        screen.getAllByRole("textbox", { name: "Key" })[1],
        "team",
      );
      expect(onValue.mock.lastCall?.[1]).toEqual([
        { path: "1.key", message: "team is set twice" },
      ]);
      expect(screen.getByText("team is set twice")).toBeTruthy();
      expect(screen.queryByText("is required")).toBeNull();
    });
  });

  it.each<[string, ParameterRule["schema"]]>([
    ["a schema without a type", {}],
    ["no schema", undefined],
  ])("edits a value with %s as YAML", (_, schema) => {
    render(
      <ParameterField
        id="p"
        rule={editable(schema)}
        value="eu-west-1"
        onValue={() => {}}
      />,
    );
    expect(screen.getByLabelText("Replicas").tagName).toBe("TEXTAREA");
  });

  it("picks the editor of a field that the name gives", () => {
    render(
      <ParameterField
        id="p"
        rule={editable({ type: "string" }, { name: "storageSize" })}
        value="50Gi"
        onValue={() => {}}
      />,
    );
    expect(screen.getByRole("textbox", { name: "Amount" })).toHaveProperty(
      "value",
      "50",
    );
  });

  it("shows the locked summary of a rule that a claim cannot change", () => {
    render(
      <ParameterField
        id="p"
        rule={{
          name: "p",
          label: "Replicas",
          claimEditable: false,
          summary: "3",
        }}
        value={undefined}
        onValue={() => {}}
      />,
    );
    expect(screen.getByText("3")).toBeTruthy();
    expect(screen.getByText("Set by your admin")).toBeTruthy();
    expect(screen.queryByRole("spinbutton")).toBeNull();
  });
});
