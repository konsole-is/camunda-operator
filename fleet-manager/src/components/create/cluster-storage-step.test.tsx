// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect, useState } from "react";
import { afterEach, describe, expect, it } from "vitest";
import type { ElasticsearchClusterPreset } from "@/generated/crds/elasticsearchclusterpreset";
import type { Environment } from "@/generated/crds/environment";
import type {
  PluginRegistration,
  PluginRegistrationSpec,
} from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import {
  type ClusterClaimObject,
  type ClusterDraft,
  clusterClaimOf,
  draftSelections,
} from "@/lib/clusters/draft";
import {
  type StorageData,
  clusterDeletion,
  storageErrors,
  storageSummary,
  storageViews,
} from "@/lib/clusters/storage";
import type { ValidationError } from "@/lib/schema";
import { ClusterStorageStep } from "./cluster-storage-step";
import { useCreateFlow } from "./use-create-flow";

afterEach(cleanup);

function section(
  name: string,
  label: string,
  order: number,
  emptyOptionLabel?: string,
): PluginSection {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name },
    spec: {
      appliesTo: ["CamundaClusterClaim"],
      label,
      selection: "One",
      order,
      ...(emptyOptionLabel && { emptyOptionLabel }),
    },
  };
}

function registration(
  name: string,
  label: string,
  spec: Partial<PluginRegistrationSpec>,
): PluginRegistration {
  return {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      sections: [],
      mode: "Optional",
      label,
      deletionPolicies: ["Delete"],
      serviceAccount: { name, namespace: "plugins" },
      ...spec,
    },
    status: {
      conditions: ["Accepted", "Ready"].map((type) => ({
        type,
        status: "True" as const,
        reason: "Test",
        message: "",
        lastTransitionTime: "2026-11-10T00:00:00Z",
      })),
    },
  };
}

function esPreset(name: string, replicas: number): ElasticsearchClusterPreset {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "ElasticsearchClusterPreset",
    metadata: { name },
    spec: { cluster: { replicas } },
  };
}

const POLICY_ARN = "^arn:aws[a-z-]*:iam::([0-9]{12}|aws):policy/.+$";

const SECTIONS = [
  section("encryption-key", "Encryption key", 20, "No key"),
  section("volume-encryption", "Volume encryption", 30, "Unencrypted volumes"),
  section("backup-storage", "Backup storage", 40, "No backups"),
  section("secondary-storage", "Secondary storage", 50),
  section("connectors-permissions", "Connectors permissions", 70, "None"),
];

const REGISTRATIONS = [
  registration("kms", "KMS key", { sections: ["encryption-key"] }),
  registration("ebs", "EBS", {
    sections: ["volume-encryption"],
    inputs: [{ name: "keyRef", from: "encryption-key/keyRef", required: true }],
  }),
  registration("s3", "S3 bucket", {
    sections: ["backup-storage"],
    parameters: {
      type: "object",
      properties: {
        retentionDays: {
          type: "integer",
          minimum: 1,
          maximum: 365,
          title: "Retention in days",
        },
      },
    },
  }),
  registration("elasticsearch", "New Elasticsearch cluster", {
    sections: ["secondary-storage"],
    controller: { name: "camunda-operator", version: "0.0.0" },
    parameters: {
      type: "object",
      properties: {
        preset: { type: "string", title: "Preset" },
        releaseRef: { type: "string", title: "Release" },
      },
    },
    inputs: [
      {
        name: "objectStorageConfig",
        from: "backup-storage/objectStorageConfig",
      },
    ],
  }),
  registration("aws-connectors-role", "Role for each cluster", {
    sections: ["connectors-permissions"],
    parameters: {
      type: "object",
      properties: {
        managedPolicyArns: {
          type: "array",
          maxItems: 10,
          items: { type: "string", pattern: POLICY_ARN },
        },
        extraPolicyArns: {
          type: "array",
          maxItems: 10,
          items: { type: "string", pattern: POLICY_ARN },
        },
        inlinePolicy: {
          type: "object",
          "x-kubernetes-preserve-unknown-fields": true,
        },
      },
    },
  }),
];

