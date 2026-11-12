import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "./tabs";

const meta = {
  title: "ui/Tabs",
  component: Tabs,
  args: { defaultValue: "overview", onValueChange: fn() },
} satisfies Meta<typeof Tabs>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  name: "Default",
  render: (args) => (
    <Tabs {...args}>
      <TabsList>
        <TabsTrigger value="overview">Overview</TabsTrigger>
        <TabsTrigger value="logs">Logs</TabsTrigger>
      </TabsList>
      <TabsContent value="overview">Overview of the cluster</TabsContent>
      <TabsContent value="logs">Logs of the cluster</TabsContent>
    </Tabs>
  ),
};

export const Line: Story = {
  name: "Line",
  render: (args) => (
    <Tabs {...args}>
      <TabsList variant="line">
        <TabsTrigger value="overview">Overview</TabsTrigger>
        <TabsTrigger value="logs">Logs</TabsTrigger>
      </TabsList>
      <TabsContent value="overview">Overview of the cluster</TabsContent>
      <TabsContent value="logs">Logs of the cluster</TabsContent>
    </Tabs>
  ),
};
