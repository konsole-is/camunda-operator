import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { type ComponentProps, useState } from "react";
import { fn } from "storybook/test";
import type { ClusterAttachments } from "@/lib/clusters/attachments";
import type { FieldLock, Step } from "@/lib/fields";
import { without, withValue } from "@/lib/session";
import { AttachmentFields } from "./attachment-fields";

const ACCESS_LOCK: FieldLock = {
  reason: "filled",
  section: "access",
  sectionLabel: "Access",
};

const urlLock = (steps: Step[]) => {
  const last = steps.at(-1);
  return last && "field" in last && last.field === "externalUrl"
    ? ACCESS_LOCK
    : undefined;
};

// AttachmentFields holds no value, so the harness holds the claim and applies
// each change to it, as a page does.
function AttachmentFieldsHarness(
  props: ComponentProps<typeof AttachmentFields>,
) {
  const [claim, setClaim] = useState<{
    spec?: { attachments?: ClusterAttachments };
  }>({ spec: { attachments: props.value } });
  return (
    <AttachmentFields
      {...props}
      value={claim.spec?.attachments}
      set={(steps, value) => {
        props.set(steps, value);
        setClaim((c) => withValue(c, steps, value));
      }}
      reset={(steps) => {
        props.reset(steps);
        setClaim((c) => without(c, steps));
      }}
    />
  );
}

const meta = {
  title: "cluster/AttachmentFields",
  component: AttachmentFields,
  args: {
    value: undefined,
    set: fn(),
    reset: fn(),
    invalid: fn(),
    context: {
      secondaryStorage: "elasticsearch",
      backupStorage: true,
      managementAuthConfigs: ["central", "keycloak"],
      lock: () => undefined,
      errors: [],
      pending: 0,
    },
  },
  decorators: [
    (Story) => (
      <div className="max-w-3xl">
        <Story />
      </div>
    ),
  ],
  render: (args) => (
    <AttachmentFieldsHarness key={JSON.stringify(args.value)} {...args} />
  ),
} satisfies Meta<typeof AttachmentFields>;
export default meta;
type Story = StoryObj<typeof meta>;

export const BothOn: Story = {
  name: "Both on",
  args: {
    value: {
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

export const LockedUrl: Story = {
  name: "Locked URL",
  args: {
    value: { optimize: { managementAuthConfigRef: "central" } },
    context: {
      ...meta.args.context,
      lock: urlLock,
      lockedUrl: "https://optimize.orders.team-a.example.com",
    },
  },
};

export const CannotAttach: Story = {
  name: "Cannot attach",
  args: {
    context: {
      ...meta.args.context,
      secondaryStorage: "rdbms",
      backupStorage: false,
    },
  },
};
