import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { fn } from "storybook/test";
import { useCreateFlow } from "@/components/create/use-create-flow";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import { planesOf } from "@/components/planes/plane";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { PluginSection } from "@/generated/crds/pluginsection";
import {
  CLAIM_LABEL,
  ENVIRONMENT_LABEL,
  MANAGEMENT_CLUSTER_ANNOTATION,
} from "@/lib/labels";
import { EMPTY_ADMIN } from "@/lib/planes/components";
import { EMPTY_PLANE_DRAFT, type PlaneDraft } from "@/lib/planes/draft";
import type { ValidationError } from "@/lib/schema";
import {
  type PlaneCreateData,
  type PlaneModels,
  type PlaneReview,
  PlaneStepBody,
  planeSteps,
} from "./plane-create-flow";
import { PlaneProgressView } from "./plane-progress";

const environment = (
  name: string,
  displayName: string,
  color: string,
  platformConfigRef?: string,
) => ({
  apiVersion: "fleet.camunda.io/v1alpha1" as const,
  kind: "Environment" as const,
  metadata: { name },
  spec: {
    displayName,
    color,
    ...(platformConfigRef && { platformConfigRef }),
    releases: { allowed: ["camunda-8-9-23"], default: "camunda-8-9-23" },
    namespaceCreation: { allowed: true, requiredLabelKeys: ["team"] },
    sections: {
      "management-database": {
        required: true,
        allowed: ["existing-database-configs", "management-database-server"],
        default: "management-database-server",
      },
      access: { allowed: ["gateway"] },
    },
    presets: { databaseServer: ["small", "large"] },
    parameters: {
      "management-database-server": { preset: { claimEditable: true } },
      "existing-database-configs": {
        keycloak: { claimEditable: true },
        identity: { claimEditable: true },
        webModeler: { claimEditable: true },
      },
    },
  },
});

const pluginSection = (
  name: string,
  label: string,
  description: string,
  slot: string,
  order: number,
  emptyOptionLabel?: string,
) => ({
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginSection",
  metadata: { name },
  spec: {
    appliesTo: ["CamundaManagementClusterClaim"],
    label,
    description,
    selection: "One",
    slot,
    order,
    ...(emptyOptionLabel && { emptyOptionLabel }),
  },
});

const registration = (
  name: string,
  label: string,
  description: string,
  spec: object,
) => ({
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginRegistration",
  metadata: { name },
  spec: {
    mode: "Optional",
    label,
    description,
    deletionPolicies: ["Delete"],
    serviceAccount: { name, namespace: "plugins" },
    ...spec,
  },
  status: {
    conditions: ["Accepted", "Ready"].map((type) => ({
      type,
      status: "True",
      reason: "Ready",
      message: "",
      lastTransitionTime: "2026-11-10T00:00:00Z",
    })),
  },
});

const builtin = { name: "camunda-operator", version: "0.0.0" };
const databaseApp = (title: string) => ({
  type: "string",
  minLength: 1,
  title,
});

const databaseServerPreset = (
  name: string,
  instances: number,
  storageSize: string,
) => ({
  apiVersion: "core.camunda.io/v1",
  kind: "DatabaseServerPreset",
  metadata: { name },
  spec: { server: { instances, storageSize } },
});

const databaseConfig = (name: string) => ({
  apiVersion: "core.camunda.io/v1",
  kind: "DatabaseConfig",
  metadata: { namespace: "platform", name },
  spec: {},
});

const namespace = (name: string, env?: string) => ({
  apiVersion: "v1" as const,
  kind: "Namespace" as const,
  metadata: { name, ...(env && { labels: { [ENVIRONMENT_LABEL]: env } }) },
});

const cluster = (
  ns: string,
  name: string,
  opts: { holder?: string; gateway?: boolean; config?: string } = {},
) => ({
  apiVersion: "core.camunda.io/v1",
  kind: "CamundaCluster",
  metadata: {
    namespace: ns,
    name,
    ...(opts.holder && {
      annotations: { [MANAGEMENT_CLUSTER_ANNOTATION]: opts.holder },
    }),
  },
  spec:
    opts.config === ""
      ? {}
      : { platformConfigRef: opts.config ?? "basic-auth" },
  ...(opts.gateway !== false && {
    status: {
      gateway: {
        grpcEndpoint: `${name}-gateway.${ns}.svc:26500`,
        restEndpoint: `http://${name}-gateway.${ns}.svc:8080`,
      },
    },
  }),
});

