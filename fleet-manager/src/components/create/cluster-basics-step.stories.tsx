import type { V1Namespace } from "@kubernetes/client-node";
import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ComponentProps } from "react";
import { fn } from "storybook/test";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import type { CamundaClusterPreset } from "@/generated/crds/camundaclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import type { RenderedHost } from "@/lib/claims/hosts";
import { claimNameErrors } from "@/lib/claims/names";
import {
  CLUSTER_STEPS,
  type ClusterDraft,
  basicsErrors,
  offeredPresets,
} from "@/lib/clusters/draft";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { environmentOf } from "@/lib/policy";
import { environmentCounts } from "@/lib/usage";
import { ClusterBasicsStep } from "./cluster-basics-step";
import {
  development,
  legacy,
  presetUsage,
  presets,
  production,
} from "./fixtures";
import { useCreateFlow } from "./use-create-flow";

const namespaces: V1Namespace[] = [
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

// ClusterBasicsStep holds no draft, so the harness holds the draft and the
// checks of step 1 as the create flow does. usage is by preset name.
function ClusterBasicsStepHarness({
  environments,
  namespaces,
  presets,
  usage = {},
  initial,
  hosts = [],
  conflicts = [],
  set,
}: {
  environments: Environment[];
  namespaces: V1Namespace[];
  presets: CamundaClusterPreset[];
  usage?: Record<string, string[]>;
  initial: ClusterDraft;
  hosts?: RenderedHost[];
  conflicts?: string[];
  set: ComponentProps<typeof ClusterBasicsStep>["set"];
}) {
  const offered = (draft: ClusterDraft) =>
    offeredPresets(
      presets,
      environments.find((e) => e.metadata.name === draft.environment),
    );
  const flow = useCreateFlow<ClusterDraft>({
    steps: CLUSTER_STEPS.map((s) =>
      s.id === "basics"
        ? {
            ...s,
            check: (draft: ClusterDraft) =>
              basicsErrors(draft, {
                environments,
                namespaces: namespaces.map((ns) => ({
                  name: ns.metadata?.name ?? "",
                  environment: environmentOf(ns, environments).environment
                    ?.metadata.name,
                })),
                takenNames: [],
                conflicts,
                presets: offered(draft).map((p) => p.metadata.name ?? ""),
              }),
          }
        : s,
    ),
    initial,
  });
  return (
    <WizardStepper
      steps={CLUSTER_STEPS.map((s) => ({ ...s, errors: flow.errors[s.id] }))}
      current={flow.current}
    >
      <ClusterBasicsStep
        draft={flow.draft}
        set={(path, value) => {
          set(path, value);
          flow.set(path, value);
        }}
        environments={environments}
        counts={environmentCounts(environments, namespaces, [])}
        namespaces={namespaces}
        presets={offered(flow.draft)}
        usage={new Map(Object.entries(usage))}
        nameErrors={[
          ...claimNameErrors(flow.draft.claim.metadata.name, []),
          ...conflicts,
        ]}
        hosts={hosts}
      />
    </WizardStepper>
  );
}

const meta = {
  title: "create/ClusterBasicsStep",
  component: ClusterBasicsStepHarness,
  args: {
    environments: [production, development, legacy],
    namespaces,
    presets,
    usage: presetUsage,
    initial: {
      environment: "prod",
      namespace: { kind: "existing", name: "team-orders" },
      claim: { metadata: { name: "search" }, spec: { presetRef: "medium" } },
    },
    set: fn(),
  },
  render: (args) => (
    <ClusterBasicsStepHarness key={JSON.stringify(args.initial)} {...args} />
  ),
} satisfies Meta<typeof ClusterBasicsStepHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ProductionPicked: Story = {
  name: "Production picked",
  args: {
    hosts: [
      { parameter: "hostnamePattern", host: "search.team-orders.example.com" },
      {
        parameter: "grpcHostnamePattern",
        host: "grpc-search.team-orders.example.com",
      },
    ],
  },
};

export const TakenHost: Story = {
  name: "Taken host",
  args: {
    environments: [production, development],
    initial: {
      environment: "prod",
      namespace: { kind: "existing", name: "team-orders" },
      claim: { metadata: { name: "billing" }, spec: { presetRef: "small" } },
    },
    hosts: [
      { parameter: "hostnamePattern", host: "billing.team-orders.example.com" },
    ],
    conflicts: [
      "The host billing.team-orders.example.com is taken by team-payments/billing. Choose another name.",
    ],
  },
};

export const NoEnvironment: Story = {
  name: "No environment",
  args: {
    environments: [],
    initial: { claim: { metadata: { name: "" }, spec: {} } },
  },
};
