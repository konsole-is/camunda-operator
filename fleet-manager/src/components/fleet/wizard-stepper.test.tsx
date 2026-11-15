// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { WizardStepper } from "./wizard-stepper";

afterEach(cleanup);

const steps = (errors: string[] = []) => [
  { id: "namespace", label: "Namespace" },
  { id: "storage", label: "Storage and access", errors },
  { id: "review", label: "Review" },
];

function Flow({ start = "namespace" }: { start?: string }) {
  const [current, setCurrent] = useState(start);
  const ids = steps().map((s) => s.id);
  const i = ids.indexOf(current);
  return (
    <WizardStepper
      steps={steps()}
      current={current}
      onNext={() => setCurrent(ids[i + 1])}
      onBack={() => setCurrent(ids[i - 1])}
      onStep={setCurrent}
    >
      <input aria-label="Field" />
    </WizardStepper>
  );
}

const stepHeading = (name: string) =>
  screen.getByRole("heading", { level: 2, name });

describe("WizardStepper", () => {
  it("blocks Next while the step reports an error", async () => {
    const onNext = vi.fn();
    render(
      <WizardStepper
        steps={steps(["Choose a secondary storage option"])}
        current="storage"
        onNext={onNext}
      >
        step
      </WizardStepper>,
    );
    const next = screen.getByRole("button", {
      name: "Next",
    }) as HTMLButtonElement;
    expect(next.disabled).toBe(true);
    expect(
      document.getElementById(next.getAttribute("aria-describedby")!)
        ?.textContent,
    ).toContain("Choose a secondary storage option");
    await userEvent.click(next);
    expect(onNext).not.toHaveBeenCalled();
  });

  it("lets Next go on when the step has no error", async () => {
    const onNext = vi.fn();
    render(
      <WizardStepper steps={steps()} current="storage" onNext={onNext}>
        step
      </WizardStepper>,
    );
    await userEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(onNext).toHaveBeenCalledTimes(1);
  });

  it("marks the current step and the state of each step", () => {
    render(
      <WizardStepper steps={steps(["x"])} current="storage">
        step
      </WizardStepper>,
    );
    const items = within(screen.getByRole("navigation", { name: "Steps" }))
      .getAllByRole("listitem")
      .map((li) => [li.textContent, li.getAttribute("aria-current")]);
    expect(items).toEqual([
      [expect.stringContaining("Done"), null],
      [expect.stringContaining("Has errors"), "step"],
      [expect.stringContaining("Not started"), null],
    ]);
  });

  it("tells the position of the current step", () => {
    render(
      <WizardStepper steps={steps()} current="storage">
        step
      </WizardStepper>,
    );
    expect(screen.getByText("Step 2 of 3")).toBeTruthy();
  });

  it("has no Next on the last step and no Back on the first", () => {
    const { rerender } = render(
      <WizardStepper steps={steps()} current="review">
        step
      </WizardStepper>,
    );
    expect(screen.queryByRole("button", { name: "Next" })).toBeNull();
    rerender(
      <WizardStepper steps={steps()} current="namespace" onBack={vi.fn()}>
        step
      </WizardStepper>,
    );
    expect(screen.queryByRole("button", { name: "Back" })).toBeNull();
  });

  it("goes back to a done step on a click", async () => {
    const onStep = vi.fn();
    render(
      <WizardStepper steps={steps()} current="review" onStep={onStep}>
        step
      </WizardStepper>,
    );
    await userEvent.click(screen.getByRole("button", { name: /Namespace/ }));
    expect(onStep).toHaveBeenCalledWith("namespace");
  });

  it("offers no click on the current step or a later step", () => {
    render(
      <WizardStepper steps={steps()} current="storage" onStep={vi.fn()}>
        step
      </WizardStepper>,
    );
    const nav = screen.getByRole("navigation", { name: "Steps" });
    expect(
      within(nav)
        .getAllByRole("button")
        .map((b) => b.textContent),
    ).toEqual([expect.stringContaining("Namespace")]);
  });

  it("offers no click without onStep", () => {
    render(
      <WizardStepper steps={steps()} current="review">
        step
      </WizardStepper>,
    );
    const nav = screen.getByRole("navigation", { name: "Steps" });
    expect(within(nav).queryAllByRole("button")).toEqual([]);
  });

  it("moves focus to the step heading on Next", async () => {
    render(<Flow />, { reactStrictMode: true });
    await userEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(document.activeElement).toBe(stepHeading("Storage and access"));
  });

  it("moves focus to the step heading on Next to the last step", async () => {
    render(<Flow start="storage" />, { reactStrictMode: true });
    await userEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(document.activeElement).toBe(stepHeading("Review"));
  });

  it("moves focus to the step heading on Back to the first step", async () => {
    render(<Flow start="storage" />, { reactStrictMode: true });
    await userEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(document.activeElement).toBe(stepHeading("Namespace"));
  });

  it("moves focus to the step heading on a click on an earlier step", async () => {
    render(<Flow start="review" />, { reactStrictMode: true });
    await userEvent.click(screen.getByRole("button", { name: /Namespace/ }));
    expect(document.activeElement).toBe(stepHeading("Namespace"));
  });

  it("leaves focus alone on the first render", () => {
    render(<Flow />, { reactStrictMode: true });
    expect(document.activeElement).toBe(document.body);
  });

  it("keeps focus where it is while the step stays", async () => {
    const { rerender } = render(
      <WizardStepper steps={steps()} current="storage">
        <input aria-label="Field" />
      </WizardStepper>,
      { reactStrictMode: true },
    );
    await userEvent.click(screen.getByLabelText("Field"));
    rerender(
      <WizardStepper
        steps={steps(["Choose a secondary storage option"])}
        current="storage"
      >
        <input aria-label="Field" />
      </WizardStepper>,
    );
    expect(document.activeElement).toBe(screen.getByLabelText("Field"));
  });

  it("blocks Next while an earlier step has errors and names that step", async () => {
    const onNext = vi.fn();
    render(
      <WizardStepper
        steps={[
          {
            id: "namespace",
            label: "Namespace",
            errors: ["The namespace team-a is gone"],
          },
          { id: "storage", label: "Storage and access" },
          { id: "review", label: "Review" },
        ]}
        current="storage"
        onNext={onNext}
      >
        step
      </WizardStepper>,
    );
    const next = screen.getByRole("button", {
      name: "Next",
    }) as HTMLButtonElement;
    expect(next.disabled).toBe(true);
    expect(
      document.getElementById(next.getAttribute("aria-describedby")!)
        ?.textContent,
    ).toContain("Namespace: The namespace team-a is gone");
    await userEvent.click(next);
    expect(onNext).not.toHaveBeenCalled();
  });

  it("ignores the errors of a later step", () => {
    render(
      <WizardStepper
        steps={[
          { id: "namespace", label: "Namespace" },
          {
            id: "storage",
            label: "Storage and access",
            errors: ["Not reached"],
          },
          { id: "review", label: "Review" },
        ]}
        current="namespace"
      >
        step
      </WizardStepper>,
    );
    expect(
      (screen.getByRole("button", { name: "Next" }) as HTMLButtonElement)
        .disabled,
    ).toBe(false);
  });

  it("lists the errors of an earlier step on the last step", () => {
    render(
      <WizardStepper
        steps={[
          {
            id: "namespace",
            label: "Namespace",
            errors: ["The namespace team-a is gone"],
          },
          { id: "storage", label: "Storage and access" },
          { id: "review", label: "Review" },
        ]}
        current="review"
      >
        step
      </WizardStepper>,
    );
    expect(
      screen.getByText("The namespace team-a is gone").closest("li")
        ?.textContent,
    ).toBe("Namespace: The namespace team-a is gone");
  });

  it("has no Back on the last step", () => {
    render(
      <WizardStepper steps={steps()} current="review" onBack={vi.fn()}>
        step
      </WizardStepper>,
    );
    expect(screen.queryByRole("button", { name: "Back" })).toBeNull();
  });
});