const data = {
  environments: [
    environment("production", "Production", "#2E7D32", "basic-auth"),
    environment("staging", "Staging", "#F9A825", "basic-auth"),
  ],
  namespaces: [
    namespace("platform", "production"),
    namespace("team-orders", "production"),
    namespace("team-billing", "production"),
    namespace("team-search", "staging"),
    namespace("sandbox"),
  ],
  clusters: [
    cluster("team-orders", "orders"),
    cluster("team-billing", "billing", { holder: "ops/plane-legacy" }),
    cluster("team-orders", "orders-next", { gateway: false }),
    cluster("team-billing", "billing-sso", { config: "sso" }),
    cluster("team-search", "search", { config: "" }),
  ],
  platformConfigs: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaPlatformConfig",
      metadata: { name: "basic-auth" },
      spec: {},
    },
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaPlatformConfig",
      metadata: { name: "sso" },
      spec: {
        auth: {
          method: "oidc",
          oidc: { issuerUrl: "https://login.example.com/realms/camunda" },
        },
      },
    },
  ],
  releases: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaRelease",
      metadata: { name: "camunda-8-9-23" },
      spec: { version: "8.9.23" },
    },
  ],
  images: {
    schemaVersion: 1,
    minors: {
      "8.9": {
        identity: { repository: "camunda/identity", version: "8.9.10" },
        console: { repository: "camunda/console", version: "8.9.122" },
        "web-modeler-restapi": {
          repository: "camunda/web-modeler-restapi",
          version: "8.9.11",
        },
        keycloak: {
          repository: "camunda/keycloak",
          tagPrefix: "quay-optimized-",
          version: "26.6.4",
        },
      },
    },
  },
  sections: [
    pluginSection(
      "management-database",
      "Databases",
      "The databases of Keycloak, Management Identity, and Web Modeler.",
      "managementDatabases",
      80,
    ),
    pluginSection(
      "access",
      "Access",
      "How users and clients reach the cluster or the management plane from outside.",
      "externalAccess",
      60,
      "No access from outside",
    ),
  ],
  registrations: [
    registration(
      "management-database-server",
      "New PostgreSQL server",
      "Runs a PostgreSQL server with one database for each component of the management plane.",
      {
        sections: ["management-database"],
        controller: builtin,
        parameters: {
          type: "object",
          required: ["preset"],
          properties: {
            preset: { type: "string", minLength: 1, title: "Preset" },
            databases: { type: "array", items: { type: "string" } },
          },
        },
      },
    ),
    registration(
      "existing-database-configs",
      "Existing databases",
      "Uses DatabaseConfigs that are already in the namespace, one for each component.",
      {
        sections: ["management-database"],
        controller: builtin,
        parameters: {
          type: "object",
          properties: {
            keycloak: databaseApp("Keycloak"),
            identity: databaseApp("Management Identity"),
            webModeler: databaseApp("Web Modeler"),
          },
        },
      },
    ),
    registration(
      "gateway",
      "Gateway",
      "Routes the plane through the shared Gateway.",
      {
        sections: ["access"],
        parameterValues: {
          managementHostnamePattern: "{name}.mgmt.example.com",
        },
      },
    ),
  ],
  accessRequests: [],
  authConfigs: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "ManagementAuthConfig",
      metadata: { name: "plane-legacy" },
    },
  ],
  databaseServerPresets: [
    databaseServerPreset("small", 1, "10Gi"),
    databaseServerPreset("large", 3, "100Gi"),
  ],
  databaseConfigs: [
    databaseConfig("keycloak-db"),
    databaseConfig("identity-db"),
    databaseConfig("modeler-db"),
  ],
  planes: {
    clusters: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaManagementCluster",
        metadata: {
          namespace: "ops",
          name: "plane-legacy",
          labels: { [CLAIM_LABEL]: "plane-legacy" },
        },
        spec: { identityProvider: { keycloak: {} } },
      },
    ],
    claims: [
      {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "CamundaManagementClusterClaim",
        metadata: { namespace: "ops", name: "plane-legacy" },
        spec: {
          environments: ["staging"],
          management: { identityProvider: { keycloak: {} } },
        },
      },
    ],
    namespaces: [],
    environments: [],
  },
} as unknown as PlaneCreateData;

const basics: PlaneDraft = {
  ...EMPTY_PLANE_DRAFT,
  environment: "production",
  name: "plane-a",
  namespace: { kind: "existing", name: "platform" },
  mode: "keycloak",
};

const components: PlaneDraft = {
  ...basics,
  admin: { ...EMPTY_ADMIN, username: "admin", email: "admin@example.com" },
  console: true,
};

const reviewStorage: PlaneDraft["storage"] = {
  "management-database": [
    {
      section: "management-database",
      registration: "management-database-server",
      parameters: { preset: "small" },
    },
  ],
  access: [{ section: "access", registration: "gateway" }],
};

