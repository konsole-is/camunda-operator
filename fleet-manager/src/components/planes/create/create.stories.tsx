import type { Story } from "@/gallery/registry";
import {
  CLAIM_LABEL,
  ENVIRONMENT_LABEL,
  MANAGEMENT_CLUSTER_ANNOTATION,
} from "@/lib/labels";
import { EMPTY_PLANE_DRAFT, type PlaneDraft } from "@/lib/planes/draft";
import { PlaneCreateStory } from "./create-story";
import type { PlaneCreateData } from "./plane-create-flow";

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
    namespaceCreation: { allowed: true, requiredLabelKeys: ["team"] },
  },
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
  },
} as unknown as PlaneCreateData;

const basics: PlaneDraft = {
  ...EMPTY_PLANE_DRAFT,
  environment: "production",
  name: "plane-a",
  namespace: { kind: "existing", name: "platform" },
  mode: "keycloak",
};

/** planeCreateStories are the gallery stories of the plane create flow. */
export const planeCreateStories: Story[] = [
  {
    component: "PlaneCreateFlow",
    name: "Basics",
    render: () => <PlaneCreateStory data={data} initial={basics} />,
  },
  {
    component: "PlaneCreateFlow",
    name: "Basics with no Environment",
    render: () => (
      <PlaneCreateStory
        data={{ ...data, environments: [] }}
        initial={EMPTY_PLANE_DRAFT}
      />
    ),
  },
  {
    component: "PlaneCreateFlow",
    name: "External Keycloak",
    render: () => (
      <PlaneCreateStory
        data={data}
        initial={{
          ...basics,
          mode: "externalKeycloak",
          externalKeycloak: {
            url: "https://keycloak.example.com/auth",
            realm: "camunda-platform",
            adminSecret: "keycloak-admin",
          },
        }}
      />
    ),
  },
  {
    component: "PlaneCreateFlow",
    name: "Environments served with every result",
    render: () => (
      <PlaneCreateStory
        data={data}
        step="served"
        initial={{
          ...basics,
          platformConfig: "sso",
          mode: "oidc",
          served: ["production", "staging"],
        }}
      />
    ),
  },
  {
    component: "PlaneCreateFlow",
    name: "Keycloak mode preview",
    render: () => (
      <PlaneCreateStory data={data} step="served" initial={basics} />
    ),
  },
];
