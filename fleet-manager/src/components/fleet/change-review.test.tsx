// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type {
  Change,
  KubeObject,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import { ChangeReview, keptPaths } from "./change-review";

const base = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "staging" },
};
const ref = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  name: "staging",
};

describe("keptPaths", () => {
  it("names a map entry that the preview keeps and the change leaves out", () =>
    expect(
      keptPaths(
        { ...base, spec: { sections: { access: {} } } },
        {
          ...base,
          spec: {
            sections: { access: {}, "backup-storage": { allowed: ["rustfs"] } },
          },
        },
      ),
    ).toEqual(["spec.sections.backup-storage"]));

  it("names a parent once", () =>
    expect(
      keptPaths(
        { ...base, spec: {} },
        { ...base, spec: { presets: { camundaCluster: ["small"] } } },
      ),
    ).toEqual(["spec.presets"]));

  it("names the paths in code-unit order", () =>
    expect(
      keptPaths(
        { ...base, spec: {} },
        { ...base, spec: { releases: { allowed: [] }, color: "#2E7D32" } },
      ),
    ).toEqual(["spec.color", "spec.releases"]));

  it("names a list that keeps a member the change leaves out", () =>
    expect(
      keptPaths(
        { ...base, spec: { releases: { allowed: ["r2"] } } },
        { ...base, spec: { releases: { allowed: ["r1", "r2"] } } },
      ),
    ).toEqual(["spec.releases.allowed"]));

  it("names no list that holds only the members of the change", () =>
    expect(
      keptPaths(
        { ...base, spec: { releases: { allowed: ["r2", "r1"] } } },
        { ...base, spec: { releases: { allowed: ["r1", "r2"] } } },
      ),
    ).toEqual([]));

  it("names nothing outside spec, and nothing without a preview", () => {
    expect(
      keptPaths(
        { ...base, spec: {} },
        {
          ...base,
          metadata: { name: "staging", labels: { a: "b" } },
          spec: {},
        },
      ),
    ).toEqual([]);
    expect(keptPaths({ ...base, spec: {} }, undefined)).toEqual([]);
  });
});

const applied: KubeObject = { ...base, spec: { displayName: "Stage" } };
const change: Change = {
  intent: "Change the environment staging",
  apply: [applied],
};
const clean: Preview = {
  objects: [
    {
      ref,
      before: { ...base, spec: { displayName: "Staging" } },
      after: applied,
      diff: [
        { op: "same", text: "spec:" },
        { op: "removed", text: "  displayName: Staging" },
        { op: "added", text: "  displayName: Stage" },
      ],
    },
  ],
  errors: [],
};

function setup({
  preview = clean,
  result = { ok: true, mode: "direct" },
  mode = "direct",
  gitLabel,
  hold,
  change: given = change,
  offerTakeOver,
}: {
  preview?: Preview | ((change: Change) => Preview);
  result?: WriteResult;
  mode?: WriteMode;
  gitLabel?: string;
  hold?: boolean;
  change?: Change;
  offerTakeOver?: boolean;
} = {}) {
  const props = {
    preview: vi.fn(async (c: Change) =>
      typeof preview === "function" ? preview(c) : preview,
    ),
    apply: vi.fn<(change: Change) => Promise<WriteResult>>(async () => result),
    show: vi.fn(),
    onWritten: vi.fn(),
    onCancel: vi.fn(),
    onWriting: vi.fn(),
  };
  const view = (c: Change) => (
    <ChangeReview
      change={c}
      mode={mode}
      diffLabel="Changes to the environment"
      effects={["Changes apply to new claims only."]}
      confirm={{ label: "Save", gitLabel }}
      savedMessage="The environment staging is saved."
      hold={hold}
      offerTakeOver={offerTakeOver}
      {...props}
    />
  );
  const { rerender } = render(view(given));
  return { ...props, rerender: (c: Change) => rerender(view(c)) };
}

const confirm = (name = "Save") => screen.getByRole("button", { name });

afterEach(cleanup);

