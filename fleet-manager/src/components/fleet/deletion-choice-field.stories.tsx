import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { type ComponentProps, useState } from "react";
import { fn } from "storybook/test";
import { DeletionChoiceField } from "./deletion-choice";

// The create flow holds the picked policy, so the story holds it too.
function DeletionChoiceFieldHarness(
  props: ComponentProps<typeof DeletionChoiceField>,
) {
  const [policy, setPolicy] = useState(props.choice.policy);
  return (
    <DeletionChoiceField
      {...props}
      choice={{ ...props.choice, policy }}
      onChange={(next) => {
        props.onChange(next);
        setPolicy(next);
      }}
    />
  );
}

const meta = {
  title: "fleet/DeletionChoiceField",
  component: DeletionChoiceField,
  args: { subject: "cluster", onChange: fn() },
  decorators: [
    (Story) => (
      <div className="max-w-3xl">
        <Story />
      </div>
    ),
  ],
  render: (args) => (
    <DeletionChoiceFieldHarness key={JSON.stringify(args.choice)} {...args} />
  ),
} satisfies Meta<typeof DeletionChoiceField>;
export default meta;
type Story = StoryObj<typeof meta>;

export const DeletionChoiceWithKeepRefused: Story = {
  name: "Deletion choice with Keep refused",
  args: {
    choice: {
      defaultPolicy: "Orphan",
      keepAllowed: false,
      alwaysDeleted: [
        {
          section: "backup-storage",
          label: "Archive bucket (Backup storage)",
        },
        { section: "access", label: "Gateway (Access)" },
      ],
      policy: "Delete",
    },
  },
};