const progressClaim = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaManagementClusterClaim",
  metadata: { namespace: "platform", name: "plane-a", generation: 1 },
  spec: {
    environments: ["production"],
    management: { identityProvider: { keycloak: {} }, identity: { admin: {} } },
    versions: { identity: "8.9.10" },
    plugins: reviewStorage["management-database"],
  },
  status: {
    environment: "production",
    requests: [
      {
        section: "management-database",
        registration: "management-database-server",
        name: "plane-a-management-database",
        outputsReady: "True",
        ready: "True",
      },
    ],
    conditions: [
      {
        type: "Ready",
        status: "False",
        reason: "Creating",
        message:
          "The CamundaManagementCluster plane-a has not reported Ready yet.",
        lastTransitionTime: "2026-11-10T00:00:00Z",
      },
      {
        type: "ManagementClusterReady",
        status: "False",
        reason: "Creating",
        message:
          "The CamundaManagementCluster plane-a has not reported Ready yet.",
        lastTransitionTime: "2026-11-10T00:00:00Z",
      },
    ],
  },
} as unknown as CamundaManagementClusterClaim;

// PlaneCreateFlow watches its lists live, so the stories show its steps
// with this harness over fixture data. The harness owns the draft, which
// starts at initial, and opens at step.
function PlaneCreateHarness({
  data,
  initial,
  step,
  ...review
}: PlaneReview & {
  data: PlaneCreateData;
  initial: PlaneDraft;
  step: string;
}) {
  const models: PlaneModels = {
    environments: data.environments as PlaneModels["environments"],
    namespaces: data.namespaces as PlaneModels["namespaces"],
    clusters: data.clusters as PlaneModels["clusters"],
    platformConfigs: data.platformConfigs as PlaneModels["platformConfigs"],
    releases: data.releases as PlaneModels["releases"],
    sections: data.sections as PlaneModels["sections"],
    registrations: data.registrations as PlaneModels["registrations"],
    accessRequests: data.accessRequests as PlaneModels["accessRequests"],
    authConfigs: data.authConfigs.map((c) => c.metadata?.name ?? ""),
    objects: {
      DatabaseServerPreset:
        data.databaseServerPresets as PlaneModels["objects"]["DatabaseServerPreset"],
      DatabaseConfig:
        data.databaseConfigs as PlaneModels["objects"]["DatabaseConfig"],
    },
    planes: planesOf(data.planes),
    sources: data.planes,
    images: data.images,
  };
  const [selectorErrors, setSelectorErrors] = useState<ValidationError[]>([]);
  const [sectionErrors, setSectionErrors] = useState<
    Record<string, ValidationError[]>
  >({});
  const steps = planeSteps(models, selectorErrors, [], sectionErrors);
  const flow = useCreateFlow({ steps, initial });
  const reached = steps.slice(0, steps.findIndex((s) => s.id === step) + 1);
  return (
    <WizardStepper
      steps={steps.map((s) => ({
        id: s.id,
        label: s.label,
        errors: reached.includes(s) ? s.check?.(flow.draft) : undefined,
      }))}
      current={step}
    >
      <PlaneStepBody
        current={step}
        draft={flow.draft}
        set={flow.set}
        models={models}
        secrets={[{ name: "keycloak-admin", keys: ["password", "username"] }]}
        onSelectorErrors={setSelectorErrors}
        onSectionErrors={(section, errors) =>
          setSectionErrors((current) => ({ ...current, [section]: errors }))
        }
        review={review}
      />
    </WizardStepper>
  );
}

const meta = {
  title: "planes/create/PlaneCreateFlow",
  component: PlaneCreateHarness,
  args: {
    data,
    initial: basics,
    step: "basics",
    mode: "direct",
    preview: fn(async () => ({ objects: [], errors: [] })),
    create: fn(async () => ({ ok: true as const, mode: "direct" as const })),
    onWritten: fn(),
  },
  // useCreateFlow reads initial only when it mounts, so a new initial from
  // the Controls panel remounts the harness.
  render: (args) => (
    <PlaneCreateHarness key={JSON.stringify(args.initial)} {...args} />
  ),
} satisfies Meta<typeof PlaneCreateHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Basics: Story = { name: "Basics" };

export const BasicsWithNoEnvironment: Story = {
  name: "Basics with no Environment",
  args: { data: { ...data, environments: [] }, initial: EMPTY_PLANE_DRAFT },
};

