import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import type { Change } from "@/lib/write/types";
import { presetFixtures } from "./fixtures";
import { PresetDetail } from "./preset-detail";

const meta = {
  title: "setup/presets/PresetDetail",
  component: PresetDetail,
  args: {
    sources: { ...presetFixtures(), errors: [], pending: [] },
    kind: "CamundaClusterPreset",
    name: "medium",
    secrets: {},
    secretsUnread: [],
    mode: "direct",
    actions: {
      // The dry run keeps exactly the applied spec.
      previewChange: fn(async (change: Change) => ({
        objects: change.apply.map((object) => ({
          ref: {
            apiVersion: object.apiVersion,
            kind: object.kind,
            name: object.metadata.name,
          },
          after: object,
          diff: [{ op: "added" as const, text: "(changed)" }],
        })),
        errors: [],
      })),
      applyChange: fn(async () => ({
        ok: true as const,
        mode: "direct" as const,
      })),
    },
  },
} satisfies Meta<typeof PresetDetail>;
export default meta;
type Story = StoryObj<typeof meta>;

export const UsedClusterPreset: Story = { name: "Used cluster preset" };

export const UsedClusterPresetWithAnEditUnderReview: Story = {
  name: "Used cluster preset with an edit under review",
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const brokers = within(
      canvas.getByRole("region", { name: "Brokers settings" }),
    );
    const replicas = within(
      brokers.getByRole("group", { name: "replicas" }),
    ).getByRole("spinbutton");
    await userEvent.clear(replicas);
    await userEvent.type(replicas, "5");
    await userEvent.click(canvas.getByRole("button", { name: "Review" }));
    await expect(await canvas.findByText("Pods roll")).toBeVisible();
  },
};

export const UnusedElasticsearchPresetWithDelete: Story = {
  name: "Unused Elasticsearch preset as YAML, with delete",
  args: { kind: "ElasticsearchClusterPreset", name: "standard" },
};

export const GitMode: Story = {
  name: "Git mode",
  args: { mode: "git", name: "spare" },
};
