import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { KindMap } from "@/lib/k8s/kinds";
import { GitLayoutPreview } from "./git-layout-preview";

const meta = {
  title: "setup/GitLayoutPreview",
  component: GitLayoutPreview,
  args: {
    path: "clusters/prod",
    namespaced: KindMap.CamundaClusterClaim,
    clusterScoped: KindMap.Environment,
  },
} satisfies Meta<typeof GitLayoutPreview>;
export default meta;
type Story = StoryObj<typeof meta>;

export const UnderAFolder: Story = { name: "Under a folder" };

export const AtTheRoot: Story = { name: "At the root", args: { path: "" } };

export const InvalidPath: Story = {
  name: "Invalid path",
  args: { path: "a/../b" },
};
