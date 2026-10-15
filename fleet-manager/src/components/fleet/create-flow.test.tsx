// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ENVIRONMENT_LABEL, NamespacePicker } from "./namespace-picker";
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

describe("NamespacePicker", () => {
  const environment = {
    name: "prod",
    displayName: "Production",
    namespaceCreation: {
      allowed: true,
      labels: { "cost-center": "4711" },
      requiredLabelKeys: ["team"],
    },
  };
  const namespaces = [
    { name: "team-payments", environment: "prod" },
    { name: "sandbox", environment: "dev" },
  ];

  it("refuses a new namespace without a value for each required label key", async () => {
    const onChange = vi.fn();
    render(
      <NamespacePicker
        namespaces={namespaces}
        environment={environment}
        onChange={onChange}
      />,
    );
    await userEvent.click(
      screen.getByRole("radio", { name: "Create a namespace" }),
    );
    await userEvent.type(
      screen.getByRole("textbox", { name: "Name" }),
      "team-orders",
    );
    expect(onChange).toHaveBeenLastCalledWith(undefined, [
      { path: "labels.team", message: "Enter a value for team" },
    ]);

    await userEvent.type(
      screen.getByRole("textbox", { name: "team" }),
      "orders",
    );
    expect(onChange).toHaveBeenLastCalledWith(
      {
        kind: "new",
        name: "team-orders",
        labels: {
          "cost-center": "4711",
          team: "orders",
          [ENVIRONMENT_LABEL]: "prod",
        },
      },
      [],
    );
  });

  it("shows the error of a required key at its field once the user typed in it", async () => {
    render(
      <NamespacePicker namespaces={namespaces} environment={environment} />,
    );
    await userEvent.click(
      screen.getByRole("radio", { name: "Create a namespace" }),
    );
    const team = screen.getByRole("textbox", { name: "team" });
    await userEvent.type(team, "a b");
    expect(team.getAttribute("aria-invalid")).toBe("true");
  });

  it("refuses a name that is taken or not a DNS label", async () => {
    const onChange = vi.fn();
    render(
      <NamespacePicker
        namespaces={namespaces}
        environment={environment}
        onChange={onChange}
      />,
    );
    await userEvent.click(
      screen.getByRole("radio", { name: "Create a namespace" }),
    );
    await userEvent.type(
      screen.getByRole("textbox", { name: "team" }),
      "orders",
    );
    await userEvent.type(
      screen.getByRole("textbox", { name: "Name" }),
      "sandbox",
    );
    expect(onChange.mock.lastCall![1]).toEqual([
      { path: "name", message: "sandbox already exists" },
    ]);
    await userEvent.clear(screen.getByRole("textbox", { name: "Name" }));
    await userEvent.type(
      screen.getByRole("textbox", { name: "Name" }),
      "Team_Orders",
    );
    expect(onChange.mock.lastCall![1]).toEqual([
      {
        path: "name",
        message:
          "Use lowercase letters, digits, and '-', at most 63 characters",
      },
    ]);
  });

  it("keeps a fixed label when its key is also required, and does not ask for it", async () => {
    const onChange = vi.fn();
    render(
      <NamespacePicker
        namespaces={namespaces}
        environment={{
          ...environment,
          namespaceCreation: {
            ...environment.namespaceCreation,
            requiredLabelKeys: ["team", "cost-center"],
          },
        }}
        onChange={onChange}
      />,
    );
    await userEvent.click(
      screen.getByRole("radio", { name: "Create a namespace" }),
    );
    expect(screen.queryByRole("textbox", { name: "cost-center" })).toBeNull();
    await userEvent.type(
      screen.getByRole("textbox", { name: "Name" }),
      "team-orders",
    );
    await userEvent.type(
      screen.getByRole("textbox", { name: "team" }),
      "orders",
    );
    expect(onChange).toHaveBeenLastCalledWith(
      {
        kind: "new",
        name: "team-orders",
        labels: {
          "cost-center": "4711",
          team: "orders",
          [ENVIRONMENT_LABEL]: "prod",
        },
      },
      [],
    );
  });

  it("lists only the namespaces of the environment", () => {
    render(
      <NamespacePicker namespaces={namespaces} environment={environment} />,
    );
    const options = screen.getAllByRole("option").map((o) => o.textContent);
    expect(options).toContain("team-payments");
    expect(options).not.toContain("sandbox");
  });

  it("drops an existing namespace that the new environment does not hold", async () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <NamespacePicker
        namespaces={namespaces}
        environment={environment}
        onChange={onChange}
      />,
    );
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Namespace" }),
      "team-payments",
    );
    expect(onChange).toHaveBeenLastCalledWith(
      { kind: "existing", name: "team-payments" },
      [],
    );
    rerender(
      <NamespacePicker
        namespaces={namespaces}
        environment={{ name: "dev" }}
        onChange={onChange}
      />,
    );
    expect(onChange).toHaveBeenLastCalledWith(undefined, [
      { path: "name", message: "Pick a namespace" },
    ]);
  });

  it("drops a new namespace when the environment goes away", async () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <NamespacePicker
        namespaces={namespaces}
        environment={environment}
        onChange={onChange}
      />,
    );
    await userEvent.click(
      screen.getByRole("radio", { name: "Create a namespace" }),
    );
    await userEvent.type(
      screen.getByRole("textbox", { name: "Name" }),
      "team-orders",
    );
    await userEvent.type(
      screen.getByRole("textbox", { name: "team" }),
      "orders",
    );
    rerender(<NamespacePicker namespaces={namespaces} onChange={onChange} />);
    expect(onChange).toHaveBeenLastCalledWith(undefined, [
      { path: "name", message: "Pick a namespace" },
    ]);
    expect(
      screen.queryByRole("radio", { name: "Create a namespace" }),
    ).toBeNull();
  });

  it("offers no new namespace when the environment does not allow it", () => {
    render(
      <NamespacePicker
        namespaces={namespaces}
        environment={{ name: "prod" }}
      />,
    );
    expect(
      screen.queryByRole("radio", { name: "Create a namespace" }),
    ).toBeNull();
  });
});
