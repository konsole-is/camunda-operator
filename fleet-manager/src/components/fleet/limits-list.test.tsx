// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { LimitsList } from "./limits-list";

afterEach(cleanup);

describe("LimitsList", () => {
  it("shows each limit by its label with (limit set at install), its value, and its description", () => {
    render(
      <LimitsList
        limits={[
          {
            label: "Permissions boundary",
            value: "arn:aws:iam::123456789012:policy/camunda-boundary",
            description: "Each role of a request gets this boundary.",
          },
          { label: "Role name prefix", value: "camunda" },
        ]}
      />,
    );
    expect(
      screen.getByText("Permissions boundary (limit set at install)"),
    ).toBeTruthy();
    expect(
      screen.getByText("arn:aws:iam::123456789012:policy/camunda-boundary"),
    ).toBeTruthy();
    expect(
      screen.getByText("Each role of a request gets this boundary."),
    ).toBeTruthy();
    expect(
      screen.getByText("Role name prefix (limit set at install)"),
    ).toBeTruthy();
    expect(screen.getByText("camunda")).toBeTruthy();
  });

  it("renders nothing for no limit", () => {
    const { container } = render(<LimitsList limits={[]} />);
    expect(container.innerHTML).toBe("");
  });
});
