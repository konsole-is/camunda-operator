import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import type { Resource } from "@/lib/k8s/kinds";
import { PlaneActions } from "./plane-actions";

const claim = (suspend?: boolean) =>
  ({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { namespace: "platform", name: "plane-a", resourceVersion: "7" },
    spec: { environments: ["platform"], ...(suspend && { suspend }) },
  }) as unknown as Resource<"CamundaManagementClusterClaim">;

const meta = {
  title: "planes/PlaneActions",
  component: PlaneActions,
  args: {
    claim: claim(),
    served: 3,
    apply: fn(async () => ({ ok: true as const, mode: "direct" as const })),
    deleteHref: "/fleet/planes/platform/plane-a/delete",
    now: "2026-11-15T12:00:00Z",
  },
} satisfies Meta<typeof PlaneActions>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Running: Story = { name: "Running plane" };

export const SuspendOpen: Story = {
  name: "Suspend open",
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Suspend" }));
    await expect(
      canvas.getByRole("button", { name: "Suspend the management plane" }),
    ).toBeVisible();
  },
};

export const Suspended: Story = {
  name: "Suspended, with Resume",
  args: { claim: claim(true) },
};

export const NotRead: Story = {
  name: "Plane not read",
  args: { unread: ["CamundaManagementCluster"] },
};
