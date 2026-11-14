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

    it("gives {} for a map set to empty, and undefined to inherit", async () => {
      const onValue = vi.fn();
      render(
        <ParameterField
          id="p"
          rule={editable(map)}
          value={undefined}
          emptyMap="inherited"
          onValue={onValue}
        />,
      );
      await userEvent.click(
        screen.getByRole("button", { name: "Set to an empty map" }),
      );
      expect(onValue).toHaveBeenLastCalledWith({}, []);
      await userEvent.click(
        screen.getByRole("button", { name: "Use the inherited value" }),
      );
      expect(onValue).toHaveBeenLastCalledWith(undefined, []);
    });

    it("shows a value of {} as the empty map without emptyMap", () => {
      render(
        <ParameterField
          id="p"
          rule={editable(map)}
          value={{}}
          onValue={() => {}}
        />,
      );
      expect(screen.getByText("Empty map")).toBeTruthy();
      expect(screen.getByRole("button", { name: "Clear" })).toBeTruthy();
    });

    it("describes each input by the error and the rule hints of the field", () => {
      render(
        <ParameterField
          id="p"
          rule={editable({
            ...map,
            "x-kubernetes-validations": [
              { rule: "size(self) < 3", message: "Use at most 2 labels" },
            ],
          })}
          value={{ team: "orders" }}
          error="is required"
          onValue={() => {}}
        />,
      );
      for (const name of ["Key", "Value"])
        expect(
          screen.getByRole("textbox", {
            name,
            description:
              "is required Checked after you save Use at most 2 labels",
          }),
        ).toBeTruthy();
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

  it("checks its YAML against the registration schema, not the field of its name", async () => {
    const onValue = vi.fn();
    render(
      <ParameterField
        id="p"
        rule={editable(
          { type: "array", items: { type: "string" } },
          { name: "extraEnvFrom" },
        )}
        value={undefined}
        onValue={onValue}
      />,
    );
    await userEvent.type(screen.getByLabelText("Replicas"), "- a");
    expect(onValue).toHaveBeenLastCalledWith(["a"], []);
    expect(
      screen.getByLabelText("Replicas").getAttribute("aria-invalid"),
    ).toBeNull();
  });

  it("shows an empty map for a value of null", () => {
    render(
      <ParameterField
        id="p"
        rule={editable({
          type: "object",
          nullable: true,
          additionalProperties: { type: "string" },
        })}
        value={null}
        onValue={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: "Add an entry" })).toBeTruthy();
    expect(screen.queryByRole("textbox", { name: "Key" })).toBeNull();
  });

  describe("a name of a cluster field", () => {
    it.each<[string, ParameterRule["schema"]]>([
      [
        "resources",
        { type: "object", properties: { requests: { type: "object" } } },
      ],
      ["extraEnv", { type: "array", items: { type: "object" } }],
      ["tolerations", { type: "array" }],
    ])(
      "edits a %s parameter with a schema of the cluster field as YAML",
      (name, schema) => {
        render(
          <ParameterField
            id="p"
            rule={editable(schema, { name })}
            value={undefined}
            onValue={() => {}}
          />,
        );
        expect(screen.getByLabelText("Replicas").tagName).toBe("TEXTAREA");
      },
    );

    it("edits a storageSize string parameter as text", () => {
      render(
        <ParameterField
          id="p"
          rule={editable({ type: "string" }, { name: "storageSize" })}
          value="50Gi"
          onValue={() => {}}
        />,
      );
      expect(screen.getByLabelText("Replicas")).toHaveProperty("value", "50Gi");
      expect(screen.queryByRole("textbox", { name: "Amount" })).toBeNull();
    });

    it("edits an extraEnv map of strings with key and value rows", () => {
      render(
        <ParameterField
          id="p"
          rule={editable(
            { type: "object", additionalProperties: { type: "string" } },
            { name: "extraEnv" },
          )}
          value={{ TZ: "UTC" }}
          onValue={() => {}}
        />,
      );
      expect(screen.getByRole("textbox", { name: "Key" })).toHaveProperty(
        "value",
        "TZ",
      );
    });

    it("edits a tolerations string as text", () => {
      render(
        <ParameterField
          id="p"
          rule={editable({ type: "string" }, { name: "tolerations" })}
          value="all"
          onValue={() => {}}
        />,
      );
      expect(screen.getByLabelText("Replicas")).toHaveProperty("value", "all");
    });

    it.each<[string, ParameterRule["schema"], unknown, string]>([
      ["extraEnv", { type: "array" }, { TZ: "UTC" }, "TZ: UTC"],
      ["tolerations", undefined, "all", "all"],
      ["resources", {}, "big", "big"],
    ])(
      "shows a %s value of another shape as YAML",
      (name, schema, value, text) => {
        render(
          <ParameterField
            id="p"
            rule={editable(schema, { name })}
            value={value}
            onValue={() => {}}
          />,
        );
        const input = screen.getByLabelText("Replicas");
        expect(input.tagName).toBe("TEXTAREA");
        expect((input as HTMLTextAreaElement).value).toContain(text);
      },
    );
  });

  it("keeps the YAML editor once the YAML gives a value that rows read", () => {
    const rule = editable({
      type: "object",
      additionalProperties: { type: "string" },
    });
    const { rerender } = render(
      <ParameterField
        id="p"
        rule={rule}
        value={{ team: 1 }}
        onValue={() => {}}
      />,
    );
    rerender(
      <ParameterField
        id="p"
        rule={rule}
        value={{ team: "1" }}
        onValue={() => {}}
      />,
    );
    expect(screen.getByLabelText("Replicas").tagName).toBe("TEXTAREA");
  });

  it("shows an int-or-string that is a number as YAML", () => {
    render(
      <ParameterField
        id="p"
        rule={editable({ "x-kubernetes-int-or-string": true })}
        value={1024}
        onValue={() => {}}
      />,
    );
    const input = screen.getByLabelText("Replicas");
    expect(input.tagName).toBe("TEXTAREA");
    expect((input as HTMLTextAreaElement).value).toContain("1024");
  });

  it("edits an object with a declared property that is not a string as YAML", () => {
    render(
      <ParameterField
        id="p"
        rule={editable({
          type: "object",
          properties: { retries: { type: "integer" } },
          additionalProperties: { type: "string" },
        })}
        value={{ retries: 3 }}
        onValue={() => {}}
      />,
    );
    expect(screen.getByLabelText("Replicas").tagName).toBe("TEXTAREA");
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
