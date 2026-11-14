import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fieldDocFor } from "@/lib/fielddocs";
import { FieldHelp } from "./field-help";

const meta = {
  title: "fleet/FieldHelp",
  component: FieldHelp,
  args: {
    label: "Partitions",
    doc: fieldDocFor(
      "core.camunda.io",
      "CamundaCluster",
      "spec.zeebe.partitions",
    ),
    operatorVersion: "0.2.3",
    defaultOpen: false,
  },
  render: (args) => (
    <div className="flex items-center gap-1 text-sm font-medium text-muted">
      <span>Partitions</span>
      <FieldHelp {...args} />
    </div>
  ),
} satisfies Meta<typeof FieldHelp>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Closed: Story = { name: "Closed" };

export const Open: Story = {
  name: "Open",
  args: { defaultOpen: true },
  decorators: [
    (Story) => (
      <div className="pb-48">
        <Story />
      </div>
    ),
  ],
};

export const OpenAtTheRightEdge: Story = {
  name: "Open at the right edge",
  args: { defaultOpen: true },
  decorators: [
    (Story) => (
      <div className="flex justify-end pb-48">
        <Story />
      </div>
    ),
  ],
};