const ENV: Environment = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "prod" },
  spec: {
    sections: {
      "encryption-key": { allowed: ["kms"] },
      "volume-encryption": { allowed: ["ebs"] },
      "backup-storage": { allowed: ["s3"] },
      "secondary-storage": {
        allowed: ["elasticsearch"],
        default: "elasticsearch",
        required: true,
      },
      "connectors-permissions": { allowed: ["aws-connectors-role"] },
    },
    presets: { elasticsearchCluster: ["small", "large"] },
    parameters: {
      s3: { retentionDays: { claimEditable: true } },
      elasticsearch: { preset: { claimEditable: true } },
      "aws-connectors-role": {
        managedPolicyArns: {
          value: [
            "arn:aws:iam::aws:policy/AmazonEC2ReadOnlyAccess",
            "arn:aws:iam::aws:policy/AmazonSNSReadOnlyAccess",
          ],
        },
        extraPolicyArns: {
          claimEditable: true,
          allowed: [
            {
              name: "sqs",
              label: "SQS access",
              value: "arn:aws:iam::aws:policy/AmazonSQSFullAccess",
            },
            {
              name: "s3",
              label: "S3 read",
              value: "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess",
            },
          ],
        },
        inlinePolicy: {
          value: {
            Version: "2012-10-17",
            Statement: [
              {
                Effect: "Allow",
                Action: "sqs:*",
                Resource: "arn:aws:sqs:*:*:x",
              },
            ],
          },
        },
      },
    },
  },
};

const DATA: StorageData = {
  environment: ENV,
  sections: SECTIONS,
  registrations: REGISTRATIONS,
  objects: {
    ElasticsearchClusterPreset: [esPreset("small", 1), esPreset("large", 3)],
  },
};

const START: ClusterDraft = {
  environment: "prod",
  namespace: { kind: "existing", name: "team-a" },
  claim: { metadata: { name: "c1" }, spec: { presetRef: "small" } },
};

/** last holds the draft, the step errors, and the claim of the last render of Harness. */
const last = {
  draft: START,
  errors: [] as string[],
  claim: undefined as ClusterClaimObject | undefined,
};
const harness = {
  draft: () => last.draft,
  errors: () => last.errors,
  claim: () => last.claim!,
};

function Harness({
  data = DATA,
  initial = START,
}: {
  data?: StorageData;
  initial?: ClusterDraft;
}) {
  const flow = useCreateFlow<ClusterDraft>({
    steps: [{ id: "storage", label: "Storage and access" }],
    initial,
  });
  const [cardErrors, setCardErrors] = useState<
    Record<string, ValidationError[]>
  >({});
  const views = storageViews(flow.draft, data);
  const errors = storageErrors(flow.draft, data, cardErrors, []);
  const deletion = clusterDeletion(flow.draft, data);
  useEffect(() => {
    last.draft = flow.draft;
    last.errors = errors;
    last.claim = clusterClaimOf(flow.draft, data.environment, deletion);
  });
  return (
    <ClusterStorageStep
      draft={flow.draft}
      views={views}
      registrations={data.registrations}
      sections={data.sections}
      context={{ environment: data.environment, objects: data.objects }}
      hosts={[]}
      conflicts={[]}
      onSection={(name, selections, errors) => {
        flow.set(["storage", name], selections);
        setCardErrors((current) => ({ ...current, [name]: errors }));
      }}
      deletion={deletion}
      onDeletion={(policy) =>
        flow.set(["claim", "spec", "deletionPolicy"], policy)
      }
    />
  );
}

const radio = (name: RegExp) =>
  screen.getByRole("radio", { name }) as HTMLInputElement;

