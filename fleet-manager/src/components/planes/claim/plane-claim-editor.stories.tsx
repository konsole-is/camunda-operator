import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import type { ClaimActions } from "@/components/fleet/claim-edit";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import { IDENTITY_INITIAL_CLAIM_ANNOTATION } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import type { PlaneClaim } from "@/lib/session";
import type { PlanePageData } from "../plane-view";
import { PlaneClaimEditProvider } from "./plane-claim-edit";
import { PlaneClaimEditor } from "./plane-claim-editor";

const actions: ClaimActions = {
  preview: fn(async () => ({ objects: [], errors: [] })),
  apply: fn(async () => ({ ok: true, mode: "direct" }) as const),
};

const SECRETS = staticSecretRefs({
  platform: [
    { name: "smtp", keys: ["username", "password"] },
    { name: "admin-password", keys: ["password"] },
  ],
});

const config = (name: string, oidc: boolean) =>
  ({
    kind: "CamundaPlatformConfig",
    metadata: { name },
    spec: {
      auth: oidc
        ? { method: "oidc", oidc: { issuerUrl: "https://sso.example.com" } }
        : { method: "basic" },
    },
  }) as unknown as Resource<"CamundaPlatformConfig">;

const keycloakClaim = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaManagementClusterClaim",
  metadata: { namespace: "platform", name: "mgmt", resourceVersion: "12" },
  spec: {
    platformConfigRef: "sso",
    environments: ["production"],
    clusterSelector: {
      matchLabels: { team: "orders" },
      matchExpressions: [{ key: "tier", operator: "In", values: ["gold"] }],
    },
    management: {
      identityProvider: { keycloak: {} },
      identity: { admin: { username: "admin", email: "admin@example.com" } },
      console: {},
      webModeler: {
        mail: {
          smtpHost: "smtp.example.com",
          smtpPort: 587,
          fromAddress: "modeler@example.com",
          tls: true,
          credentialsSecretRef: { name: "smtp" },
        },
      },
    },
    versions: {
      identity: "8.9.14",
      keycloak: "26.6.4",
      console: "8.9.133",
      webModeler: "8.9.15",
    },
    plugins: [
      {
        section: "databases",
        registration: "postgres",
        parameters: { storageSize: "10Gi" },
      },
      { section: "access", registration: "gateway" },
    ],
    deletionPolicy: "Delete",
  },
  status: {
    resolvedPolicy: {
      sections: {
        databases: { allowed: ["postgres"] },
        access: { allowed: ["gateway"] },
      },
      parameters: {
        postgres: {
          storageSize: { claimEditable: true, value: "5Gi" },
          storageClass: { value: "fast" },
        },
      },
    },
  },
} as unknown as PlaneClaim;

const oidcClaim = {
  ...keycloakClaim,
  spec: {
    ...keycloakClaim.spec,
    management: {
      identityProvider: { oidc: {} },
      identity: { admin: { claimName: "sub", claimValue: "4f1c" } },
      console: {},
    },
    versions: { identity: "8.9.14", console: "8.9.133" },
    plugins: [{ section: "databases", registration: "postgres" }],
  },
} as PlaneClaim;

function planeOf(annotations: Record<string, string> = {}) {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: {
      namespace: "platform",
      name: "mgmt",
      labels: { [CLAIM_LABEL]: "mgmt" },
      annotations,
    },
    spec: {
      identityProvider: {
        keycloak: { externalUrl: "https://mgmt.example.com/auth" },
      },
      identity: { externalUrl: "https://mgmt.example.com/identity" },
      console: { externalUrl: "https://mgmt.example.com/console" },
      webModeler: {
        externalUrl: "https://mgmt.example.com/modeler",
        websocketsExternalUrl: "wss://mgmt.example.com/modeler-ws",
      },
    },
  } as unknown as Resource<"CamundaManagementCluster">;
}

const cluster = (namespace: string, name: string, labels = {}) =>
  ({
    kind: "CamundaCluster",
    metadata: { namespace, name, labels: { tier: "gold", ...labels } },
    spec: { platformConfigRef: "basic" },
    status: { gateway: { grpc: `${name}:26500` } },
  }) as unknown as Resource<"CamundaCluster">;

const data: PlanePageData = {
  namespace: "platform",
  name: "mgmt",
  now: "2026-11-16T12:00:00Z",
  namespaces: [],
  environments: [],
  optimizes: [],
  managementAuthConfigs: [],
  pods: [],
  platformConfigs: [],
  registrations: [],
  claimTab: {
    clusters: [
      cluster("team-orders", "orders", { team: "orders" }),
      cluster("team-qa", "qa", { team: "orders" }),
    ],
    platformConfigs: [config("basic", false), config("sso", true)],
    sections: [
      {
        kind: "PluginSection",
        metadata: { name: "databases" },
        spec: {
          label: "Databases",
          slot: "managementDatabases",
          appliesTo: ["CamundaManagementClusterClaim"],
        },
      },
      {
        kind: "PluginSection",
        metadata: { name: "access" },
        spec: {
          label: "Access",
          slot: "externalAccess",
          appliesTo: ["CamundaManagementClusterClaim"],
        },
      },
    ] as unknown as Resource<"PluginSection">[],
    registrations: [
      {
        kind: "PluginRegistration",
        metadata: { name: "postgres" },
        spec: {
          deletionPolicies: ["Delete", "Orphan"],
          parameters: {
            type: "object",
            properties: {
              storageSize: {
                type: "string",
                description: "The size of the volume of each database.",
              },
              storageClass: { type: "string" },
            },
          },
        },
      },
      {
        kind: "PluginRegistration",
        metadata: { name: "gateway" },
        spec: { deletionPolicies: ["Delete"] },
      },
    ] as unknown as Resource<"PluginRegistration">[],
    requests: [],
    readErrors: [],
  },
};

