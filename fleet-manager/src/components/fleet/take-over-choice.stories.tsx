import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { type ComponentProps, useState } from "react";
import { fn } from "storybook/test";
import type {
  ForeignField,
  ForeignManager,
  ObjectRef,
} from "@/lib/write/types";
import { TakeOverChoice } from "./take-over-choice";

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
const server: ForeignManager = {
  ...controller,
  name: "camunda-operator/databaseserver-recovery",
};
const restore: ForeignManager = {
  ...controller,
  name: "camunda-operator/pointintimerestore-recovery",
};
const field = (path: string, ...managers: ForeignManager[]): ForeignField => ({
  path,
  steps: path.split(".").map((name) => ({ field: name })),
  managers,
});

// The review holds the choice, as ChangeReview and ClaimReview do.
function Harness(props: ComponentProps<typeof TakeOverChoice>) {
  const [chosen, setChosen] = useState(props.chosen);
  return (
    <div className="max-w-2xl">
      <TakeOverChoice
        {...props}
        chosen={chosen}
        onChoose={(next) => {
          props.onChoose(next);
          setChosen(next);
        }}
      />
    </div>
  );
}

const meta = {
  title: "fleet/TakeOverChoice",
  component: TakeOverChoice,
  args: {
    ownership: [
      {
        ref: environment,
        changed: [field("spec.displayName", kubectl)],
        cleared: [],
      },
    ],
    chosen: false,
    onChoose: fn(),
  },
  render: (args) => <Harness key={JSON.stringify(args)} {...args} />,
} satisfies Meta<typeof TakeOverChoice>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Changed: Story = { name: "Changed field" };

export const Cleared: Story = {
  name: "Cleared fields",
  args: {
    ownership: [
      {
        ref: environment,
        changed: [],
        cleared: [
          field("spec.color", kubectl),
          field("spec.sections.backup-storage.required", kubectl),
        ],
      },
    ],
  },
};

export const GitOps: Story = {
  name: "GitOps tool",
  args: {
    ownership: [
      {
        ref: environment,
        changed: [field("spec.displayName", argo)],
        cleared: [field("spec.color", argo)],
      },
    ],
  },
};

export const Operator: Story = {
  name: "Operator field",
  args: {
    ownership: [
      {
        ref: claim("orders"),
        changed: [field("spec.releaseRef", controller)],
        cleared: [],
      },
    ],
  },
};

export const OperatorCleared: Story = {
  name: "Cleared fields of the operator",
  args: {
    ownership: [
      {
        ref: {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "DatabaseServerConfig",
          name: "main",
        },
        changed: [],
        cleared: [
          field("spec.pitr.lastRecovery", server),
          field("spec.recovery", restore),
        ],
      },
    ],
  },
};

export const Batch: Story = {
  name: "A batch of two claims",
  args: {
    ownership: [
      {
        ref: claim("orders"),
        changed: [field("spec.releaseRef", kubectl)],
        cleared: [],
      },
      {
        ref: claim("billing"),
        changed: [
          field("spec.releaseRef", {
            name: "my-script",
            operation: "Apply",
            kind: "other",
            known: false,
          }),
        ],
        cleared: [],
      },
    ],
  },
};

export const SeveralManagers: Story = {
  name: "Several managers on one field",
  args: {
    ownership: [
      {
        ref: environment,
        changed: [
          field("spec.displayName", kubectl, {
            name: "kubectl-edit",
            operation: "Update",
            kind: "other",
            known: true,
          }),
        ],
        cleared: [],
      },
    ],
  },
};

export const AmbiguousPath: Story = {
  name: "A path that names two fields",
  args: {
    ownership: [
      {
        ref: environment,
        changed: [field("spec.a.b", kubectl)],
        cleared: [
          {
            ...field("spec.a.b", kubectl),
            steps: [{ field: "spec" }, { field: "a.b" }],
          },
        ],
      },
    ],
  },
};

export const ClearedTaken: Story = {
  name: "Cleared fields after the choice",
  args: {
    ownership: [
      {
        ref: claim("orders"),
        changed: [],
        cleared: [
          field("spec.attachments.backupSchedule.retained.completed", kubectl),
          field("spec.platformConfigRef", kubectl),
        ],
      },
    ],
    chosen: true,
    taken: {
      objects: [
        {
          ref: claim("orders"),
          after: {
            apiVersion: "fleet.camunda.io/v1alpha1",
            kind: "CamundaClusterClaim",
            metadata: { name: "orders", namespace: "team-a" },
            spec: {
              attachments: { backupSchedule: { retained: { completed: 7 } } },
            },
          },
          diff: [],
        },
      ],
      errors: [],
    },
  },
};
