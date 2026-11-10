// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { DeletionChoice } from "@/lib/clusters/storage";
import { DeletionChoiceField } from "./deletion-choice";

afterEach(cleanup);

const refused: DeletionChoice = {
  defaultPolicy: "Delete",
  keepAllowed: false,
  alwaysDeleted: [
    { section: "backup-storage", label: "Ephemeral bucket (Backup storage)" },
  ],
  policy: "Delete",
};

const open: DeletionChoice = {
  defaultPolicy: "Delete",
  keepAllowed: true,
  alwaysDeleted: [],
  policy: "Delete",
};

const radio = (name: RegExp) =>
  screen.getByRole("radio", { name }) as HTMLInputElement;

describe("DeletionChoiceField", () => {
  it("asks what happens when the cluster is deleted", () => {
    render(<DeletionChoiceField choice={open} onChange={vi.fn()} />);

    expect(
      screen.getByRole("group", { name: "When the cluster is deleted" }),
    ).toBeTruthy();
  });

  it("disables Keep the resources with the options that cannot keep", () => {
    render(<DeletionChoiceField choice={refused} onChange={vi.fn()} />);

    expect(radio(/Keep the resources/).disabled).toBe(true);
    expect(radio(/Delete the resources/).disabled).toBe(false);
    const item = screen.getByText(/Ephemeral bucket \(Backup storage\)/);
    expect(within(item.closest("li")!).getByText("Always deleted")).toBeTruthy();
  });

  it("enables Keep the resources when every option can keep", () => {
    render(<DeletionChoiceField choice={open} onChange={vi.fn()} />);

    expect(radio(/Keep the resources/).disabled).toBe(false);
    expect(screen.queryByText("Always deleted")).toBeNull();
  });

  it("checks the policy of the choice", () => {
    render(
      <DeletionChoiceField
        choice={{ ...open, policy: "Orphan" }}
        onChange={vi.fn()}
      />,
    );

    expect(radio(/Keep the resources/).checked).toBe(true);
    expect(radio(/Delete the resources/).checked).toBe(false);
  });

  it("labels the default", () => {
    render(
      <DeletionChoiceField
        choice={{ ...open, defaultPolicy: "Orphan", policy: "Delete" }}
        onChange={vi.fn()}
      />,
    );

    expect(radio(/Keep the resources/).labels![0].textContent).toContain(
      "Default",
    );
    expect(radio(/Delete the resources/).labels![0].textContent).not.toContain(
      "Default",
    );
  });

  it("says why Delete is chosen when the default keeps and cannot", () => {
    render(
      <DeletionChoiceField
        choice={{ ...refused, defaultPolicy: "Orphan" }}
        onChange={vi.fn()}
      />,
    );

    expect(
      screen.getByText(
        "The default keeps the resources, but the options below cannot keep them.",
      ),
    ).toBeTruthy();
  });

  it("does not explain the default when Keep is allowed", () => {
    render(
      <DeletionChoiceField
        choice={{ ...open, defaultPolicy: "Orphan", policy: "Orphan" }}
        onChange={vi.fn()}
      />,
    );

    expect(screen.queryByText(/The default keeps the resources/)).toBeNull();
  });

  it("reports a pick", async () => {
    const onChange = vi.fn();
    render(<DeletionChoiceField choice={open} onChange={onChange} />);

    await userEvent.click(radio(/Keep the resources/));

    expect(onChange).toHaveBeenCalledWith("Orphan");
  });
});
