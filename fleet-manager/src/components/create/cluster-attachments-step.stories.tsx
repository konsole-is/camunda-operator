import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { type ComponentProps, useState } from "react";
import { fn } from "storybook/test";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import type { ClusterAttachments } from "@/lib/clusters/attachments";
import { CLUSTER_STEPS } from "@/lib/clusters/draft";
import type { StorageSummary } from "@/lib/clusters/storage";
import type { FieldLock, Step } from "@/lib/fields";
import { without, withValue } from "@/lib/session";
import { ClusterAttachmentsStep } from "./cluster-attachments-step";

const ACCESS_LOCK: FieldLock = {
  reason: "filled",
  section: "access",
  sectionLabel: "Access",
};

const summary = (over: Partial<StorageSummary> = {}): StorageSummary => ({
  secondaryStorageType: "elasticsearch",
  backupStorage: { section: "backup-storage", registration: "bucket" },
  optimizeUrl: false,
  slots: [],
  picks: [],
  ...over,
});

// ClusterAttachmentsStep holds no draft, so the harness holds the claim and
// applies each change to it, as the create flow does. With access, the URL
// is locked, as it is for any pick of the access section.
function ClusterAttachmentsStepHarness({
  initial,
  access,
  ...props
}: Omit<ComponentProps<typeof ClusterAttachmentsStep>, "value" | "lock"> & {
  initial?: ClusterAttachments;
  access: boolean;
}) {
  const [claim, setClaim] = useState<{
    spec?: { attachments?: ClusterAttachments };
  }>({ spec: { attachments: initial } });
  return (
    <WizardStepper steps={[...CLUSTER_STEPS]} current="attachments">
      <ClusterAttachmentsStep
        {...props}
        value={claim.spec?.attachments}
        lock={(steps: Step[]) => {
          const last = steps.at(-1);
          return access &&
            last &&
            "field" in last &&
            last.field === "externalUrl"
            ? ACCESS_LOCK
            : undefined;
        }}
        set={(steps, value) => {
          props.set(steps, value);
          setClaim((c) => withValue(c, steps, value));
        }}
        reset={(steps) => {
          props.reset(steps);
          setClaim((c) => without(c, steps));
        }}
      />
    </WizardStepper>
  );
}

const meta = {
  title: "create/ClusterAttachmentsStep",
  component: ClusterAttachmentsStepHarness,
  args: {
    initial: undefined,
    access: false,
    summary: summary(),
    managementAuthConfigs: ["central", "keycloak"],
    set: fn(),
    reset: fn(),
    invalid: fn(),
  },
  render: (args) => (
    <ClusterAttachmentsStepHarness key={JSON.stringify(args)} {...args} />
  ),
} satisfies Meta<typeof ClusterAttachmentsStepHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OptimizeWithAccess: Story = {
  name: "Optimize with access",
  args: {
    initial: { optimize: { managementAuthConfigRef: "central" } },
    access: true,
    summary: summary({ optimizeUrl: true }),
  },
};

export const OptimizeWithoutAccess: Story = {
  name: "Optimize without access",
  args: {
    initial: {
      optimize: {
        managementAuthConfigRef: "central",
        externalUrl: "https://optimize.orders.example.com",
      },
      backupSchedule: {
        schedule: "0 2 * * *",
        retained: { completed: 7, failed: 2 },
      },
    },
  },
};

export const PostgreSQLSecondaryStorage: Story = {
  name: "PostgreSQL secondary storage",
  args: { summary: summary({ secondaryStorageType: "rdbms" }) },
};

export const NoManagementAuthConfig: Story = {
  name: "No ManagementAuthConfig",
  args: { managementAuthConfigs: [] },
};

export const ScheduleWithoutBackupStorage: Story = {
  name: "Schedule without backup storage",
  args: { summary: summary({ backupStorage: undefined }) },
};
