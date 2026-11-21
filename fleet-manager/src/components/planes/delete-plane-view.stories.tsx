import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import { deleteProgress } from "@/components/cluster/delete-cluster-model";
import { deleteClaimSteps } from "@/lib/claims/deletion";
import type { DeletionPolicy } from "@/lib/policy";
import { type DeletePlaneData, deletePlaneModel } from "./delete-plane-model";
import { DeletePlaneView } from "./delete-plane-view";

// The stories build their objects as plain JSON, as the cache gives them.
const object = (value: object) => value as never;

const CLAIM_UID = "3b2d7f0e-5c4a-4e0b-9f61-2a7d8c9e1f20";
const controlled = { ownerReferences: [{ controller: true, uid: CLAIM_UID }] };
const selections = [
  { section: "management-database", registration: "cloudnative-pg" },
  { section: "access", registration: "gateway" },
];

const claim = (deletionPolicy?: DeletionPolicy) =>
  object({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: {
      namespace: "platform",
      name: "plane-a",
      resourceVersion: "7",
      uid: CLAIM_UID,
    },
    spec: { environments: ["platform"], plugins: selections, deletionPolicy },
    status: {
      resolvedPolicy: { deletionPolicy: "Delete" },
      managementCluster: { name: "plane-a" },
      requests: selections.map((s) => ({
        ...s,
        name: `plane-a-${s.section}`,
      })),
    },
  });

const registration = (
  name: string,
  label: string,
  sections: string[],
  deletionPolicies: DeletionPolicy[],
) =>
  object({ metadata: { name }, spec: { label, sections, deletionPolicies } });

const data = (over: Partial<DeletePlaneData> = {}): DeletePlaneData => ({
  claim: claim(),
  plane: object({
    metadata: { namespace: "platform", name: "plane-a", ...controlled },
    spec: {},
    status: {
      managementAuthConfig: "plane-a",
      clusters: [
        { namespace: "team-orders", name: "orders", attached: true },
        { namespace: "team-payments", name: "payments", attached: true },
      ],
    },
  }),
  environments: [],
  sections: [
    object({
      metadata: { name: "management-database" },
      spec: {
        label: "Management database",
        order: 10,
        selection: "One",
        appliesTo: ["CamundaManagementClusterClaim"],
      },
    }),
    object({
      metadata: { name: "access" },
      spec: {
        label: "Access",
        order: 20,
        selection: "One",
        appliesTo: ["CamundaManagementClusterClaim"],
      },
    }),
  ],
  registrations: [
    registration(
      "cloudnative-pg",
      "CloudNativePG",
      ["management-database"],
      ["Delete", "Orphan"],
    ),
    registration("gateway", "Gateway", ["access"], ["Delete", "Orphan"]),
  ],
  requests: [
    object({
      metadata: {
        namespace: "platform",
        name: "plane-a-management-database",
        ...controlled,
      },
      status: {
        resources: [
          { kind: "DatabaseServer", name: "plane-a-pg", onOrphan: "Kept" },
          { kind: "Database", name: "plane-a-identity", onOrphan: "Kept" },
        ],
      },
    }),
    object({
      metadata: {
        namespace: "platform",
        name: "plane-a-access",
        ...controlled,
      },
      status: {
        resources: [
          { kind: "HTTPRoute", name: "plane-a", onOrphan: "Deleted" },
        ],
      },
    }),
  ],
  optimizes: [
    object({
      metadata: { namespace: "team-orders", name: "orders" },
      spec: { managementAuthRef: "plane-a" },
    }),
  ],
  errors: [],
  ...over,
});

const remove = deleteClaimSteps(
  data().claim,
  "Delete",
  "Delete the management plane plane-a",
).remove;

const meta = {
  title: "planes/DeletePlaneView",
  component: DeletePlaneView,
  args: {
    model: deletePlaneModel(data(), "Delete"),
    mode: "direct",
    stage: { kind: "choose" },
    errors: [],
    remove,
    policyStep: false,
    onPick: fn(),
    preview: fn(async () => ({ objects: [], errors: [] })),
    apply: fn(async () => ({ ok: true as const, mode: "direct" as const })),
    show: fn(),
    onWritten: fn(),
    onStopWaiting: fn(),
  },
} satisfies Meta<typeof DeletePlaneView>;
export default meta;
type Story = StoryObj<typeof meta>;

