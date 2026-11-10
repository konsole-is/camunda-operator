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
  gitConfirmLabel,
}: {
  preview?: Preview;
  result?: WriteResult;
  mode?: WriteMode;
  gitConfirmLabel?: string;
} = {}) {
  const props = {
    preview: vi.fn(async () => preview),
    apply: vi.fn<(change: Change) => Promise<WriteResult>>(async () => result),
    show: vi.fn(),
    onWritten: vi.fn(),
    onCancel: vi.fn(),
  };
  render(
    <ChangeReview
      change={change}
      mode={mode}
      diffLabel="Changes to the environment"
      effects={["Changes apply to new claims only."]}
      confirmLabel="Save"
      gitConfirmLabel={gitConfirmLabel}
      savedMessage="The environment staging is saved."
      {...props}
    />,
  );
  return props;
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

  it("reads Push to Git in Git mode, and the confirm label with gitConfirmLabel", async () => {
    setup({ mode: "git" });
    await waitFor(() =>
      expect(confirm("Push to Git")).toHaveProperty("disabled", false),
    );
    cleanup();
    setup({ mode: "git", gitConfirmLabel: "Add the namespace" });
    await waitFor(() =>
      expect(confirm("Add the namespace")).toHaveProperty("disabled", false),
    );
  });

  it("notes each kept field", async () => {
    setup({
      preview: {
        ...clean,
        objects: [
          {
            ...clean.objects[0],
            after: {
              ...applied,
              spec: {
                displayName: "Stage",
                sections: { "backup-storage": { allowed: ["rustfs"] } },
              },
            },
          },
        ],
      },
    });
    await waitFor(() =>
      expect(
        screen.getByText(
          "spec.sections stays, because another field manager set it.",
        ),
      ).toBeTruthy(),
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
});
