"use client";

import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import { clusterFields, parameterFields } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import type { ResolvedPolicy } from "@/lib/policy";
import type { OpenAPISchema } from "@/lib/schema";
import { settingsOf } from "./editable-settings";
import { SelectionSettingsEditor } from "./selection-settings-editor";
import { SettingsEditor } from "./settings-editor";
import { componentSettings } from "./settings";
import type { Process } from "./topology";

const SECRETS = [
  { name: "orders-db", keys: ["password", "username"] },
  { name: "orders-oidc", keys: ["secret"] },
];

const preset: CamundaClusterSpec = {
  zeebe: {
    replicas: 3,
    partitions: 3,
    storageSize: "32Gi",
    resources: {
      requests: { cpu: "1", memory: "4Gi" },
      limits: { memory: "4Gi" },
    },
  },
  gateway: { replicas: 2 },
};

/** SettingsEditorStoryName names one state of the settings editors in the gallery. */
export type SettingsEditorStoryName =
  "brokers" | "embedded" | "locked" | "optimize" | "selection";

/**
 * SettingsEditorStory renders the settings editors of the cluster orders in
 * one named state, with changes that go nowhere.
 */
export function SettingsEditorStory({
  name,
}: {
  name: SettingsEditorStoryName;
}) {
  const ignore = () => {};
  if (name === "selection")
    return (
      <SelectionSettingsEditor
        fields={selectionFields()}
        schema={ELASTICSEARCH}
        secrets={SECRETS}
        set={ignore}
        reset={ignore}
      />
    );
  const { host, process, cluster, live, optimize } = STATES[name];
  const spec = { ...cluster, ...live };
  const merged = mergeCluster({ cluster: preset }, undefined, spec);
  const fields = clusterFields({
    cluster: {
      kind: "CamundaCluster",
      metadata: {
        name: "orders",
        namespace: "team-a",
        labels: { [CLAIM_LABEL]: "orders" },
      },
      spec,
    } as unknown as Resource<"CamundaCluster">,
    merged,
    claim: {
      kind: "CamundaClusterClaim",
      metadata: { name: "orders", namespace: "team-a" },
      spec: {
        presetRef: "medium",
        cluster,
        ...(optimize && { attachments: { optimize } }),
      },
    } as unknown as Resource<"CamundaClusterClaim">,
    sections: [],
    registrations: [],
    optimize:
      optimize &&
      ({
        kind: "CamundaOptimize",
        metadata: { name: "orders", namespace: "team-a" },
        spec: {
          clusterRef: { name: "orders" },
          managementAuthRef: optimize.managementAuthConfigRef,
          importer: optimize.importer,
        },
      } as unknown as Resource<"CamundaOptimize">),
  });
  const groups = componentSettings(fields, host, merged.spec);
  const group = groups.find((g) => g.component === process) ?? groups[0];
  return (
    <SettingsEditor
      group={group}
      settings={settingsOf({
        group,
        spec: process === "optimize" ? optimize : merged.spec,
      })}
      secrets={SECRETS}
      set={ignore}
      reset={ignore}
    />
  );
}

type State = {
  host: Process | "optimize";
  process: Process | "optimize";
  cluster: CamundaClusterSpec;
  live?: CamundaClusterSpec;
  optimize?: {
    managementAuthConfigRef: string;
    importer: { replicas: number };
  };
};

const STATES: Record<Exclude<SettingsEditorStoryName, "selection">, State> = {
  brokers: {
    host: "zeebe",
    process: "zeebe",
    cluster: {
      zeebe: { replicas: 5 },
      extraEnv: [{ name: "TZ", value: "UTC" }],
    },
  },
  embedded: {
    host: "gateway",
    process: "tasklist",
    cluster: { gateway: { mode: "Standalone" }, tasklist: { replicas: 1 } },
  },
  locked: {
    host: "connectors",
    process: "connectors",
    cluster: {},
    live: {
      connectors: {
        version: "8.9.23",
        serviceAccount: { name: "orders-connectors" },
      },
    },
  },
  optimize: {
    host: "optimize",
    process: "optimize",
    cluster: {},
    optimize: {
      managementAuthConfigRef: "keycloak",
      importer: { replicas: 1 },
    },
  },
};

const ELASTICSEARCH: OpenAPISchema = {
  type: "object",
  properties: {
    preset: { type: "string", title: "Preset" },
    releaseRef: { type: "string", title: "Release" },
    overrides: {
      type: "object",
      properties: {
        replicas: { type: "integer", format: "int32", minimum: 1 },
        resources: {
          type: "object",
          "x-kubernetes-preserve-unknown-fields": true,
        },
        storageSize: { type: "string" },
        scheduling: {
          type: "object",
          "x-kubernetes-preserve-unknown-fields": true,
        },
        podLabels: {
          type: "object",
          additionalProperties: { type: "string" },
        },
      },
    },
  },
};

function selectionFields() {
  return parameterFields({
    cluster: {
      kind: "CamundaCluster",
      metadata: { name: "orders", labels: { [CLAIM_LABEL]: "orders" } },
      spec: {},
    } as unknown as Resource<"CamundaCluster">,
    merged: mergeCluster(undefined, undefined, {}),
    claim: {
      kind: "CamundaClusterClaim",
      metadata: { name: "orders" },
      spec: {
        presetRef: "medium",
        plugins: [
          {
            section: "secondary-storage",
            registration: "elasticsearch",
            parameters: { overrides: { replicas: 3, storageSize: "50Gi" } },
          },
        ],
      },
    } as unknown as Resource<"CamundaClusterClaim">,
    policy: {
      parameters: {
        elasticsearch: {
          preset: { value: "medium" },
          releaseRef: { value: "camunda-8-9-23" },
          overrides: {
            claimEditable: true,
            value: { replicas: 1, storageSize: "50Gi" },
          },
        },
      },
    } as unknown as ResolvedPolicy,
    sections: [],
    registrations: [
      {
        kind: "PluginRegistration",
        metadata: { name: "elasticsearch" },
        spec: { parameters: ELASTICSEARCH },
      } as unknown as Resource<"PluginRegistration">,
    ],
  });
}