export const DeletePreselected: Story = {
  name: "Delete preselected, name typed",
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.type(
      await canvas.findByLabelText("Type plane-a to confirm"),
      "plane-a",
    );
    await expect(canvas.getByLabelText("Type plane-a to confirm")).toHaveValue(
      "plane-a",
    );
  },
};

export const KeepOff: Story = {
  name: "Keep off for an option that always deletes",
  args: {
    model: deletePlaneModel(
      data({
        registrations: [
          registration(
            "cloudnative-pg",
            "CloudNativePG",
            ["management-database"],
            ["Delete"],
          ),
          registration("gateway", "Gateway", ["access"], ["Delete", "Orphan"]),
        ],
      }),
      "Delete",
    ),
  },
};

export const OrphanChosen: Story = {
  name: "Keep chosen, two commits in Git mode",
  args: {
    model: deletePlaneModel(data({ claim: claim("Orphan") }), "Orphan"),
    mode: "git",
    policyStep: true,
  },
};

export const PlaneNotRead: Story = {
  name: "Plane not read",
  args: {
    model: deletePlaneModel(data(), "Delete", ["CamundaManagementCluster"]),
    errors: [{ kind: "CamundaManagementCluster", message: "connection lost" }],
  },
};

export const Deleting: Story = {
  name: "Deleting",
  args: {
    stage: {
      kind: "deleting",
      gitWaiting: false,
      progress: deleteProgress(deletePlaneModel(data(), "Delete").targets, {
        CamundaManagementCluster: [
          {
            metadata: {
              name: "plane-a",
              deletionTimestamp: "2026-11-16T10:00:00Z",
            },
          },
        ],
        PluginRequest: [],
        CamundaManagementClusterClaim: [{ metadata: { name: "plane-a" } }],
      }),
    },
  },
};

export const OwnKeycloak: Story = {
  name: "Plane with its own Keycloak",
  args: {
    model: deletePlaneModel(
      data({
        plane: object({
          metadata: { namespace: "platform", name: "plane-a", ...controlled },
          spec: { identityProvider: { keycloak: {} } },
          status: {
            managementAuthConfig: "plane-a",
            clusters: [
              { namespace: "team-orders", name: "orders", attached: true },
            ],
          },
        }),
      }),
      "Delete",
    ),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(
      await canvas.findByText(/This plane still serves 1 cluster\./),
    ).toBeVisible();
  },
};

export const PolicyOfAnotherTool: Story = {
  name: "Keep picked, and kubectl set the deletion policy",
  args: {
    model: deletePlaneModel(data({ claim: claim("Delete") }), "Orphan"),
    policyStep: true,
    policyOwned:
      "Another tool set the deletion policy of the claim, so the delete cannot change it without the take-over. Take over the field below, or choose Delete the resources to delete with the policy that the claim has.",
    preview: fn(async () => {
      const ref = {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "CamundaManagementClusterClaim",
        namespace: "platform",
        name: "plane-a",
      };
      return {
        objects: [],
        errors: [],
        ownership: [
          {
            ref,
            changed: [
              {
                path: "spec.deletionPolicy",
                steps: [{ field: "spec" }, { field: "deletionPolicy" }],
                managers: [
                  {
                    name: "kubectl-client-side-apply",
                    operation: "Update",
                    kind: "other" as const,
                    known: true,
                  },
                ],
              },
            ],
            cleared: [],
          },
        ],
      };
    }),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(
      await canvas.findByRole("group", {
        name: "Fields that another tool set",
      }),
    ).toHaveTextContent(
      "spec.deletionPolicy: kubectl-client-side-apply set it.",
    );
    await userEvent.type(
      canvas.getByLabelText("Type plane-a to confirm"),
      "plane-a",
    );
    const confirm = canvas.getByRole("button", {
      name: "Delete the management plane",
    });
    await expect(confirm).toBeDisabled();
    await userEvent.click(
      canvas.getByRole("checkbox", { name: "Take over these fields" }),
    );
    await expect(confirm).toBeEnabled();
  },
};
