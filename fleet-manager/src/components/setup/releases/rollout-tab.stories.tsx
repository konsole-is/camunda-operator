import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";
import type { Change, KubeObject } from "@/lib/write/types";
import { cluster, rolloutData, targetRelease } from "./rollout-fixtures";
import { RolloutTab } from "./rollout-tab";

const changed = (o: KubeObject) => ({
  ref: {
    apiVersion: o.apiVersion,
    kind: o.kind,
    namespace: o.metadata.namespace,
    name: o.metadata.name,
  },
  diff: [
    { op: "removed" as const, text: "  releaseRef: camunda-8-9-0" },
    { op: "added" as const, text: "  releaseRef: camunda-8-9-1" },
  ],
});

// Two clusters can move: the fixture cluster fresh reports 8.9.0 here.
function twoCanMove() {
  const data = rolloutData();
  return {
    ...data,
    clusters: data.clusters.map((c) =>
      c.metadata?.name === "fresh"
        ? cluster("team-pay", "fresh", { version: "8.9.0" })
        : c,
    ),
  };
}

const meta = {
  title: "setup/releases/RolloutTab",
  component: RolloutTab,
  args: {
    release: targetRelease,
    data: rolloutData(),
    mode: "direct",
    actions: {
      previewMove: fn(async (c: Change) => ({
        objects: c.apply.map(changed),
        errors: [],
      })),
      applyMove: fn(async () => ({
        ok: true as const,
        mode: "direct" as const,
      })),
    },
  },
} satisfies Meta<typeof RolloutTab>;
export default meta;
type Story = StoryObj<typeof meta>;

export const EveryEligibility: Story = { name: "Every eligibility" };

export const BulkReviewWithImpacts: Story = {
  name: "Bulk review with impacts",
  args: { data: twoCanMove() },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(
      canvas.getByRole("button", { name: "Select all that can move" }),
    );
    await userEvent.click(canvas.getByRole("button", { name: "Move selected" }));
    await expect(
      await canvas.findByText(/team-pay\/fresh: The brokers restart/),
    ).toBeVisible();
  },
};

export const DowngradeConfirm: Story = {
  name: "Downgrade confirm",
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const row = canvas.getByRole("link", { name: "payments" }).closest("tr")!;
    await userEvent.click(
      within(row).getByRole("button", { name: "Confirm downgrade" }),
    );
    await expect(
      await canvas.findByRole("button", { name: "Confirm downgrade" }),
    ).toBeVisible();
  },
};

export const PartialWrite: Story = {
  name: "Partial write",
  args: {
    data: twoCanMove(),
    actions: {
      previewMove: fn(async (c: Change) => ({
        objects: c.apply.map(changed),
        errors: [],
      })),
      applyMove: fn(async () => ({
        ok: false as const,
        errors: [
          {
            code: "Stale" as const,
            message: "The CamundaClusterClaim fresh changed since you opened it.",
            ref: {
              apiVersion: "fleet.camunda.io/v1alpha1",
              kind: "CamundaClusterClaim",
              namespace: "team-pay",
              name: "fresh",
            },
          },
        ],
      })),
    },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(
      canvas.getByRole("button", { name: "Select all that can move" }),
    );
    await userEvent.click(canvas.getByRole("button", { name: "Move selected" }));
    const confirm = await canvas.findByRole("button", { name: "Move" });
    await waitFor(() => expect(confirm).toBeEnabled());
    await userEvent.click(confirm);
    await expect(
      await canvas.findByText(/^team-pay\/fresh: /),
    ).toBeVisible();
  },
};

export const ListThatCannotBeRead: Story = {
  name: "A list that cannot be read",
  args: {
    data: {
      ...rolloutData(),
      errors: [
        {
          kind: "LogicalBackupRDBMS",
          message:
            'logicalbackuprdbms.core.camunda.io is forbidden: User "fleet-manager" cannot list resource "logicalbackuprdbms"',
        },
      ],
    },
  },
};

export const WithoutSettings: Story = {
  name: "Without settings",
  args: { mode: undefined },
};