describe("ClusterStorageStep", () => {
  it("enables the volume encryption option once an encryption key is chosen", async () => {
    render(<Harness />);
    expect(radio(/EBS/).disabled).toBe(true);
    expect(screen.getByText("Needs an encryption key")).toBeTruthy();

    await userEvent.click(radio(/KMS key/));

    expect(radio(/EBS/).disabled).toBe(false);
  });

  it("drops the volume encryption pick when the key pick goes", async () => {
    render(<Harness />);
    await userEvent.click(radio(/KMS key/));
    await userEvent.click(radio(/EBS/));
    expect(
      draftSelections(harness.draft()).map((s) => s.registration),
    ).toContain("ebs");

    await userEvent.click(radio(/No key/));

    expect(radio(/EBS/).disabled).toBe(true);
    expect(
      draftSelections(harness.draft()).map((s) => s.registration),
    ).not.toContain("ebs");
    expect(harness.errors()).toEqual([]);
  });

  it("shows a parameter outside its range at its field and blocks the step", async () => {
    render(<Harness />);
    await userEvent.click(radio(/S3 bucket/));
    await userEvent.type(
      screen.getByRole("spinbutton", { name: "Retention in days" }),
      "0",
    );

    expect(screen.getByText("must be >= 1")).toBeTruthy();
    expect(harness.errors()).toEqual([
      "Fix the Retention in days of S3 bucket.",
    ]);
  });

  it("shows the allowed labels of aws-connectors-role and no ARN", async () => {
    const { container } = render(<Harness />);
    await userEvent.click(radio(/Role for each cluster/));
    const group = screen.getByRole("group", { name: "extraPolicyArns" });
    await userEvent.click(
      within(group).getByRole("checkbox", { name: "SQS access" }),
    );

    expect(within(group).getByText("S3 read")).toBeTruthy();
    expect(screen.getByText("2 items")).toBeTruthy();
    expect(screen.getByText("2 fields")).toBeTruthy();
    expect(container.innerHTML).not.toMatch(/arn:aws/);
    const claim = harness.claim();
    expect(claim.spec.plugins).toContainEqual({
      section: "connectors-permissions",
      registration: "aws-connectors-role",
      parameters: { extraPolicyArns: ["sqs"] },
    });
    expect(JSON.stringify(claim)).not.toMatch(/arn:aws/);
  });

  it("shows preset cards for an editable built-in preset and the snapshot line", async () => {
    render(<Harness />);

    const presets = screen.getByRole("group", { name: "Preset" });
    expect(within(presets).getByText("small")).toBeTruthy();
    expect(within(presets).getByText("large")).toBeTruthy();
    expect(within(presets).getAllByText("Nodes")).toHaveLength(2);
    expect(screen.queryByText(/^Snapshots:/)).toBeNull();

    await userEvent.click(
      within(presets).getByRole("radio", { name: /^small/ }),
    );
    expect(screen.getByText("Snapshots: No snapshots")).toBeTruthy();

    await userEvent.click(radio(/S3 bucket/));

    expect(screen.getByText("Snapshots: From Backup storage")).toBeTruthy();
  });

  it("keeps a typed parameter when a live list updates", async () => {
    const { rerender } = render(<Harness />);
    await userEvent.click(radio(/S3 bucket/));
    await userEvent.type(
      screen.getByRole("spinbutton", { name: "Retention in days" }),
      "30",
    );

    rerender(<Harness data={structuredClone(DATA)} />);

    expect(
      (
        screen.getByRole("spinbutton", {
          name: "Retention in days",
        }) as HTMLInputElement
      ).value,
    ).toBe("30");
    expect(draftSelections(harness.draft())).toContainEqual({
      section: "backup-storage",
      registration: "s3",
      parameters: { retentionDays: 30 },
    });
  });

  it("restores the picks of a shown section and the picked empty option", () => {
    render(
      <Harness
        initial={{
          ...START,
          storage: {
            "encryption-key": [
              { section: "encryption-key", registration: "kms" },
            ],
            "volume-encryption": [],
          },
        }}
      />,
    );

    expect(radio(/KMS key/).checked).toBe(true);
    expect(radio(/Unencrypted volumes/).checked).toBe(true);
    expect(radio(/EBS/).checked).toBe(false);
  });

  it("preselects the default of a section that the step did not show before", () => {
    render(<Harness />);

    expect(radio(/New Elasticsearch cluster/).checked).toBe(true);
  });

  it("preselects a default that needs the default of another section on the first visit", () => {
    const sections = ENV.spec.sections!;
    render(
      <Harness
        data={{
          ...DATA,
          environment: {
            ...ENV,
            spec: {
              ...ENV.spec,
              sections: {
                ...sections,
                "encryption-key": { allowed: ["kms"], default: "kms" },
                "volume-encryption": { allowed: ["ebs"], default: "ebs" },
              },
            },
          },
        }}
      />,
    );

    expect(radio(/KMS key/).checked).toBe(true);
    expect(radio(/EBS/).checked).toBe(true);
    expect(draftSelections(harness.draft()).map((s) => s.registration)).toEqual(
      expect.arrayContaining(["kms", "ebs"]),
    );
  });

  it("says that the admin allows no option in a section without options", () => {
    render(
      <Harness
        data={{
          ...DATA,
          environment: { ...ENV, spec: { ...ENV.spec, sections: {} } },
        }}
      />,
    );

    // Only Secondary storage has no empty option.
    expect(
      screen.getAllByText("Your admin allows no option here."),
    ).toHaveLength(1);
    expect(radio(/No key/).checked).toBe(true);
  });

  const keeping = (spec: Environment["spec"] = ENV.spec): StorageData => ({
    ...DATA,
    environment: { ...ENV, spec },
    registrations: DATA.registrations.map((r) =>
      r.metadata.name === "s3"
        ? r
        : {
            ...r,
            spec: { ...r.spec, deletionPolicies: ["Delete", "Orphan"] },
          },
    ) as PluginRegistration[],
  });

  it("keeps the default of the Environment once the pick that cannot keep goes", async () => {
    render(
      <Harness
        data={keeping({
          ...ENV.spec,
          deletionPolicy: "Orphan",
          sections: {
            ...ENV.spec.sections,
            "backup-storage": { allowed: ["s3"], default: "s3" },
          },
        })}
      />,
    );
    expect(radio(/Keep the resources/).disabled).toBe(true);
    expect(radio(/Delete the resources/).checked).toBe(true);
    expect(
      screen.getByText(
        "The default keeps the resources, but the options below cannot keep them.",
      ),
    ).toBeTruthy();
    expect(harness.claim().spec.deletionPolicy).toBe("Delete");

    await userEvent.click(radio(/No backups/));

    expect(radio(/Keep the resources/).checked).toBe(true);
    expect(harness.claim().spec).not.toHaveProperty("deletionPolicy");
  });

  it("keeps the Keep pick of the user through a pick that cannot keep", async () => {
    render(<Harness data={keeping()} />);
    await userEvent.click(radio(/Keep the resources/));
    expect(harness.claim().spec.deletionPolicy).toBe("Orphan");

    await userEvent.click(radio(/S3 bucket/));
    expect(radio(/Keep the resources/).disabled).toBe(true);
    expect(radio(/Delete the resources/).checked).toBe(true);
    expect(screen.getByText("S3 bucket (Backup storage)")).toBeTruthy();
    expect(harness.claim().spec).not.toHaveProperty("deletionPolicy");

    await userEvent.click(radio(/No backups/));
    expect(radio(/Keep the resources/).checked).toBe(true);
    expect(harness.claim().spec.deletionPolicy).toBe("Orphan");
  });

  it("lists the picked labels in the summary for the review, without an ARN", async () => {
    render(<Harness />);
    await userEvent.click(radio(/Role for each cluster/));
    const group = screen.getByRole("group", { name: "extraPolicyArns" });
    await userEvent.click(
      within(group).getByRole("checkbox", { name: "SQS access" }),
    );

    const summary = storageSummary(harness.draft(), DATA);
    expect(summary.picks).toContainEqual(
      expect.objectContaining({
        registration: "aws-connectors-role",
        values: [{ label: "extraPolicyArns", value: "SQS access" }],
      }),
    );
    expect(JSON.stringify(summary)).not.toMatch(/arn:aws/);
  });
});