const namespaces = [
  ["platform", "platform"],
  ["team-orders", "production"],
  ["team-qa", "staging"],
].map(
  ([name, environment]) =>
    ({
      kind: "Namespace",
      metadata: { name, labels: { [ENVIRONMENT_LABEL]: environment } },
    }) as unknown as Resource<"Namespace">,
);
const environments = [
  ["platform", "basic"],
  ["production", undefined],
  ["staging", undefined],
].map(
  ([name, ref]) =>
    ({
      kind: "Environment",
      metadata: { name },
      spec: ref ? { platformConfigRef: ref } : {},
    }) as unknown as Resource<"Environment">,
);

// The panels follow the Secrets on /api/watch, which a static Storybook
// does not serve: the stories read them from SECRETS.
class IdleEventSource {
  static readonly CLOSED = 2;
  readyState = 0;
  addEventListener() {}
  close() {
    this.readyState = IdleEventSource.CLOSED;
  }
}

type Args = {
  claim: PlaneClaim;
  plane?: Resource<"CamundaManagementCluster">;
  requested?: Parameters<typeof PlaneClaimEditor>[0]["requested"];
};

function Editor({ requested }: Args) {
  return <PlaneClaimEditor requested={requested} />;
}

const meta = {
  title: "planes/claim/Claim editor",
  component: Editor,
  args: { claim: keycloakClaim, plane: planeOf() },
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
        <SecretRefsReaderContext.Provider value={SECRETS}>
          <PlaneClaimEditProvider
            key={JSON.stringify(args)}
            data={data}
            plane={args.plane}
            claim={args.claim}
            namespaces={namespaces}
            environments={environments}
            pageUnread={[]}
            writeMode="direct"
            actions={actions}
          >
            <Story />
          </PlaneClaimEditProvider>
        </SecretRefsReaderContext.Provider>
      </div>
    ),
  ],
} satisfies Meta<typeof Editor>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ReadViewWithEditButtons: Story = {
  name: "Read view with Edit buttons",
};

export const OidcWithTheFixedClaimPair: Story = {
  name: "OIDC with the fixed claim pair",
  args: {
    claim: oidcClaim,
    plane: planeOf({ [IDENTITY_INITIAL_CLAIM_ANNOTATION]: "sub=4f1c" }),
    requested: "identity",
  },
};

// The review dialog of a move shows in the page scenario plane-claim-review.
export const MoveToOidc: Story = {
  name: "Move to OIDC",
  args: { requested: "identity" },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(
      canvas.getByRole("radio", { name: /OIDC from the platform config/ }),
    );
    await userEvent.type(canvas.getByLabelText("Claim name"), "sub");
    await userEvent.type(canvas.getByLabelText("Claim value"), "4f1c");
    await expect(canvas.getByRole("button", { name: "Review" })).toBeEnabled();
  },
};

export const OidcOffForBasicAuthentication: Story = {
  name: "OIDC off for basic authentication",
  args: {
    claim: {
      ...keycloakClaim,
      spec: { ...keycloakClaim.spec, platformConfigRef: "basic" },
    } as PlaneClaim,
    requested: "identity",
  },
};

export const ClustersThatAttachAndDetach: Story = {
  name: "Clusters that attach and detach",
  args: { requested: "environments" },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("checkbox", { name: "staging" }));
    await userEvent.click(canvas.getByRole("checkbox", { name: "production" }));
    await expect(
      canvas.getByRole("list", { name: "Clusters that attach or detach" }),
    ).toBeVisible();
  },
};

export const ConsoleAndWebModelerOff: Story = {
  name: "Console and Web Modeler off",
  args: { requested: "components" },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(
      canvas.getByRole("checkbox", { name: "Run Console" }),
    );
    await userEvent.click(
      canvas.getByRole("checkbox", { name: "Run Web Modeler" }),
    );
    await expect(canvas.getByText(/Console stops/)).toBeVisible();
  },
};

export const AVersionErrorAndAWarning: Story = {
  name: "A version error and a warning",
  args: { requested: "versions" },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const keycloak = canvas.getByLabelText("Keycloak version");
    await userEvent.clear(keycloak);
    await userEvent.type(keycloak, "27.0.0");
    const identity = canvas.getByLabelText("Management Identity version");
    await userEvent.clear(identity);
    await userEvent.type(identity, "8.9.10");
    await expect(canvas.getByText(/moves down/)).toBeVisible();
  },
};

export const ADatabaseWithAnEditableParameter: Story = {
  name: "A database with an editable parameter",
  args: { requested: "databases" },
};

export const BasicsWithThePlatformConfig: Story = {
  name: "Basics with the platform config",
  args: { requested: "basics" },
};

export const DeletionPolicy: Story = {
  name: "Deletion policy",
  args: { requested: "deletion" },
};

export const ShowYaml: Story = {
  name: "Show YAML",
  play: async ({ canvasElement }) => {
    await userEvent.click(
      within(canvasElement).getByRole("button", { name: "Show YAML" }),
    );
  },
};
