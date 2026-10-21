// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { GitWaitingLine, type PushedObject } from "./git-waiting-line";

afterEach(cleanup);

const now = "2026-10-18T12:00:00Z";
const pushed: PushedObject = {
  path: "clusters/team-a/camundaclusterclaims/orders.yaml",
  revision: "bbb",
  pushedAt: "2026-10-18T11:58:00Z",
};
const line =
  "1 change waiting for GitOps: clusters/team-a/camundaclusterclaims/orders.yaml, pushed 2m ago.";

describe("GitWaitingLine", () => {
  it("shows the waiting line when the live revision differs", () => {
    render(<GitWaitingLine pushed={pushed} liveRevision="aaa" now={now} />);
    expect(screen.getByRole("status").textContent).toBe(line);
  });

  it("shows the line when the live object has no revision yet", () => {
    render(<GitWaitingLine pushed={pushed} now={now} />);
    expect(screen.getByRole("status").textContent).toBe(line);
  });

  it("renders nothing when the live object carries the pushed revision", () => {
    const { container } = render(
      <GitWaitingLine pushed={pushed} liveRevision="bbb" now={now} />,
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders nothing without a pushed object", () => {
    const { container } = render(
      <GitWaitingLine liveRevision="aaa" now={now} />,
    );
    expect(container.innerHTML).toBe("");
  });

  it("never names a GitOps tool", () => {
    render(<GitWaitingLine pushed={pushed} now={now} />);
    expect(screen.getByRole("status").textContent).not.toMatch(/argo|flux/i);
  });
});
