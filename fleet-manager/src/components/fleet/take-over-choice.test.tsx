// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type {
  ApiError,
  ForeignField,
  ForeignManager,
  ObjectOwnership,
  ObjectRef,
} from "@/lib/write/types";
import {
  TakeOverChoice,
  takeOverOpen,
  withoutTakeOverConflicts,
} from "./take-over-choice";

const environment: ObjectRef = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  name: "staging",
};
const claim = (name: string): ObjectRef => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  namespace: "team-a",
  name,
});

const kubectl: ForeignManager = {
  name: "kubectl-client-side-apply",
  operation: "Update",
  kind: "other",
  known: true,
};
const edit: ForeignManager = {
  name: "kubectl-edit",
  operation: "Update",
  kind: "other",
  known: true,
};
const argo: ForeignManager = {
  name: "argocd-controller",
  operation: "Apply",
  kind: "gitops",
  known: true,
};
const controller: ForeignManager = {
  name: "CamundaClusterClaim/cluster/0b5c7a2e-1f0e-4f43-9d0a-6a0c2f1e9b11",
  operation: "Apply",
  kind: "operator",
  known: true,
};
const script: ForeignManager = {
  name: "my-script",
  operation: "Apply",
  kind: "other",
  known: false,
};

const field = (path: string, ...managers: ForeignManager[]): ForeignField => ({
  path,
  steps: path.split(".").map((name) => ({ field: name })),
  managers,
});

const owned = (
  ref: ObjectRef,
  changed: ForeignField[],
  cleared: ForeignField[] = [],
): ObjectOwnership => ({ ref, changed, cleared });

function show(
  ownership: ObjectOwnership[],
  chosen = false,
  labelOf?: (ref: ObjectRef) => string,
) {
  const onChoose = vi.fn();
  render(
    <TakeOverChoice
      ownership={ownership}
      chosen={chosen}
      onChoose={onChoose}
      labelOf={labelOf}
    />,
  );
  return { onChoose, group: screen.getByRole("group", { name: TITLE }) };
}

const TITLE = "Fields that another tool set";
const KUBECTL_LINE =
  "After the take-over, kubectl apply --server-side of an older file fails with a conflict on these fields. A plain kubectl apply of that file sets the old values again.";
const BLOCKED =
  "Take over the fields to save, or go back and leave them as they are.";

afterEach(cleanup);

