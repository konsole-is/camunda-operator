import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ComponentProps } from "react";
import { fn } from "storybook/test";
import { objectDiff } from "@/lib/diff";
import type { Change, Preview } from "@/lib/write/types";
import { ChangeReview } from "./change-review";
import { Toast, useWriteToast } from "./toast";

const sections = { "backup-storage": { allowed: ["rustfs"] } };
const before = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "staging" },
  spec: { displayName: "Staging", deletionPolicy: "Delete", sections },
};
const after = {
  ...before,
  spec: { displayName: "Stage", deletionPolicy: "Orphan" },
};
const kept = { ...after, spec: { ...after.spec, sections } };
const keptPreview = {
  objects: [
    {
      ref: {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "Environment",
        name: "staging",
      },
      before,
      after: kept,
      diff: objectDiff(before, kept),
    },
  ],
  errors: [],
};

// A page shows the toast of the write under the review.
function ChangeReviewHarness(props: ComponentProps<typeof ChangeReview>) {
  const { toast, show, dismiss } = useWriteToast();
  return (
    <div className="flex flex-col gap-4">
      <ChangeReview
        {...props}
        show={(result, message) => {
          props.show(result, message);
          show(result, message);
        }}
      />
      {toast && <Toast result={toast} onDismiss={dismiss} />}
    </div>
  );
}

const meta = {
  title: "fleet/ChangeReview",
  component: ChangeReview,
  args: {
    change: {
      intent: "Change the environment staging",
      apply: [after],
      preconditions: { "Environment//staging": "12" },
    },
    mode: "direct",
    diffLabel: "Changes to the environment",
    effects: ["Changes apply to new claims only."],
    confirm: { label: "Save" },
    savedMessage: "The environment staging is saved.",
    preview: fn(async () => keptPreview),
    apply: fn(async () => ({ ok: true as const, mode: "direct" as const })),
    show: fn(),
  },
  render: (args) => (
    <ChangeReviewHarness
      key={JSON.stringify([args.change, args.mode])}
      {...args}
    />
  ),
} satisfies Meta<typeof ChangeReview>;
export default meta;
type Story = StoryObj<typeof meta>;

const ref = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  name: "staging",
};

// In direct mode the preview names the cleared field that kubectl set, and
// the review offers to take it over.
export const KeptField: Story = {
  name: "Kept field",
  args: {
    preview: fn(async (): Promise<Preview> => ({
      ...keptPreview,
      ownership: [
        {
          ref,
          changed: [],
          cleared: [
            {
              path: "spec.sections",
              steps: [{ field: "spec" }, { field: "sections" }],
              managers: [
                {
                  name: "kubectl-client-side-apply",
                  operation: "Update",
                  kind: "other",
                  known: true,
                },
              ],
            },
          ],
        },
      ],
    })),
  },
};

export const Git: Story = { name: "Git", args: { mode: "git" } };
const kubectlField = {
  path: "spec.displayName",
  steps: [{ field: "spec" }, { field: "displayName" }],
  managers: [
    {
      name: "kubectl-client-side-apply",
      operation: "Update",
      kind: "other" as const,
      known: true,
    },
  ],
};
const ownership = [{ ref, changed: [kubectlField], cleared: [] }];
const takenOver = { ...before, spec: { ...before.spec, displayName: "Stage" } };

// Without the choice the dry run conflicts on the field. With it, the dry
// run of the change with the take-over is clean.
export const KubectlField: Story = {
  name: "A field that kubectl set",
  args: {
    change: {
      intent: "Change the environment staging",
      apply: [takenOver],
      preconditions: { "Environment//staging": "12" },
    },
    preview: fn(async (change: Change): Promise<Preview> =>
      change.takeOver
        ? {
            objects: [
              {
                ref,
                before,
                after: takenOver,
                diff: objectDiff(before, takenOver),
              },
            ],
            errors: [],
            ownership,
          }
        : {
            objects: [],
            errors: [
              {
                code: "Conflict" as const,
                message: 'conflict with "kubectl-client-side-apply"',
                path: "spec.displayName",
                manager: "kubectl-client-side-apply",
                ref,
              },
            ],
            ownership,
          },
    ),
  },
};

export const KubectlFieldGit: Story = {
  name: "A field that kubectl set, in Git mode",
  args: {
    mode: "git",
    change: {
      intent: "Change the environment staging",
      apply: [after],
      preconditions: { "Environment//staging": "12" },
    },
  },
};
