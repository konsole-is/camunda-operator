import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ComponentDots } from "./component-dots";
import { NOW, fleet } from "./fixtures";
import { clusterRows } from "./model";

const meta = {
  title: "fleet-view/ComponentDots",
  component: ComponentDots,
  args: {
    dots: clusterRows(fleet, NOW).find((r) => r.name === "returns")!.components,
  },
} satisfies Meta<typeof ComponentDots>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Open: Story = { name: "Open", args: { defaultOpen: true } };
