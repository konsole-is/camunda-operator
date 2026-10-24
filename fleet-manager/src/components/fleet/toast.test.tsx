// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { Toast, toastResultOf } from "./toast";

afterEach(cleanup);

describe("Toast", () => {
  it("shows the Git waiting state with the short commit and no tool name", () => {
    render(
      <Toast result={{ kind: "waiting", commit: "4f1c3a7e9b2d8c6f0a5e" }} />,
    );
    const status = screen.getByRole("status");
    expect(status.textContent).toContain("Waiting for GitOps to apply");
    expect(status.textContent).toContain("4f1c3a7");
    expect(status.textContent).not.toMatch(/argo|flux/i);
  });

  it("announces an error as an alert", () => {
    render(
      <Toast
        result={{
          kind: "error",
          message: "The claim changed since you opened it.",
        }}
      />,
    );
    expect(screen.getByRole("alert").textContent).toContain(
      "The claim changed since you opened it.",
    );
  });

  it("shows a saved result as a status", () => {
    render(
      <Toast result={{ kind: "saved", message: "Saved the claim orders" }} />,
    );
    expect(screen.getByRole("status").textContent).toContain(
      "Saved the claim orders",
    );
  });
});

describe("toastResultOf", () => {
  it("maps a direct save to saved with the message of the page", () => {
    expect(
      toastResultOf({ ok: true, mode: "direct" }, "Saved the claim orders"),
    ).toEqual({
      kind: "saved",
      message: "Saved the claim orders",
    });
  });

  it("maps a Git push to waiting with the commit", () => {
    expect(
      toastResultOf(
        {
          ok: true,
          mode: "git",
          commit: "4f1c3a7e9b",
          revisions: {},
          paths: {},
        },
        "x",
      ),
    ).toEqual({ kind: "waiting", commit: "4f1c3a7e9b" });
  });

  it("maps a refusal to an error with the API messages", () => {
    expect(
      toastResultOf(
        {
          ok: false,
          errors: [
            {
              code: "Stale",
              message:
                "The CamundaClusterClaim orders changed since you opened it.",
            },
            { code: "Invalid", message: "spec.presetRef: Required value" },
          ],
        },
        "Saved",
      ),
    ).toEqual({
      kind: "error",
      message:
        "The CamundaClusterClaim orders changed since you opened it. spec.presetRef: Required value",
    });
  });
});