describe("TakeOverChoice", () => {
  it("lists a changed and a cleared field with their managers", () => {
    const { group, onChoose } = show([
      owned(
        environment,
        [field("spec.displayName", kubectl, edit)],
        [field("spec.color", kubectl)],
      ),
    ]);
    expect(within(group).getByRole("heading", { name: TITLE })).toBeTruthy();
    expect(group.textContent).toContain(
      "spec.displayName: kubectl-client-side-apply and kubectl-edit set it. The save changes it.",
    );
    expect(group.textContent).toContain(
      "spec.color: kubectl-client-side-apply set it. It stays unless you take it over.",
    );
    expect(group.textContent).toContain(KUBECTL_LINE);
    expect(group.textContent).toContain(BLOCKED);
    // One object: no heading per object.
    expect(group.textContent).not.toContain("Environment staging");

    fireEvent.click(
      within(group).getByRole("checkbox", { name: "Take over these fields" }),
    );
    expect(onChoose).toHaveBeenCalledWith(true);
  });

  it("names a manager once when it owns a field with two operations", () => {
    const { group } = show([
      owned(environment, [
        field("spec.displayName", { ...kubectl, operation: "Apply" }, kubectl),
      ]),
    ]);
    expect(group.textContent).toContain(
      "spec.displayName: kubectl-client-side-apply set it. The save changes it.",
    );
  });

  it("says that a cleared field stays until the choice, and is removed after it", () => {
    const ownership = [owned(environment, [], [field("spec.color", kubectl)])];
    const first = show(ownership);
    expect(first.group.textContent).toContain(
      "spec.color: kubectl-client-side-apply set it. It stays unless you take it over.",
    );
    // A cleared field alone does not stop the save.
    expect(first.group.textContent).not.toContain(BLOCKED);
    cleanup();

    const { group } = show(ownership, true);
    expect(group.textContent).toContain(
      "spec.color: kubectl-client-side-apply set it. The save removes it.",
    );
    expect(
      within(group)
        .getByRole("checkbox", { name: "Take over these fields" })
        .getAttribute("aria-checked"),
    ).toBe("true");
  });

  it("warns once for each GitOps manager", () => {
    const { group } = show([
      owned(
        environment,
        [field("spec.displayName", argo)],
        [field("spec.color", argo)],
      ),
      owned(claim("orders"), [field("spec.releaseRef", argo)]),
    ]);
    const warning =
      "argocd-controller is a GitOps tool. On its next sync it sets its own value again, unless the source in Git changes too.";
    expect(group.textContent?.split(warning)).toHaveLength(2);
    expect(within(group).getByRole("checkbox")).toBeTruthy();
  });

  it("says that a tool whose name is no GitOps tool set a field of an object that a GitOps tool manages", () => {
    const { group } = show([
      owned(environment, [
        field("spec.displayName", { ...edit, kind: "gitops" }),
      ]),
    ]);
    expect(group.textContent).toContain(
      "kubectl-edit set a field of an object that a GitOps tool manages. On its next sync the GitOps tool can set its own value again, unless the source in Git changes too.",
    );
    expect(group.textContent).not.toContain("kubectl-edit is a GitOps tool");
  });

  it("does not say that the GitOps tool of the fleet manager changed the object when its name is no GitOps tool", () => {
    // kubectl in a CI job that applies the Git repository of the fleet
    // manager is the GitOps tool, found by the revision annotation.
    const { group } = show([
      owned(environment, [
        field("spec.displayName", {
          name: "kubectl",
          operation: "Apply",
          kind: "gitops",
          known: true,
        }),
      ]),
    ]);
    expect(group.textContent).toContain(
      "kubectl set a field of an object that a GitOps tool manages. On its next sync the GitOps tool can set its own value again, unless the source in Git changes too.",
    );
    expect(group.textContent).not.toContain("changed an object");
  });

  it("warns only about a tool that a row names", () => {
    const { group } = show([
      owned(claim("orders"), [
        field("spec.releaseRef", controller, script, {
          ...edit,
          kind: "gitops",
        }),
        field("spec.displayName", kubectl),
      ]),
    ]);
    expect(group.textContent).not.toContain("my-script");
    expect(group.textContent).not.toContain("kubectl-edit");
  });

  it("says once that the fleet manager does not know a tool", () => {
    const { group } = show([
      owned(
        environment,
        [field("spec.displayName", script)],
        [field("spec.color", script, kubectl)],
      ),
    ]);
    const line = "my-script: The fleet manager does not know this tool.";
    expect(group.textContent?.split(line)).toHaveLength(2);
    expect(group.textContent).not.toContain(
      "kubectl-client-side-apply: The fleet manager does not know this tool.",
    );
  });

  it("offers no checkbox for an operator field and names the controller", () => {
    const { group } = show([
      owned(
        claim("orders"),
        [field("spec.releaseRef", controller)],
        [field("spec.deletionPolicy", controller)],
      ),
    ]);
    expect(within(group).queryByRole("checkbox")).toBeNull();
    expect(group.textContent).toContain(
      `spec.releaseRef: ${controller.name} of the operator sets it. The fleet manager does not take it over, because the operator sets it again. Leave it as it is to save.`,
    );
    expect(group.textContent).toContain(
      `spec.deletionPolicy: ${controller.name} of the operator sets it. The fleet manager does not take it over, because the operator sets it again.`,
    );
    expect(group.textContent).not.toContain(
      "spec.deletionPolicy: Leave it as it is",
    );
    expect(group.textContent).not.toContain(KUBECTL_LINE);
    expect(group.textContent).not.toContain(BLOCKED);
  });

  it("offers the checkbox for the other fields beside an operator field", () => {
    const { group } = show([
      owned(claim("orders"), [
        field("spec.releaseRef", controller),
        field("spec.displayName", kubectl),
      ]),
    ]);
    expect(
      within(group).getByRole("checkbox", { name: "Take over these fields" }),
    ).toBeTruthy();
  });

  it("lists the fields of each object of a batch under its name", () => {
    const { group } = show(
      [
        owned(claim("orders"), [field("spec.releaseRef", kubectl)]),
        owned(claim("billing"), [field("spec.releaseRef", edit)]),
      ],
      false,
      (ref) => `Claim ${ref.name}`,
    );
    const orders = within(group).getByRole("list", { name: "Claim orders" });
    const billing = within(group).getByRole("list", { name: "Claim billing" });
    expect(orders.textContent).toContain("kubectl-client-side-apply set it");
    expect(billing.textContent).toContain("kubectl-edit set it");
    expect(within(group).getAllByRole("checkbox")).toHaveLength(1);
  });

  it("heads each object of a batch with its kind, name, and namespace by default", () => {
    const { group } = show([
      owned(claim("orders"), [field("spec.releaseRef", kubectl)]),
      owned({ ...claim("orders"), namespace: "team-b" }, [
        field("spec.releaseRef", kubectl),
      ]),
      owned(environment, [field("spec.displayName", kubectl)]),
    ]);
    for (const name of [
      "CamundaClusterClaim orders in team-a",
      "CamundaClusterClaim orders in team-b",
      "Environment staging",
    ])
      expect(within(group).getByRole("list", { name })).toBeTruthy();
  });

  it("offers no checkbox when a path names two fields, and says why", () => {
    const { group } = show([
      owned(
        environment,
        [field("spec.a.b", kubectl)],
        [
          {
            ...field("spec.a.b", kubectl),
            steps: [{ field: "spec" }, { field: "a.b" }],
          },
        ],
      ),
    ]);
    expect(within(group).queryByRole("checkbox")).toBeNull();
    expect(group.textContent).toContain(
      "spec.a.b names more than one field of the Environment staging. The fleet manager does not take over a field that it cannot name exactly.",
    );
    expect(group.textContent).toContain(
      "spec.a.b: kubectl-client-side-apply set it. It stays.",
    );
    expect(group.textContent).toContain(
      "To save, go back and leave the changed fields as they are, or change them with the tool that set them and open the page again.",
    );
    expect(group.textContent).not.toContain(BLOCKED);
  });

});