export const ExternalKeycloak: Story = {
  name: "External Keycloak",
  args: {
    initial: {
      ...basics,
      mode: "externalKeycloak",
      externalKeycloak: {
        url: "https://keycloak.example.com/auth",
        realm: "camunda-platform",
        adminSecret: "keycloak-admin",
      },
    },
  },
};

export const EnvironmentsServedWithEveryResult: Story = {
  name: "Environments served with every result",
  args: {
    step: "served",
    initial: {
      ...basics,
      platformConfig: "sso",
      mode: "oidc",
      served: ["production", "staging"],
    },
  },
};

export const KeycloakModePreview: Story = {
  name: "Keycloak mode preview",
  args: { step: "served" },
};

export const ComponentsAndVersionsInKeycloakMode: Story = {
  name: "Components and versions in Keycloak mode",
  args: { step: "components", initial: { ...components, webModeler: true } },
};

export const ComponentsAndVersionsInOidcMode: Story = {
  name: "Components and versions in OIDC mode",
  args: {
    step: "components",
    initial: {
      ...components,
      platformConfig: "sso",
      mode: "oidc",
      admin: { ...EMPTY_ADMIN, claimName: "sub" },
    },
  },
};

export const KeycloakVersionWithAWarning: Story = {
  name: "Keycloak version with a warning",
  args: {
    step: "components",
    initial: { ...components, versions: { keycloak: "26.7.0" } },
  },
};

export const StorageAndAccessWithNoAccessFromOutside: Story = {
  name: "Storage and access with no access from outside",
  args: {
    step: "storage",
    initial: {
      ...components,
      urls: { keycloak: "https://management.example.com/auth" },
    },
  },
};

export const StorageAndAccessWithAnAccessOption: Story = {
  name: "Storage and access with an access option",
  args: {
    step: "storage",
    initial: {
      ...components,
      storage: {
        "management-database": [
          {
            section: "management-database",
            registration: "existing-database-configs",
            parameters: { keycloak: "keycloak-db", identity: "identity-db" },
          },
        ],
        access: [{ section: "access", registration: "gateway" }],
      },
    },
  },
};

export const StorageAndAccessWithATakenHost: Story = {
  name: "Storage and access with a taken host",
  args: {
    data: {
      ...data,
      accessRequests: [
        {
          apiVersion: "plugins.camunda.io/v1alpha1",
          kind: "PluginRequest",
          metadata: { namespace: "ops", name: "plane-b-access" },
          spec: {
            claimRef: {
              kind: "CamundaManagementClusterClaim",
              name: "plane-b",
            },
            section: "access",
            registration: "gateway",
          },
          status: { outputs: { managementHost: "plane-a.mgmt.example.com" } },
        },
      ] as unknown as PlaneCreateData["accessRequests"],
    },
    step: "storage",
    initial: {
      ...components,
      storage: { access: [{ section: "access", registration: "gateway" }] },
    },
  },
};

export const StorageAndAccessWithAPickOfASectionThatIsGone: Story = {
  name: "Storage and access with a pick of a section that is gone",
  args: {
    step: "storage",
    initial: {
      ...components,
      storage: {
        "dns-zone": [{ section: "dns-zone", registration: "route53" }],
      },
    },
  },
};

export const StorageAndAccessWithADefaultThatKeepsTheResources: Story = {
  name: "Storage and access with a default that keeps the resources",
  args: {
    data: {
      ...data,
      environments: data.environments.map((e) =>
        e.metadata.name === "production"
          ? { ...e, spec: { ...e.spec, deletionPolicy: "Orphan" as const } }
          : e,
      ),
    },
    step: "storage",
    initial: { ...components, storage: reviewStorage },
  },
};

export const ReviewInDirectMode: Story = {
  name: "Review in direct mode",
  args: { step: "review", initial: { ...components, storage: reviewStorage } },
};

export const ReviewInGitMode: Story = {
  name: "Review in Git mode",
  args: {
    step: "review",
    mode: "git",
    initial: {
      ...components,
      namespace: {
        kind: "new",
        name: "team-plane",
        labels: { [ENVIRONMENT_LABEL]: "production", team: "platform" },
      },
      storage: reviewStorage,
    },
  },
};

export const ProgressWithEveryRowPending: Story = {
  name: "Progress with every row pending",
  parameters: { controls: { disable: true } },
  render: () => (
    <PlaneProgressView
      name="plane-a"
      sections={data.sections as unknown as PluginSection[]}
    />
  ),
};

export const ProgressWithTheDatabasesReady: Story = {
  name: "Progress with the databases ready",
  parameters: { controls: { disable: true } },
  render: () => (
    <PlaneProgressView
      name="plane-a"
      sections={data.sections as unknown as PluginSection[]}
      claim={progressClaim}
    />
  ),
};
