// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { PRECHECK_REASONS } from "@/lib/session";
import { PrecheckLine } from "./precheck-line";

afterEach(cleanup);

const claim = (status: "True" | "False", reason: string, message: string) => ({
  metadata: { generation: 2 },
  status: {
    conditions: [
      { type: "Ready", status, reason, message, observedGeneration: 2 },
    ],
  },
});

describe("PrecheckLine", () => {
  it.each(PRECHECK_REASONS)("shows the reason %s and the message", (reason) => {
    render(
      <PrecheckLine
        claim={claim(
          "False",
          reason,
          "Value of zeebe.replicas is not allowed.",
        )}
      />,
    );
    const line = screen.getByRole("status");
    expect(line.textContent).toContain(reason);
    expect(line.textContent).toContain(
      "Value of zeebe.replicas is not allowed.",
    );
  });

  it("shows nothing for a ready claim", () => {
    const { container } = render(
      <PrecheckLine claim={claim("True", "Ready", "The cluster is ready.")} />,
    );
    expect(container.textContent).toBe("");
  });

  it("shows nothing for a claim that is not ready for another reason", () => {
    const { container } = render(
      <PrecheckLine
        claim={claim("False", "Provisioning", "The cluster is starting.")}
      />,
    );
    expect(container.textContent).toBe("");
  });
});
