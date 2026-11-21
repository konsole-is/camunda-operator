import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { type ComponentProps, useState } from "react";
import { expect, fn, userEvent, within } from "storybook/test";
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
// brokerResources replaces the resources of the brokers in the preset
// medium.
function ClusterSettingsStepHarness({
  initial,
  secrets,
  pick,
  set,
  reset,
  errors,
  brokerResources,
}: {
  initial: ClusterDraft;
  secrets: SecretRefs;
  errors?: ComponentProps<typeof ClusterSettingsStep>["errors"];
  brokerResources?: object;
  pick: ComponentProps<typeof ClusterSettingsStep>["pick"];
  set: ComponentProps<typeof ClusterSettingsStep>["set"];
  reset: ComponentProps<typeof ClusterSettingsStep>["reset"];
}) {
  const [draft, setDraft] = useState(initial);
  const offered =
    brokerResources === undefined
      ? presets
      : presets.map((p) =>
          p.metadata.name === "medium"
            ? {
                ...p,
                spec: {
                  cluster: {
                    ...p.spec.cluster,
                    zeebe: {
                      ...p.spec.cluster.zeebe,
                      resources: brokerResources,
                    },
                  },
                },
              }
            : p,
        );
  const drafted = draftCluster(
    draft,
    offered as unknown as Resource<"CamundaClusterPreset">[],
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
        presets={presets.map((p) => p.metadata.name ?? "")}
        fields={fields}
        merged={drafted.merged}
        below={drafted.below}
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
        errors={errors}
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

// brokers returns the settings group of the brokers and the button in its
// heading, which opens and closes the group.
function brokers(canvasElement: HTMLElement) {
  const group = within(canvasElement).getByRole("region", {
    name: "Brokers settings",
  });
  const row = within(
    within(group).getByRole("heading", { level: 3, name: "Brokers" }),
  ).getByRole("button");
  return { group, row };
}

export const Defaults: Story = {
  play: async ({ canvasElement }) => {
    const { row } = brokers(canvasElement);
    await expect(row).toHaveAttribute("aria-expanded", "false");
    await expect(row).toHaveAccessibleName("Brokers 0 changed in the claim");
  },
};

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

export const ChangedSetting: Story = {
  name: "Changed setting",
  args: {
    initial: {
      ...base,
      claim: {
        ...base.claim,
        spec: { ...base.claim.spec, cluster: { zeebe: { replicas: 5 } } },
      },
    },
  },
  play: async ({ canvasElement }) => {
    await expect(brokers(canvasElement).row).toHaveAccessibleName(
      "Brokers 1 changed in the claim",
    );
  },
};

export const ErrorInSection: Story = {
  name: "Error in a section",
  play: async ({ canvasElement }) => {
    const { group, row } = brokers(canvasElement);
    await userEvent.click(row);
    const replicas = within(
      within(group).getAllByRole("group", { name: "replicas" })[0],
    ).getByRole("spinbutton");
    await userEvent.clear(replicas);
    await userEvent.type(replicas, "-1");
    await userEvent.click(row);
    await expect(row).toHaveAttribute("aria-expanded", "false");
    await expect(row).toHaveAccessibleName(/ 1 error$/);
    await expect(within(row).getByText("1 error")).toBeVisible();
  },
};

export const MissingSecret: Story = {
  name: "Missing Secret",
  args: {
    initial: {
      ...base,
      claim: {
        ...base.claim,
        spec: {
          ...base.claim.spec,
          cluster: {
            auth: { clientSecretRef: { name: "orders-oidc", key: "secret" } },
          },
        },
      },
    },
    errors: [
      {
        path: "spec.cluster.auth.clientSecretRef",
        message:
          "The namespace team-orders holds no Secret orders-oidc. Pick a Secret of team-orders, or reset the field.",
      },
    ],
  },
};

export const PresetLimitBelowRequest: Story = {
  name: "Preset limit below the request",
  args: { brokerResources: { requests: { cpu: "2" }, limits: { cpu: "1" } } },
  play: async ({ canvasElement }) => {
    const { group, row } = brokers(canvasElement);
    await expect(row).toHaveAttribute("aria-expanded", "true");
    await expect(row).toHaveAccessibleName(/ 1 error$/);
    const resources = within(group).getAllByRole("group", {
      name: "resources",
    })[0];
    await expect(within(resources).getByRole("alert")).toHaveTextContent(
      "The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request.",
    );
    const limit = within(
      within(resources).getByRole("group", { name: "CPU limit" }),
    ).getByRole("textbox", { name: "Amount" });
    await userEvent.clear(limit);
    await expect(within(resources).getByRole("alert")).toHaveTextContent(
      "The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request.",
    );
    await expect(limit).toHaveAccessibleDescription(
      "The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request.",
    );
    await expect(row).toHaveAccessibleName(/ 1 error$/);
  },
};
