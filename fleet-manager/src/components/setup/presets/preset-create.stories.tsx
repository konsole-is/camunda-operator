import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import { presetFixtures } from "./fixtures";
import { PresetCreate } from "./preset-create";

const meta = {
  title: "setup/presets/PresetCreate",
  component: PresetCreate,
  args: {
    sources: { ...presetFixtures(), errors: [] },
    mode: "direct",
    actions: {
      previewChange: fn(async () => ({ objects: [], errors: [] })),
      applyChange: fn(async () => ({
        ok: true as const,
        mode: "direct" as const,
      })),
    },
  },
} satisfies Meta<typeof PresetCreate>;
export default meta;
type Story = StoryObj<typeof meta>;

export const EmptyPreset: Story = { name: "Empty preset" };

export const CopyOfAnElasticsearchPreset: Story = {
  name: "Copy of an Elasticsearch preset",
  play: async ({ canvas }) => {
    await userEvent.click(canvas.getByRole("radio", { name: "Elasticsearch" }));
    await userEvent.selectOptions(
      canvas.getByRole("combobox", { name: "Copy from" }),
      "standard",
    );
    const settings = canvas.getByRole("region", {
      name: "Elasticsearch settings",
    });
    await expect(settings).toBeVisible();
    const replicas = within(settings).getByRole("group", { name: "replicas" });
    await expect(within(replicas).getByRole("spinbutton")).toHaveValue(3);
  },
};

export const ChangesOff: Story = {
  name: "Changes off",
  args: { mode: undefined },
};
