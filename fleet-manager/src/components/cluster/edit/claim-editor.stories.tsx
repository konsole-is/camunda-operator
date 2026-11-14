import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import type { ClusterSlotContext } from "../slots";
import { ClaimEditorPanels } from "./claim-editor";
import { type ClaimActions, ClusterEditProvider } from "./cluster-edit";

const actions: ClaimActions = {
  preview: fn(async () => ({ objects: [], errors: [] })),
  apply: fn(async () => ({ ok: true, mode: "direct" }) as const),
};

const release = (version: string) =>
  ({
    kind: "CamundaRelease",
    metadata: { name: `camunda-${version.replaceAll(".", "-")}` },
    spec: { version },
  }) as unknown as Resource<"CamundaRelease">;

const preset = (name: string) =>
  ({
    kind: "CamundaClusterPreset",
    metadata: { name },
    spec: { cluster: {} },
  }) as unknown as Resource<"CamundaClusterPreset">;

// StoryCluster changes the cluster and the claim of a story. version is the
// reported version, none when empty. suspended suspends both.
type StoryCluster = {
  version?: string;
  suspended?: boolean;
  claimSpec?: Record<string, unknown>;
  claimStatus?: Record<string, unknown>;
  over?: Partial<ClusterSlotContext>;
};

function contextOf({
  version = "8.9.3",
  suspended = false,
  claimSpec = {},
  claimStatus = {},
  over = {},
}: StoryCluster): ClusterSlotContext {
  const metadata = { namespace: "team-payments", name: "orders" };
  const spec = {
    presetRef: "medium",
    releaseRef: "camunda-8-9-3",
    zeebe: { replicas: 3 },
    ...(suspended && { suspend: true }),
  };
  const cluster = {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: { ...metadata, labels: { [CLAIM_LABEL]: "orders" } },
    spec,
    status: version ? { management: { version } } : {},
  } as unknown as Resource<"CamundaCluster">;
  const claim = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { ...metadata, resourceVersion: "41" },
    spec: {
      presetRef: "medium",
      releaseRef: "camunda-8-9-3",
      cluster: { zeebe: { replicas: 3 } },
      ...(suspended && { suspend: true }),
      ...claimSpec,
    },
    status: { environment: "prod", ...claimStatus },
  } as unknown as Resource<"CamundaClusterClaim">;
  const environment = {
    kind: "Environment",
    metadata: { name: "prod" },
    spec: {
      displayName: "Production",
      presets: { camundaCluster: ["medium", "large"] },
      releases: {
        allowed: ["camunda-8-9-1", "camunda-8-9-3", "camunda-8-11-0"],
        default: "camunda-8-9-3",
      },
    },
  } as unknown as Resource<"Environment">;
  const releases = [release("8.9.1"), release("8.9.3"), release("8.11.0")];
  const presets = [preset("medium"), preset("large")];
  return {
    cluster,
    claim,
    merged: mergeCluster(presets[0].spec, releases[1].spec, spec),
    environment,
    environments: [environment],
    presets,
    releases,
    platformConfigs: [],
    managementAuthConfigs: [],
    sections: [],
    registrations: [],
    requests: [],
    backups: [],
    restores: [],
    backupStorage: { kind: "none", schedules: [] },
    schedules: [],
    fields: [],
    writeMode: "direct",
    now: "2026-11-10T12:00:00Z",
    ...over,
  };
}

// ACCESS gives the claim the access option Gateway, whose host prefix and
// sources the claim may change and whose zone the admin sets.
const ACCESS: StoryCluster = {
  claimSpec: {
    plugins: [
      {
        section: "access",
        registration: "gateway-access",
        parameters: { hostPrefix: "orders", sources: ["office"] },
      },
    ],
  },
  claimStatus: {
    resolvedPolicy: {
      sections: { access: { allowed: ["gateway-access"] } },
      parameters: {
        "gateway-access": {
          hostPrefix: { claimEditable: true, value: "camunda" },
          zone: { value: "payments.example.com" },
          sources: {
            claimEditable: true,
            allowed: [
              { name: "office", label: "Office", value: "10.0.0.0/16" },
              { name: "vpn", label: "VPN", value: "10.8.0.0/16" },
            ],
          },
        },
      },
    },
  },
  over: {
    sections: [
      {
        kind: "PluginSection",
        metadata: { name: "access" },
        spec: {
          label: "Access",
          selection: "One",
          appliesTo: ["CamundaClusterClaim"],
        },
      } as unknown as Resource<"PluginSection">,
    ],
    registrations: [
      {
        kind: "PluginRegistration",
        metadata: { name: "gateway-access" },
        spec: {
          label: "Gateway",
          deletionPolicies: ["Delete"],
          parameters: {
            type: "object",
            properties: {
              hostPrefix: {
                type: "string",
                description: "The first label of each host name.",
              },
              zone: { type: "string" },
              sources: { type: "array", items: { type: "string" } },
            },
          },
        },
      } as unknown as Resource<"PluginRegistration">,
    ],
  },
};

