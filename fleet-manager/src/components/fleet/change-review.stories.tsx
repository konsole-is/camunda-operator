import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ComponentProps } from "react";
import { fn } from "storybook/test";
import { objectDiff } from "@/lib/diff";
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
      force: true,
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
  render: (args) => <ChangeReviewHarness {...args} />,
} satisfies Meta<typeof ChangeReview>;
export default meta;
type Story = StoryObj<typeof meta>;

export const KeptField: Story = { name: "Kept field" };

export const Git: Story = { name: "Git", args: { mode: "git" } };
