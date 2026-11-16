import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { ComponentCards } from "@/components/cluster/component-card";
import type { Resource } from "@/lib/k8s/kinds";
import { COMPONENT_LABEL } from "@/lib/labels";
import type { PodUsage } from "@/lib/observe/types";
import {
  type PlaneCardId,
  planeCardHref,
  planeCards,
} from "./plane-cards";

const condition = (type: string, status = "True", reason = "Healthy") => ({
  type,
  status,
  reason,
  message: status === "True" ? "" : `${type} reports ${reason}`,
  lastTransitionTime: "2026-11-15T12:00:00Z",
});

function plane(
  identityProvider: Record<string, unknown>,
  conditions: ReturnType<typeof condition>[],
  apps = true,
): Resource<"CamundaManagementCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: { namespace: "platform", name: "plane-a" },
    spec: {
      platformConfigRef: "oidc-platform",
      identityProvider,
      identity: {},
      ...(apps && { console: {}, webModeler: {} }),
    },
    status: { conditions },
  } as unknown as Resource<"CamundaManagementCluster">;
}

function pod(name: string, component: string, ready = true): Resource<"Pod"> {
  return {
    kind: "Pod",
    metadata: { name, labels: { [COMPONENT_LABEL]: component } },
    status: {
      phase: "Running",
      conditions: [{ type: "Ready", status: ready ? "True" : "False" }],
      containerStatuses: [{ name: "app", ready, restartCount: 0 }],
    },
  } as unknown as Resource<"Pod">;
}

const usage = (pod: string, share: number): PodUsage => ({
  pod,
  cpu: { usedCores: share * 0.5, limitCores: 1 },
  memory: { usedBytes: share * 1024 ** 3, limitBytes: 1024 ** 3 },
});

const config = {
  metadata: { name: "oidc-platform" },
  spec: {
    auth: {
      method: "oidc",
      oidc: { issuerUrl: "https://login.example.com/realms/camunda" },
    },
  },
} as unknown as Resource<"CamundaPlatformConfig">;

const pods = [
  pod("plane-a-management-identity-0", "management-identity"),
  pod("plane-a-console-0", "console"),
  pod("plane-a-web-modeler-restapi-0", "web-modeler-restapi"),
  pod("plane-a-web-modeler-websockets-0", "web-modeler-websockets"),
];

const meta = {
  title: "planes/PlaneCards",
  component: ComponentCards<PlaneCardId>,
  args: { hrefOf: planeCardHref, onOpen: fn(), cards: [] },
} satisfies Meta<typeof ComponentCards<PlaneCardId>>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OidcReady: Story = {
  name: "OIDC ready",
  args: {
    cards: planeCards({
      plane: plane({ oidc: {} }, [
        condition("IdentityReady"),
        condition("ConsoleReady"),
        condition("WebModelerReady"),
      ]),
      pods,
      config,
      usage: pods.map((p, i) => usage(p.metadata?.name ?? "", 0.2 + i * 0.1)),
    }),
  },
};

export const KeycloakCreating: Story = {
  name: "Keycloak creating",
  args: {
    cards: planeCards({
      plane: plane(
        { keycloak: {} },
        [
          condition("KeycloakReady", "False", "Creating"),
          condition("IdentityReady", "False", "PrerequisiteNotMet"),
        ],
        false,
      ),
      pods: [
        pod("plane-a-keycloak-0", "keycloak", false),
        pod("plane-a-management-identity-0", "management-identity", false),
      ],
      usage: [],
    }),
  },
};

export const IdentityDown: Story = {
  name: "Identity down",
  args: {
    cards: planeCards({
      plane: plane({ oidc: {} }, [
        condition("IdentityReady", "False", "Down"),
        condition("ConsoleReady"),
        condition("WebModelerReady"),
      ]),
      pods: [
        pod("plane-a-management-identity-0", "management-identity", false),
        ...pods.slice(1),
      ],
      config,
      usage: [],
    }),
  },
};

export const NotRead: Story = {
  name: "Plane and pods not read",
  args: {
    cards: planeCards({
      plane: plane({ oidc: {} }, [condition("IdentityReady")]),
      pods,
      config,
      usage: [],
      unread: ["CamundaManagementCluster", "Pod", "CamundaPlatformConfig"],
    }),
  },
};
