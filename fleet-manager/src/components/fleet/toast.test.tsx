// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { Toast } from "./toast";

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