// ATTACHED gives the cluster Elasticsearch, backup storage, and an
// Optimize attachment, and a ManagementAuthConfig to pick.
const ATTACHED: StoryCluster = {
  claimSpec: {
    attachments: {
      optimize: {
        managementAuthConfigRef: "keycloak",
        externalUrl: "https://optimize.payments.example.com",
      },
      backupSchedule: { schedule: "0 2 * * *", retained: { completed: 7 } },
    },
  },
  over: {
    storage: {
      kind: "SecondaryStorageConfig",
      metadata: { namespace: "team-payments", name: "orders-storage" },
      spec: { type: "elasticsearch" },
    } as unknown as Resource<"SecondaryStorageConfig">,
    managementAuthConfigs: ["camunda", "keycloak"].map(
      (name) =>
        ({
          kind: "ManagementAuthConfig",
          metadata: { name },
          spec: {},
        }) as unknown as Resource<"ManagementAuthConfig">,
    ),
    backupStorage: {
      kind: "elasticsearch",
      bucket: { name: "backups", location: "s3://acme-backups/orders" },
      repository: "camunda-backups",
      schedules: [],
    },
  },
};

// A section in edit follows the Secrets of the namespace on /api/watch,
// which a static Storybook does not serve. The stories offer no Secret, so
// a watch that never connects shows the same panel and sends no request.
class IdleEventSource {
  static readonly CLOSED = 2;
  readyState = 0;
  addEventListener() {}
  close() {
    this.readyState = IdleEventSource.CLOSED;
  }
}

const meta = {
  title: "cluster/edit/Claim editor",
  component: ClaimEditorPanels,
  args: { context: contextOf({}) },
  beforeEach: () => {
    const real = globalThis.EventSource;
    globalThis.EventSource = IdleEventSource as unknown as typeof EventSource;
    return () => {
      globalThis.EventSource = real;
    };
  },
  decorators: [
    (Story, { args }) => (
      <div className="max-w-5xl">
        <ClusterEditProvider context={args.context} actions={actions}>
          <Story />
        </ClusterEditProvider>
      </div>
    ),
  ],
} satisfies Meta<typeof ClaimEditorPanels>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ReadViewWithEditButtons: Story = {
  name: "Read view with Edit buttons",
};

export const BasicsInEdit: Story = {
  name: "Basics in edit",
  args: { requested: "basics" },
  play: async ({ canvasElement }) => {
    const suspend = within(canvasElement).getByRole("checkbox", {
      name: "Suspend the cluster",
    });
    await userEvent.click(suspend);
    await expect(suspend).toBeChecked();
  },
};

export const ReleaseWithADowngrade: Story = {
  name: "Release with a downgrade",
  args: { requested: "release" },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.selectOptions(
      canvas.getByRole("combobox", { name: "Release" }),
      "camunda-8-9-1",
    );
    await expect(canvas.getByText(/is a downgrade\./)).toBeVisible();
  },
};

export const ReleaseWhileSuspended: Story = {
  name: "Release while suspended",
  args: {
    context: contextOf({ version: "", suspended: true }),
    requested: "release",
  },
};

export const ASectionWithAnEditableParameter: Story = {
  name: "A section with an editable parameter",
  args: { context: contextOf(ACCESS), requested: "section:access" },
};

export const AttachmentsInEdit: Story = {
  name: "Attachments in edit",
  args: { context: contextOf(ATTACHED), requested: "attachments" },
};

export const DeletionWithKeepRefused: Story = {
  name: "Deletion with Keep refused",
  args: { context: contextOf(ACCESS), requested: "deletion" },
};