describe("ChangeReview", () => {
  it("previews the change once and enables the confirm when the preview is clean", async () => {
    const { preview } = setup();
    await waitFor(() => expect(confirm()).toHaveProperty("disabled", false));
    expect(preview).toHaveBeenCalledTimes(1);
    expect(preview).toHaveBeenCalledWith(change);
    expect(
      screen.getByRole("figure", { name: "Changes to the environment" })
        .textContent,
    ).toContain("displayName: Stage");
    expect(screen.getByText("Changes apply to new claims only.")).toBeTruthy();
  });

  it("shows the errors of the preview and keeps the confirm disabled", async () => {
    setup({
      preview: {
        objects: [],
        errors: [
          {
            code: "Invalid",
            message: "releases.default must be one of releases.allowed",
            path: "spec",
          },
        ],
      },
    });
    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(
        "spec: releases.default must be one of releases.allowed",
      ),
    );
    expect(confirm()).toHaveProperty("disabled", true);
  });

  it("shows the Stale error of the write and keeps the confirm disabled", async () => {
    const { apply, show } = setup({
      result: {
        ok: false,
        errors: [
          {
            code: "Stale",
            message: "Environment staging changed since you read it.",
          },
        ],
      },
    });
    await waitFor(() => expect(confirm()).toHaveProperty("disabled", false));
    fireEvent.click(confirm());
    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(
        "Environment staging changed since you read it.",
      ),
    );
    expect(confirm()).toHaveProperty("disabled", true);
    expect(apply).toHaveBeenCalledTimes(1);
    expect(show).not.toHaveBeenCalled();
  });

  it("calls show with the saved message and then onWritten after an ok write", async () => {
    const { apply, show, onWritten } = setup();
    await waitFor(() => expect(confirm()).toHaveProperty("disabled", false));
    fireEvent.click(confirm());
    fireEvent.click(confirm());
    await waitFor(() => expect(onWritten).toHaveBeenCalled());
    expect(apply).toHaveBeenCalledTimes(1);
    expect(apply).toHaveBeenCalledWith(change);
    expect(show).toHaveBeenCalledWith(
      { ok: true, mode: "direct" },
      "The environment staging is saved.",
    );
  });

  it("reads Push to Git in Git mode, and the confirm label with gitLabel", async () => {
    setup({ mode: "git" });
    await waitFor(() =>
      expect(confirm("Push to Git")).toHaveProperty("disabled", false),
    );
    cleanup();
    setup({ mode: "git", gitLabel: "Add the namespace" });
    await waitFor(() =>
      expect(confirm("Add the namespace")).toHaveProperty("disabled", false),
    );
  });

  it("shows each other object of the preview as its own diff", async () => {
    setup({
      preview: {
        objects: [
          clean.objects[0],
          {
            ref: { apiVersion: "v1", kind: "Namespace", name: "team-a" },
            diff: [{ op: "added", text: "metadata:" }],
          },
        ],
        errors: [],
      },
    });
    await waitFor(() =>
      expect(
        screen.getByRole("figure", { name: "Changes to Namespace team-a" }),
      ).toBeTruthy(),
    );
  });

  it("keeps the confirm disabled when the preview changes nothing", async () => {
    setup({
      preview: {
        objects: [
          {
            ...clean.objects[0],
            diff: [{ op: "same", text: "spec:" }],
          },
        ],
        errors: [],
      },
    });
    await waitFor(() =>
      expect(screen.getByText("The change writes nothing.")).toBeTruthy(),
    );
    expect(confirm()).toHaveProperty("disabled", true);
  });

  it("offers Back while the preview runs, and hides it while the write runs", async () => {
    const { apply } = setup();
    expect(screen.getByRole("button", { name: "Back" })).toBeTruthy();
    await waitFor(() => expect(confirm()).toHaveProperty("disabled", false));
    apply.mockImplementation(() => new Promise<WriteResult>(() => {}));
    fireEvent.click(confirm());
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Back" })).toBeNull(),
    );
  });

  it("calls onWriting with true when the write starts and false when it settles", async () => {
    const { apply, onWriting } = setup();
    await waitFor(() => expect(confirm()).toHaveProperty("disabled", false));
    let settle: (result: WriteResult) => void = () => {};
    apply.mockImplementation(
      () => new Promise<WriteResult>((resolve) => (settle = resolve)),
    );
    fireEvent.click(confirm());
    await waitFor(() => expect(onWriting).toHaveBeenLastCalledWith(true));
    settle({ ok: true, mode: "direct" });
    await waitFor(() => expect(onWriting).toHaveBeenLastCalledWith(false));
  });

  it("takes the effects from the preview when effects is a function", async () => {
    const effects = vi.fn((p: Preview) => [`${p.objects.length} object`]);
    render(
      <ChangeReview
        change={change}
        mode="direct"
        diffLabel="Changes to the environment"
        effects={effects}
        confirm={{ label: "Save" }}
        savedMessage="Saved."
        preview={async () => clean}
        apply={async () => ({ ok: true, mode: "direct" })}
        show={vi.fn()}
      />,
    );
    await waitFor(() => expect(screen.getByText("1 object")).toBeTruthy());
    expect(effects).toHaveBeenLastCalledWith(clean);
  });

  it("keeps the confirm disabled while hold is set, also after a clean preview", async () => {
    const { preview, apply } = setup({ hold: true });
    await waitFor(() => expect(preview).toHaveBeenCalled());
    await waitFor(() =>
      expect(
        screen.queryByText("Checking the change with the API server."),
      ).toBeNull(),
    );
    expect(confirm()).toHaveProperty("disabled", true);
    fireEvent.click(confirm());
    expect(apply).not.toHaveBeenCalled();
  });
});