describe("takeOverOpen", () => {
  it("is true for an offered changed field until chosen", () => {
    const ownership = [
      owned(environment, [field("spec.displayName", kubectl)]),
    ];
    expect(takeOverOpen(ownership, false)).toBe(true);
    expect(takeOverOpen(ownership, true)).toBe(false);
  });

  it("is always true for an operator changed field", () => {
    const ownership = [
      owned(claim("orders"), [
        field("spec.releaseRef", controller),
        field("spec.displayName", kubectl),
      ]),
    ];
    expect(takeOverOpen(ownership, false)).toBe(true);
    expect(takeOverOpen(ownership, true)).toBe(true);
  });

  it("is false for cleared fields only, and for no fields", () => {
    expect(
      takeOverOpen(
        [owned(environment, [], [field("spec.color", controller, kubectl)])],
        false,
      ),
    ).toBe(false);
    expect(takeOverOpen([], false)).toBe(false);
  });

  it("is true for a changed field whose path names two fields, also when chosen", () => {
    const ownership = [
      owned(
        environment,
        [field("spec.a.b", kubectl)],
        [
          {
            ...field("spec.a.b", kubectl),
            steps: [{ field: "spec" }, { field: "a.b" }],
          },
        ],
      ),
    ];
    expect(takeOverOpen(ownership, true)).toBe(true);
  });
});

describe("withoutTakeOverConflicts", () => {
  const conflictOf = (ref: ObjectRef, path: string): ApiError => ({
    code: "Conflict",
    message: `conflict on ${path}`,
    path,
    ref,
  });

  it("drops the Conflict errors of an object with an offered changed field", () => {
    const errors: ApiError[] = [
      conflictOf(claim("orders"), "spec.releaseRef"),
      { code: "Invalid", message: "bad", ref: claim("orders") },
      conflictOf(claim("billing"), "spec.releaseRef"),
      conflictOf(claim("payments"), "spec.releaseRef"),
    ];
    expect(
      withoutTakeOverConflicts(errors, [
        owned(claim("orders"), [field("spec.releaseRef", kubectl)]),
        owned(claim("billing"), [field("spec.releaseRef", controller)]),
      ]),
    ).toEqual([errors[1], errors[2], errors[3]]);
  });
});
