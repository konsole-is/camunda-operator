import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import type { Step } from "@/lib/fields";
import { STALE_CLAIM_MESSAGE } from "@/lib/session";
import type { Preview } from "@/lib/write/types";
import { ClaimReviewHarness, ordersClaim } from "./fixtures";

const replicas: Step[] = [
  { field: "spec" },
  { field: "cluster" },
  { field: "zeebe" },
  { field: "replicas" },
];

const meta = {
  title: "fleet/ClaimReview",
  component: ClaimReviewHarness,
  args: {
    claim: {
      ...ordersClaim,
      metadata: { ...ordersClaim.metadata, resourceVersion: "41" },
    },
    changes: [{ steps: replicas, value: 5 }],
    mode: "direct",
    impacts: ["Broker pods restart."],
    preview: fn(async (): Promise<Preview> => ({ objects: [], errors: [] })),
    apply: fn(async () => ({ ok: true as const, mode: "direct" as const })),
    show: fn(),
    onWritten: fn(),
    onRefused: fn(),
    onCancel: fn(),
  },
} satisfies Meta<typeof ClaimReviewHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Direct: Story = { name: "Direct" };

export const Git: Story = { name: "Git", args: { mode: "git" } };

export const WithCaution: Story = {
  name: "With caution",
  args: {
    caution: "A backup of orders runs now. The rollout makes it fail.",
  },
};

export const PreviewRefused: Story = {
  name: "Preview refused",
  args: {
    changes: [{ steps: replicas, value: 0 }],
    impacts: [],
    preview: fn(async (): Promise<Preview> => ({
      objects: [],
      errors: [
        {
          code: "Invalid",
          message: "Invalid value: 0: must be greater than or equal to 1",
          path: "spec.cluster.zeebe.replicas",
        },
      ],
    })),
  },
};

export const Stale: Story = {
  name: "Stale",
  args: {
    preview: fn(async (): Promise<Preview> => ({
      objects: [],
      errors: [{ code: "Stale", message: STALE_CLAIM_MESSAGE }],
    })),
  },
};
