import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import type { Resource } from "@/lib/k8s/kinds";
import type { Change, Preview, WriteResult } from "@/lib/write/types";
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
    apply: fn(async (): Promise<WriteResult> => ({ ok: true, mode: "direct" })),
    preview: fn(async (): Promise<Preview> => ({ objects: [], errors: [] })),
    deleteHref: "/fleet/planes/platform/plane-a/delete",
    writeMode: "direct",
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

export const KeycloakSuspendOpen: Story = {
  name: "Suspend open, own Keycloak",
  args: { ownKeycloak: true },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Suspend" }));
    await expect(
      canvas.getByText(/uses the Keycloak of this plane as its OIDC issuer/),
    ).toBeVisible();
  },
};

export const ClaimDeleting: Story = {
  name: "Claim being deleted",
  args: {
    claim: {
      ...claim(),
      metadata: {
        ...claim().metadata,
        deletionTimestamp: "2026-11-15T11:00:00Z",
      },
    } as Resource<"CamundaManagementClusterClaim">,
  },
};

export const ChangesOff: Story = {
  name: "Write mode not known",
  args: { writeMode: undefined },
};

const ref = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaManagementClusterClaim",
  namespace: "platform",
  name: "plane-a",
};
const conflict = {
  code: "Conflict" as const,
  message: 'conflict with "kubectl"',
  path: "spec.suspend",
  manager: "kubectl",
  ref,
};

export const SuspendAsksToTakeOver: Story = {
  name: "Suspend asks to take over",
  args: {
    // kubectl set spec.suspend, so a write without the take-over conflicts.
    apply: fn(async (change: Change): Promise<WriteResult> =>
      change.takeOver
        ? { ok: true, mode: "direct" }
        : { ok: false, errors: [conflict] },
    ),
    preview: fn(async (): Promise<Preview> => ({
      objects: [],
      errors: [conflict],
      ownership: [
        {
          ref,
          changed: [
            {
              path: "spec.suspend",
              steps: [{ field: "spec" }, { field: "suspend" }],
              managers: [
                {
                  name: "kubectl",
                  operation: "Apply",
                  kind: "other" as const,
                  known: true,
                },
              ],
            },
          ],
          cleared: [],
        },
      ],
    })),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Suspend" }));
    await userEvent.click(
      canvas.getByRole("button", { name: "Suspend the management plane" }),
    );
    await expect(
      await canvas.findByRole("group", {
        name: "Fields that another tool set",
      }),
    ).toBeVisible();
    await expect(
      canvas.getByRole("button", { name: "Suspend the management plane" }),
    ).toBeDisabled();
  },
};
