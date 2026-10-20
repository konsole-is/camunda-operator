import type { Story } from "@/gallery/registry";
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
import { NotAttachedChip } from "./not-attached-chip";
import { type PlaneSources, planesOf } from "./plane";
import { PlaneList } from "./plane-list";
import { PlanesTable } from "./planes-table";

function cluster(
  name: string,
  spec: Record<string, unknown>,
  status: Record<string, unknown> | undefined,
  managed = true,
) {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: {
      namespace: "platform",
      name,
      labels: managed ? { [CLAIM_LABEL]: name } : undefined,
    },
    spec,
    status,
  } as never;
}

function claim(
  name: string,
  environments: string[],
  management: Record<string, unknown>,
) {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { namespace: "platform", name },
    spec: { environments, management },
  } as never;
}

const notAttached = [
  {
    namespace: "team-billing",
    name: "billing",
    reason: "ClaimedElsewhere",
    message: "The plane platform/sandbox already serves this cluster",
  },
  {
    namespace: "team-search",
    name: "search",
    reason: "NotReady",
    message: "The orchestration cluster is not ready",
  },
];

const sources: PlaneSources = {
  clusters: [
    cluster(
      "production",
      { identityProvider: { keycloak: {} }, console: {}, webModeler: {} },
      {
        conditions: [{ type: "Ready", status: "True", reason: "Healthy" }],
        clusters: [
          { namespace: "team-orders", name: "orders", attached: true },
          { namespace: "team-payments", name: "payments", attached: true },
          ...notAttached.map((c) => ({ ...c, attached: false })),
        ],
        optimize: [
          {
            namespace: "team-orders",
            name: "optimize",
            externalUrl: "https://optimize.example.com",
          },
        ],
        managementAuthConfig: "production",
      },
    ),
    cluster(
      "staging",
      { identityProvider: { keycloak: {} }, console: {} },
      {
        conditions: [
          {
            type: "Ready",
            status: "False",
            reason: "UnsupportedVersion",
            message: "Keycloak 27.0.0 is not a supported version",
          },
        ],
        managementAuthConfig: "staging",
      },
    ),
    cluster(
      "partners",
      { identityProvider: { oidc: {} }, console: {} },
      {
        conditions: [{ type: "Ready", status: "True", reason: "Healthy" }],
        clusters: [
          { namespace: "team-partners", name: "partners", attached: true },
        ],
        managementAuthConfig: "partners",
      },
    ),
    cluster(
      "legacy",
      {
        identityProvider: {
          externalKeycloak: {
            url: "https://keycloak.example.com",
            realm: "camunda",
            adminCredentialsSecretRef: { name: "keycloak-admin" },
          },
        },
        namespaceSelector: { matchLabels: { team: "payments" } },
      },
      { conditions: [{ type: "Ready", status: "True", reason: "Healthy" }] },
      false,
    ),
  ],
  claims: [
    claim("production", ["production"], {
      identityProvider: { keycloak: {} },
    }),
    claim("staging", ["staging"], { identityProvider: { keycloak: {} } }),
    claim("partners", ["production", "staging"], {
      identityProvider: { oidc: {} },
    }),
    claim("sandbox", ["sandbox"], {
      identityProvider: { keycloak: {} },
      webModeler: {},
    }),
  ],
  namespaces: [
    {
      apiVersion: "v1",
      kind: "Namespace",
      metadata: {
        name: "team-payments",
        labels: { team: "payments", [ENVIRONMENT_LABEL]: "production" },
      },
    } as never,
  ],
};

const planes = planesOf(sources);

export const planesStories: Story[] = [
  {
    component: "PlaneList",
    name: "Mixed planes",
    render: () => <PlaneList planes={planes} />,
  },
  {
    component: "PlaneList",
    name: "No planes",
    render: () => <PlaneList planes={[]} />,
  },
  {
    component: "PlanesTable",
    name: "Planes of production",
    render: () => <PlanesTable planes={planes} environment="production" />,
  },
  {
    component: "NotAttachedChip",
    name: "Open",
    render: () => <NotAttachedChip clusters={notAttached} defaultOpen />,
  },
];
