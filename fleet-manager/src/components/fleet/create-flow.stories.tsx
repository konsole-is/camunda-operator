import type { V1Namespace } from "@kubernetes/client-node";
import { BasicsStory } from "@/components/create/basics-story";
import { EnvironmentCards } from "@/components/create/environment-cards";
import { PresetCards } from "@/components/create/preset-cards";
import type { Story } from "@/gallery/registry";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { NamespacePicker } from "./namespace-picker";
import { WizardStepper } from "./wizard-stepper";

const steps = (errors?: string[]) => [
  { id: "namespace", label: "Namespace" },
  { id: "storage", label: "Storage and access", errors },
  { id: "settings", label: "Settings" },
  { id: "review", label: "Review" },
];
const environment = {
  name: "prod",
  displayName: "Production",
  namespaceCreation: {
    allowed: true,
    labels: { "cost-center": "4711" },
    requiredLabelKeys: ["team"],
  },
};
const namespaces = [
  { name: "team-payments", environment: "prod" },
  { name: "team-orders", environment: "prod" },
];

const production: Environment = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "prod" },
  spec: {
    displayName: "Production",
    color: "#2E7D32",
    namespaceCreation: environment.namespaceCreation,
    presets: { camundaCluster: ["small", "medium", "large"] },
  },
};
const development: Environment = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "dev" },
  spec: {
    displayName: "Development",
    color: "#1565C0",
    presets: { camundaCluster: ["small"] },
  },
};
const legacy: Environment = {
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
const counts = {
  prod: { namespaces: 2, clusters: 3 },
  dev: { namespaces: 1, clusters: 0 },
  legacy: { namespaces: 0, clusters: 0 },
};
const clusterNamespaces: V1Namespace[] = [
  {
    metadata: {
      name: "team-payments",
      labels: { [ENVIRONMENT_LABEL]: "prod" },
    },
  },
  {
    metadata: { name: "team-orders", labels: { [ENVIRONMENT_LABEL]: "prod" } },
  },
  { metadata: { name: "sandbox", labels: { [ENVIRONMENT_LABEL]: "dev" } } },
];
const preset = (
  name: string,
  replicas: number,
  partitions: number,
  cpu: string,
  memory: string,
): CamundaClusterPreset => ({
  apiVersion: "core.camunda.io/v1",
  kind: "CamundaClusterPreset",
  metadata: { name },
  spec: {
    cluster: {
      zeebe: { replicas, partitions, resources: { requests: { cpu, memory } } },
    },
  },
});
const presets = [
  preset("small", 1, 1, "500m", "1Gi"),
  preset("medium", 3, 3, "1", "4Gi"),
  preset("large", 5, 12, "4", "16Gi"),
];
const usage = {
  small: ["team-orders/search"],
  medium: ["team-orders/orders", "team-payments/billing"],
};

export const createFlowStories: Story[] = [
  {
    component: "WizardStepper",
    name: "Step without errors",
    render: () => (
      <WizardStepper steps={steps()} current="storage">
        <p className="text-sm text-text-2">
          The section cards of the step go here.
        </p>
      </WizardStepper>
    ),
  },
  {
    component: "WizardStepper",
    name: "Step with errors",
    render: () => (
      <WizardStepper
        steps={steps(["Choose a secondary storage option"])}
        current="storage"
      >
        <p className="text-sm text-text-2">
          The section cards of the step go here.
        </p>
      </WizardStepper>
    ),
  },
  {
    component: "WizardStepper",
    name: "Last step",
    render: () => (
      <WizardStepper steps={steps()} current="review">
        <p className="text-sm text-text-2">The review goes here.</p>
      </WizardStepper>
    ),
  },
  {
    component: "NamespacePicker",
    name: "Existing namespace",
    render: () => (
      <NamespacePicker
        namespaces={namespaces}
        environment={environment}
        defaultValue={{ kind: "existing", name: "team-orders" }}
      />
    ),
  },
  {
    component: "NamespacePicker",
    name: "New namespace",
    render: () => (
      <NamespacePicker
        namespaces={namespaces}
        environment={environment}
        defaultValue={{
          kind: "new",
          name: "team-search",
          labels: { team: "search" },
        }}
      />
    ),
  },
  {
    component: "NamespacePicker",
    name: "Creation not allowed",
    render: () => (
      <NamespacePicker
        namespaces={namespaces}
        environment={{ name: "prod", displayName: "Production" }}
      />
    ),
  },
  {
    component: "EnvironmentCards",
    name: "Two environments, one picked",
    render: () => (
      <EnvironmentCards
        environments={[production, development]}
        counts={counts}
        value="prod"
      />
    ),
  },
  {
    component: "EnvironmentCards",
    name: "Not ready",
    render: () => (
      <EnvironmentCards environments={[production, legacy]} counts={counts} />
    ),
  },
  {
    component: "EnvironmentCards",
    name: "Several picked",
    render: () => (
      <EnvironmentCards
        multiple
        environments={[production, development, legacy]}
        counts={counts}
        value={["dev", "prod"]}
      />
    ),
  },
  {
    component: "PresetCards",
    name: "Three presets",
    render: () => (
      <PresetCards
        presets={presets}
        usage={new Map(Object.entries(usage))}
        value="medium"
      />
    ),
  },
  {
    component: "ClusterStepBasics",
    name: "Production picked",
    render: () => (
      <BasicsStory
        environments={[production, development, legacy]}
        namespaces={clusterNamespaces}
        presets={presets}
        usage={usage}
        initial={{
          environment: "prod",
          namespace: { kind: "existing", name: "team-orders" },
          claim: {
            metadata: { name: "search" },
            spec: { presetRef: "medium" },
          },
        }}
        hosts={[
          {
            parameter: "hostnamePattern",
            host: "search.team-orders.example.com",
          },
          {
            parameter: "grpcHostnamePattern",
            host: "grpc-search.team-orders.example.com",
          },
        ]}
      />
    ),
  },
  {
    component: "ClusterStepBasics",
    name: "Taken host",
    render: () => (
      <BasicsStory
        environments={[production, development]}
        namespaces={clusterNamespaces}
        presets={presets}
        usage={usage}
        initial={{
          environment: "prod",
          namespace: { kind: "existing", name: "team-orders" },
          claim: {
            metadata: { name: "billing" },
            spec: { presetRef: "small" },
          },
        }}
        hosts={[
          {
            parameter: "hostnamePattern",
            host: "billing.team-orders.example.com",
          },
        ]}
        conflicts={[
          "The host billing.team-orders.example.com is taken by team-payments/billing. Choose another name.",
        ]}
      />
    ),
  },
  {
    component: "ClusterStepBasics",
    name: "No environment",
    render: () => (
      <BasicsStory
        environments={[]}
        namespaces={clusterNamespaces}
        presets={presets}
        usage={usage}
        initial={{ claim: { metadata: { name: "" }, spec: {} } }}
      />
    ),
  },
];
