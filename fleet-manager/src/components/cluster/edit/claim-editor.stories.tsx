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
