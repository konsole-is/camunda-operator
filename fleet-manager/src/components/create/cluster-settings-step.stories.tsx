import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { type ComponentProps, useState } from "react";
import { fn } from "storybook/test";
import { createReleaseChoices } from "@/components/cluster/edit/release-options";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import { WizardStepper } from "@/components/fleet/wizard-stepper";
import type { Environment } from "@/generated/crds/environment";
import {
  CLUSTER_STEPS,
  type ClusterDraft,
  draftCluster,
  withClaimValue,
  withoutClaimValue,
} from "@/lib/clusters/draft";
import { clusterFields } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { ClusterSettingsStep } from "./cluster-settings-step";
import { presets, production } from "./fixtures";

const release = (name: string, version: string) =>
  ({
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaRelease",
    metadata: { name },
    spec: { version },
  }) as unknown as Resource<"CamundaRelease">;

const releases = [
  release("camunda-8-8", "8.8.4"),
  release("camunda-8-9", "8.9.9"),
];

const environment: Environment = {
  ...production,
  spec: {
    ...production.spec,
    releases: {
      allowed: ["camunda-8-8", "camunda-8-9"],
      default: "camunda-8-9",
    },
    platformConfigRef: "basic",
  },
};

const accessSection = {
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginSection",
  metadata: { name: "access" },
  spec: {
    appliesTo: ["CamundaClusterClaim"],
    label: "Access",
    selection: "One",
    slot: "externalAccess",
  },
} as unknown as Resource<"PluginSection">;

const teamSecrets: SecretRefs = [
  { name: "orders-db", keys: ["password", "username"] },
  { name: "orders-license", keys: ["license"] },
];

// ClusterSettingsStep holds no draft, so the harness holds the draft and
// derives the draft cluster and its fields as the create flow does.
function ClusterSettingsStepHarness({
  initial,
  secrets,
  pick,
  set,
  reset,
}: {
  initial: ClusterDraft;
  secrets: SecretRefs;
  pick: ComponentProps<typeof ClusterSettingsStep>["pick"];
  set: ComponentProps<typeof ClusterSettingsStep>["set"];
  reset: ComponentProps<typeof ClusterSettingsStep>["reset"];
}) {
  const [draft, setDraft] = useState(initial);
  const drafted = draftCluster(
    draft,
    presets as unknown as Resource<"CamundaClusterPreset">[],
    releases,
    environment,
  );
  const fields = clusterFields({
    cluster: drafted.cluster,
    merged: drafted.merged,
    claim: drafted.claim as unknown as Resource<"CamundaClusterClaim">,
    sections: [accessSection],
    registrations: [],
  });
  return (
    <WizardStepper steps={[...CLUSTER_STEPS]} current="settings">
      <ClusterSettingsStep
        draft={draft}
        environment={environment}
        releases={createReleaseChoices(releases, environment)}
        platformConfigs={["basic", "oidc"]}
        fields={fields}
        merged={drafted.merged}
        secrets={secrets}
        pick={(field, name) => {
          pick(field, name);
          setDraft((d) =>
            name === undefined
              ? withoutClaimValue(d, [{ field: "spec" }, { field }])
              : withClaimValue(d, [{ field: "spec" }, { field }], name),
          );
        }}
        set={(steps, value) => {
          set(steps, value);
          setDraft((d) => withClaimValue(d, steps, value));
        }}
        reset={(steps) => {
          reset(steps);
          setDraft((d) => withoutClaimValue(d, steps));
        }}
        invalid={() => {}}
      />
    </WizardStepper>
  );
}

const base: ClusterDraft = {
  environment: "prod",
  namespace: { kind: "existing", name: "team-orders" },
  claim: { metadata: { name: "orders" }, spec: { presetRef: "medium" } },
};

const meta = {
  title: "create/ClusterSettingsStep",
  component: ClusterSettingsStepHarness,
  args: {
    initial: base,
    secrets: teamSecrets,
    pick: fn(),
    set: fn(),
    reset: fn(),
  },
  render: (args) => (
    <ClusterSettingsStepHarness key={JSON.stringify(args)} {...args} />
  ),
} satisfies Meta<typeof ClusterSettingsStepHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Defaults: Story = {};

export const Overridden: Story = {
  args: {
    initial: {
      ...base,
      claim: {
        ...base.claim,
        spec: {
          ...base.claim.spec,
          releaseRef: "camunda-8-8",
          platformConfigRef: "oidc",
          cluster: { podLabels: { team: "orders" } },
        },
      },
    },
  },
};

export const FilledByAccess: Story = {
  name: "Filled by Access",
  args: {
    initial: {
      ...base,
      storage: { access: [{ section: "access", registration: "gateway" }] },
    },
  },
};

export const NewNamespace: Story = {
  name: "New namespace",
  args: {
    initial: {
      ...base,
      namespace: { kind: "new", name: "team-new", labels: {} },
    },
    secrets: [],
  },
};
