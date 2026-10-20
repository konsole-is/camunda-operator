// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { StatusChip } from "./status-chip";

afterEach(cleanup);

describe("StatusChip", () => {
  it("says Ready and hides the reason when Ready is True", () => {
    render(<StatusChip condition={{ status: "True", reason: "Healthy" }} />);
    expect(screen.getByText("Ready")).toBeTruthy();
    expect(screen.queryByText(/Healthy/)).toBeNull();
  });

  it("shows the reason of a True Ready with readyReason", () => {
    render(
      <StatusChip
        condition={{ status: "True", reason: "Suspended" }}
        readyReason
      />,
    );
    expect(screen.getByText("Ready · Suspended")).toBeTruthy();
  });

  it("shows the reason when Ready is False", () => {
    render(
      <StatusChip
        condition={{
          status: "False",
          reason: "ZeebeNotReady",
          message: "1 of 3 brokers ready",
        }}
      />,
    );
    const chip = screen.getByText(/Not ready/);
    expect(chip.textContent).toContain("ZeebeNotReady");
    expect(chip.closest("[title]")?.getAttribute("title")).toBe(
      "1 of 3 brokers ready",
    );
  });

  it("says Not ready when Ready is False without a reason", () => {
    render(<StatusChip condition={{ status: "False" }} />);
    expect(screen.getByText("Not ready")).toBeTruthy();
  });

  it("says Unknown with the reason when Ready is Unknown", () => {
    render(
      <StatusChip condition={{ status: "Unknown", reason: "Reconciling" }} />,
    );
    expect(screen.getByText(/Unknown/).textContent).toContain("Reconciling");
  });

  it("says No status when the object has no Ready condition", () => {
    render(<StatusChip />);
    expect(screen.getByText("No status")).toBeTruthy();
  });

  it("says Loading and marks itself busy while the status loads", () => {
    render(<StatusChip loading />);
    expect(
      screen
        .getByText("Loading")
        .closest("[aria-busy]")
        ?.getAttribute("aria-busy"),
    ).toBe("true");
  });
});