describe("ChangeReview with fields of another tool", () => {
  const key = "Environment//staging";
  const updated: Change = {
    intent: "Change the environment staging",
    apply: [applied],
    preconditions: { [key]: "12" },
  };
  const kubectl = {
    name: "kubectl-client-side-apply",
    operation: "Update",
    kind: "other" as const,
    known: true,
  };
  const displayName = {
    path: "spec.displayName",
    steps: [{ field: "spec" }, { field: "displayName" }],
    managers: [kubectl],
  };
  const color = {
    path: "spec.color",
    steps: [{ field: "spec" }, { field: "color" }],
    managers: [kubectl],
  };
  const conflicted: Preview = {
    objects: [],
    errors: [
      {
        code: "Conflict",
        message: 'conflict with "kubectl-client-side-apply"',
        path: "spec.displayName",
        manager: "kubectl-client-side-apply",
        ref,
      },
    ],
    ownership: [{ ref, changed: [displayName], cleared: [] }],
  };
  const taken: Preview = {
    objects: [
      {
        ref,
        diff: [
          { op: "same", text: "spec:" },
          { op: "removed", text: "  displayName: Staging" },
          { op: "added", text: "  displayName: Taken over" },
        ],
      },
    ],
    errors: [],
    ownership: conflicted.ownership,
  };
  const byChange = (c: Change) => (c.takeOver || c.remove ? taken : conflicted);
  const choice = () =>
    screen.getByRole("checkbox", { name: "Take over these fields" });

  it("Save stays off until the take-over is chosen, then saves the change with the lists", async () => {
    const { preview, apply } = setup({ change: updated, preview: byChange });
    await waitFor(() =>
      expect(
        screen.getByRole("group", { name: "Fields that another tool set" })
          .textContent,
      ).toContain(
        "spec.displayName: kubectl-client-side-apply set it. The save changes it.",
      ),
    );
    expect(confirm()).toHaveProperty("disabled", true);
    // The panel names the conflict, so no alert repeats it.
    expect(screen.queryByRole("alert")).toBeNull();

    fireEvent.click(choice());
    const withList = { ...updated, takeOver: { [key]: ["spec.displayName"] } };
    await waitFor(() => expect(confirm()).toHaveProperty("disabled", false));
    expect(preview).toHaveBeenLastCalledWith(withList);
    fireEvent.click(confirm());
    await waitFor(() => expect(apply).toHaveBeenCalledWith(withList));
  });

  it("a cleared field only: Save is on, and the note says the field stays", async () => {
    const { apply } = setup({
      change: updated,
      preview: {
        ...clean,
        ownership: [{ ref, changed: [], cleared: [color] }],
      },
    });
    await waitFor(() => expect(confirm()).toHaveProperty("disabled", false));
    expect(
      screen.getByText(
        "kubectl-client-side-apply set it. It stays unless you take it over.",
        { exact: false },
      ),
    ).toBeTruthy();
    fireEvent.click(confirm());
    await waitFor(() => expect(apply).toHaveBeenCalledWith(updated));
  });

  it("after the choice, names the default of a cleared field that the preview with the take-over still holds", async () => {
    const ownership = [{ ref, changed: [], cleared: [color, displayName] }];
    setup({
      change: updated,
      preview: (c) =>
        c.remove
          ? {
              ...taken,
              objects: [
                {
                  ref,
                  after: { ...applied, spec: { color: "blue" } },
                  diff: [],
                },
              ],
              ownership,
            }
          : { ...clean, ownership },
    });
    await waitFor(() => expect(choice()).toBeTruthy());
    fireEvent.click(choice());
    const group = () =>
      screen.getByRole("group", { name: "Fields that another tool set" });
    await waitFor(() =>
      expect(group().textContent).toContain(
        "spec.color: kubectl-client-side-apply set it. It gets its default value blue again.",
      ),
    );
    expect(group().textContent).toContain(
      "spec.displayName: kubectl-client-side-apply set it. The save removes it.",
    );
  });

  it("shows the diff of the change with the take-over before the save", async () => {
    setup({ change: updated, preview: byChange });
    await waitFor(() => expect(choice()).toBeTruthy());
    fireEvent.click(choice());
    await waitFor(() =>
      expect(
        screen.getByRole("figure", { name: "Changes to the environment" })
          .textContent,
      ).toContain("displayName: Taken over"),
    );
  });

  it("shows a Conflict of the preview with the take-over, and Save stays off", async () => {
    const unlisted = {
      code: "Conflict" as const,
      message:
        "The save would take spec.owner from kubectl-client-side-apply, and the review did not list it.",
      path: "spec.owner",
      manager: "kubectl-client-side-apply",
      ref,
    };
    setup({
      change: updated,
      preview: (c) =>
        c.takeOver ? { ...taken, errors: [unlisted] } : conflicted,
    });
    await waitFor(() => expect(choice()).toBeTruthy());
    fireEvent.click(choice());
    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(unlisted.message),
    );
    expect(confirm()).toHaveProperty("disabled", true);
  });

  it("keeps the errors of a refused save when the choice changes", async () => {
    const message =
      "The fleet manager saved the Environment staging, but did not remove spec.color. The Environment staging changed since you opened it.";
    setup({
      change: updated,
      preview: byChange,
      result: { ok: false, errors: [{ code: "Stale", message, ref }] },
    });
    await waitFor(() => expect(choice()).toBeTruthy());
    fireEvent.click(choice());
    await waitFor(() => expect(confirm()).toHaveProperty("disabled", false));
    fireEvent.click(confirm());
    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(message),
    );

    fireEvent.click(choice());
    fireEvent.click(choice());
    await waitFor(() =>
      expect(choice().getAttribute("aria-checked")).toBe("true"),
    );
    await waitFor(() =>
      expect(
        screen.queryByText("Checking the change with the API server."),
      ).toBeNull(),
    );
    expect(screen.getByRole("alert").textContent).toContain(message);
    expect(confirm()).toHaveProperty("disabled", true);
  });

  it("the choice resets for a new change", async () => {
    const { rerender, apply } = setup({ change: updated, preview: byChange });
    await waitFor(() => expect(choice()).toBeTruthy());
    fireEvent.click(choice());
    await waitFor(() => expect(confirm()).toHaveProperty("disabled", false));

    rerender({ ...updated });
    await waitFor(() =>
      expect(choice().getAttribute("aria-checked")).toBe("false"),
    );
    expect(confirm()).toHaveProperty("disabled", true);
    fireEvent.click(confirm());
    expect(apply).not.toHaveBeenCalled();
  });

  it("Git mode shows the kept notes and no choice", async () => {
    setup({
      mode: "git",
      change: updated,
      preview: {
        ...clean,
        objects: [
          {
            ...clean.objects[0],
            after: { ...applied, spec: { displayName: "Stage", color: "red" } },
          },
        ],
        ownership: [{ ref, changed: [], cleared: [color] }],
      },
    });
    await waitFor(() =>
      expect(
        screen.getByText(
          "spec.color stays, because another tool set it.",
        ),
      ).toBeTruthy(),
    );
    expect(
      screen.queryByRole("group", { name: "Fields that another tool set" }),
    ).toBeNull();
  });

  it("offers no choice when the page sets offerTakeOver to false", async () => {
    setup({ change: updated, preview: conflicted, offerTakeOver: false });
    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(
        'conflict with "kubectl-client-side-apply"',
      ),
    );
    expect(
      screen.queryByRole("group", { name: "Fields that another tool set" }),
    ).toBeNull();
    expect(confirm()).toHaveProperty("disabled", true);
  });

  it("an operator field keeps Save off and offers no choice", async () => {
    const operator = {
      ...displayName,
      managers: [
        {
          name: "camunda-operator/environment",
          operation: "Apply",
          kind: "operator" as const,
          known: true,
        },
      ],
    };
    setup({
      change: updated,
      preview: {
        ...conflicted,
        ownership: [{ ref, changed: [operator], cleared: [] }],
      },
    });
    await waitFor(() =>
      expect(screen.getByText(/Leave it as it is to save\./)).toBeTruthy(),
    );
    expect(screen.queryByRole("checkbox")).toBeNull();
    expect(confirm()).toHaveProperty("disabled", true);
  });
});
