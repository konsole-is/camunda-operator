import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { CamundaReleaseSpec } from "@/generated/crds/camundarelease";
import type { Story } from "@/gallery/registry";
import { fieldDocsOf } from "@/lib/fielddocs";
import { clusterFields } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster, type ManagedFieldsEntry } from "@/lib/merge";
import { ComponentSettings } from "./component-settings";
import { componentSettings } from "./settings";
import type { Process } from "./topology";

const CLAIM = "CamundaClusterClaim/cluster/0b5c7a2e";

const release = { version: "8.9.23" } as CamundaReleaseSpec;

const preset: CamundaClusterSpec = {
  zeebe: { replicas: 3, partitions: 3, storageSize: "32Gi" },
  gateway: { replicas: 2 },
};

function settings(
  component: Process,
  spec: CamundaClusterSpec,
  managedFields: ManagedFieldsEntry[],
  managed = true,
) {
  const merged = mergeCluster({ cluster: preset }, release, spec);
  const fields = clusterFields({
    cluster: {
      kind: "CamundaCluster",
      metadata: {
        name: "orders",
        namespace: "team-a",
        labels: managed ? { [CLAIM_LABEL]: "orders" } : {},
        managedFields,
      },
      spec,
    } as unknown as Resource<"CamundaCluster">,
    merged,
    claim: {
      kind: "CamundaClusterClaim",
      metadata: { name: "orders", namespace: "team-a" },
      spec: {
        presetRef: "medium",
        cluster: { zeebe: { replicas: 5 } },
        plugins: [
          { section: "volume-encryption", registration: "aws-ebs-kms" },
        ],
      },
    } as unknown as Resource<"CamundaClusterClaim">,
    sections: [
      {
        kind: "PluginSection",
        metadata: { name: "volume-encryption" },
        spec: {
          label: "Volume encryption",
          slot: "volumeEncryption",
          appliesTo: ["CamundaClusterClaim"],
          selection: "One",
        },
      } as unknown as Resource<"PluginSection">,
    ],
    registrations: [],
  });
  return (
    <ComponentSettings
      groups={componentSettings(fields, component, merged.spec)}
      docs={fieldDocsOf(fields)}
      operatorVersion="0.2.0"
      cluster={{ namespace: "team-a", name: "orders" }}
    />
  );
}

const owned = (manager: string, fields: object, time: string) => ({
  manager,
  time,
  fieldsV1: { "f:spec": fields },
});

export const settingsStories: Story[] = [
  {
    component: "ComponentSettings",
    name: "Brokers with a filled and a resized field",
    render: () =>
      settings(
        "zeebe",
        {
          zeebe: {
            replicas: 5,
            storageClassName: "encrypted",
            storageSize: "64Gi",
          },
          podLabels: { team: "orders" },
        },
        [
          owned(
            CLAIM,
            { "f:zeebe": { "f:replicas": {}, "f:storageClassName": {} } },
            "2026-10-01T00:00:00Z",
          ),
          owned(
            "pvc-autoresizer",
            { "f:zeebe": { "f:storageSize": {} } },
            "2026-10-02T00:00:00Z",
          ),
        ],
      ),
  },
  {
    component: "ComponentSettings",
    name: "Gateway with embedded apps",
    render: () =>
      settings(
        "gateway",
        {
          gateway: { mode: "Standalone" },
          operate: { replicas: 1, podLabels: { team: "orders" } },
          tasklist: { replicas: 1 },
        },
        [
          owned(
            CLAIM,
            {
              "f:gateway": { "f:mode": {} },
              "f:operate": {
                "f:replicas": {},
                "f:podLabels": { "f:team": {} },
              },
              "f:tasklist": { "f:replicas": {} },
            },
            "2026-10-01T00:00:00Z",
          ),
        ],
      ),
  },
  {
    component: "ComponentSettings",
    name: "Not managed",
    render: () =>
      settings(
        "zeebe",
        { zeebe: { replicas: 3, storageSize: "64Gi" } },
        [
          owned(
            "kubectl-client-side-apply",
            { "f:zeebe": { "f:replicas": {}, "f:storageSize": {} } },
            "2026-10-01T00:00:00Z",
          ),
        ],
        false,
      ),
  },
];
