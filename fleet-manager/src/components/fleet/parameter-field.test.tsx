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
