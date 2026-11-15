// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { ConditionBadge } from "./condition-badge";
import { condition } from "./fixtures";

afterEach(cleanup);

describe("ConditionBadge", () => {
  it.each([
    [condition("Accepted", "True", "Healthy"), "Accepted: True"],
    [
      condition("Accepted", "False", "SectionMissing"),
      "Accepted: False · SectionMissing",
    ],
    [condition("Ready", "True", "Suspended"), "Ready: True · Suspended"],
    [condition("Ready", "Unknown", "Pending"), "Ready: Unknown · Pending"],
    [undefined, "Ready: No status"],
  ])("shows %o as %s", (c, text) => {
    render(<ConditionBadge type={c?.type ?? "Ready"} condition={c} />);
    expect(screen.getByText(text)).toBeTruthy();
  });

  it("shows the message as the hover text", () => {
    render(
      <ConditionBadge
        type="Ready"
        condition={condition("Ready", "False", "AccessDenied", "No access.")}
      />,
    );
    expect(
      screen
        .getByText("Ready: False · AccessDenied")
        .closest("[title]")
        ?.getAttribute("title"),
    ).toBe("No access.");
  });
});
