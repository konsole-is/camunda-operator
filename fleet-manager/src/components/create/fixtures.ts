import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import { DESCRIPTION_ANNOTATION } from "@/lib/labels";

export const production: Environment = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "prod" },
  spec: {
    displayName: "Production",
    color: "#2E7D32",
    namespaceCreation: {
      allowed: true,
      labels: { "cost-center": "4711" },
      requiredLabelKeys: ["team"],
    },
    presets: { camundaCluster: ["small", "medium", "large"] },
  },
};

export const development: Environment = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "dev" },
  spec: {
    displayName: "Development",
    color: "#1565C0",
    presets: { camundaCluster: ["small"] },
  },
};

export const legacy: Environment = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "legacy" },
  spec: { displayName: "Legacy" },
  status: {
    conditions: [
      {
        type: "Ready",
        status: "False",
        reason: "InvalidReference",
        message: 'CamundaRelease "8.9-old" does not exist',
        lastTransitionTime: "2026-10-30T00:00:00Z",
      },
    ],
  },
};

const preset = (
  name: string,
  replicas: number,
  partitions: number,
  cpu: string,
  memory: string,
  description?: string,
): CamundaClusterPreset => ({
  apiVersion: "core.camunda.io/v1",
  kind: "CamundaClusterPreset",
  metadata: {
    name,
    ...(description && {
      annotations: { [DESCRIPTION_ANNOTATION]: description },
    }),
  },
  spec: {
    cluster: {
      zeebe: { replicas, partitions, resources: { requests: { cpu, memory } } },
    },
  },
});

export const presets = [
  preset(
    "small",
    1,
    1,
    "500m",
    "1Gi",
    "For tests and demos. One broker, no replication.",
  ),
  preset(
    "medium",
    3,
    3,
    "1",
    "4Gi",
    "For production workloads with three brokers and replication.",
  ),
  preset("large", 5, 12, "4", "16Gi"),
];

export const presetUsage: Record<string, string[]> = {
  small: ["team-orders/search"],
  medium: ["team-orders/orders", "team-payments/billing"],
};
