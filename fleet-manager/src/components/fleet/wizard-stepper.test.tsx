// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { WizardStepper } from "./wizard-stepper";

afterEach(cleanup);

describe("WizardStepper", () => {
  const steps = (errors: string[] = []) => [
    { id: "namespace", label: "Namespace" },
    { id: "storage", label: "Storage and access", errors },
    { id: "review", label: "Review" },
  ];

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
});
