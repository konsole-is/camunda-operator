import type { Story } from "@/gallery/registry";
import type { Resource } from "@/lib/k8s/kinds";
import { ClaimEditorStory, type StoryCluster } from "./claim-editor-story";
import {
  WAITING_FOR_GITOPS,
  WAITING_ON_LIVE,
  WaitingPanel,
} from "./waiting-panel";

// ACCESS gives the claim the access option Gateway, whose host prefix the
// claim may change and whose zone the admin sets.
const ACCESS: StoryCluster = {
  claimSpec: {
    plugins: [
      {
        section: "access",
        registration: "gateway-access",
        parameters: { hostPrefix: "orders" },
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

export const claimEditorStories: Story[] = [
  {
    component: "Claim editor",
    name: "Read view with Edit buttons",
    render: () => <ClaimEditorStory />,
  },
  {
    component: "Claim editor",
    name: "Basics in edit",
    render: () => (
      <ClaimEditorStory
        panel="basics"
        changes={[[[{ field: "spec" }, { field: "suspend" }], true]]}
      />
    ),
  },
  {
    component: "Claim editor",
    name: "Release with a downgrade",
    render: () => (
      <ClaimEditorStory
        panel="release"
        changes={[
          [[{ field: "spec" }, { field: "releaseRef" }], "camunda-8-9-1"],
          [[{ field: "spec" }, { field: "allowVersionDowngrade" }], "8.9.1"],
        ]}
      />
    ),
  },
  {
    component: "Claim editor",
    name: "Release while suspended",
    render: () => (
      <ClaimEditorStory
        panel="release"
        cluster={{ version: "", suspended: true }}
      />
    ),
  },
  {
    component: "Claim editor",
    name: "A section with an editable parameter",
    render: () => <ClaimEditorStory panel="section:access" cluster={ACCESS} />,
  },
  {
    component: "Claim editor",
    name: "Attachments in edit",
    render: () => <ClaimEditorStory panel="attachments" cluster={ATTACHED} />,
  },
  {
    component: "Claim editor",
    name: "Deletion with Keep refused",
    render: () => <ClaimEditorStory panel="deletion" cluster={ACCESS} />,
  },
  {
    component: "WaitingPanel",
    name: "A push of this page",
    render: () => (
      <div className="max-w-3xl">
        <WaitingPanel text={WAITING_FOR_GITOPS} onStopWaiting={() => {}} />
      </div>
    ),
  },
  {
    component: "WaitingPanel",
    name: "A push of an earlier visit",
    render: () => (
      <div className="max-w-3xl">
        <WaitingPanel text={WAITING_ON_LIVE} onStopWaiting={() => {}} />
      </div>
    ),
  },
];
