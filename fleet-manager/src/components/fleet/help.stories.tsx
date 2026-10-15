import type { Story } from "@/gallery/registry";
import { fieldDocFor } from "@/lib/fielddocs";
import { ConditionsTable } from "./conditions-table";
import { FieldHelp } from "./field-help";

const doc = fieldDocFor(
  "core.camunda.io",
  "CamundaCluster",
  "spec.zeebe.partitions",
);
const now = "2026-10-15T12:00:00Z";

const label = (open: boolean) => (
  <div className="flex items-center gap-1 text-sm font-medium text-muted">
    <span>Partitions</span>
    <FieldHelp
      label="Partitions"
      doc={doc}
      operatorVersion="0.2.3"
      defaultOpen={open}
    />
  </div>
);

export const helpStories: Story[] = [
  { component: "FieldHelp", name: "Closed", render: () => label(false) },
  {
    component: "FieldHelp",
    name: "Open",
    render: () => <div className="pb-48">{label(true)}</div>,
  },
  {
    component: "FieldHelp",
    name: "Open at the right edge",
    render: () => <div className="flex justify-end pb-48">{label(true)}</div>,
  },
  {
    component: "ConditionsTable",
    name: "Not ready condition",
    render: () => (
      <ConditionsTable
        now={now}
        conditions={[
          {
            type: "AdminSecretReady",
            status: "False",
            reason: "SecretMissing",
            message: "The Secret orders-admin does not exist.",
            lastTransitionTime: "2026-10-15T11:48:00Z",
          },
          {
            type: "MirroredSecretsReady",
            status: "True",
            reason: "Healthy",
            message: "",
            lastTransitionTime: "2026-10-14T09:00:00Z",
          },
        ]}
      />
    ),
  },
  {
    component: "ConditionsTable",
    name: "Groups",
    render: () => (
      <ConditionsTable
        now={now}
        conditions={[
          {
            type: "MirroredSecretsReady",
            status: "True",
            reason: "Healthy",
            lastTransitionTime: "2026-10-14T09:00:00Z",
          },
        ]}
        groups={[
          {
            label: "Claim",
            conditions: [
              {
                type: "RequestsReady",
                status: "True",
                reason: "Healthy",
                lastTransitionTime: "2026-10-15T11:00:00Z",
              },
              {
                type: "ClusterReady",
                status: "False",
                reason: "Converging",
                message: "Zeebe has 1 of 3 brokers ready.",
                lastTransitionTime: "2026-10-15T11:58:00Z",
              },
              {
                type: "AttachmentsReady",
                status: "Unknown",
                reason: "Waiting",
                lastTransitionTime: "2026-10-15T11:58:00Z",
              },
            ],
          },
        ]}
      />
    ),
  },
  {
    component: "ConditionsTable",
    name: "Empty",
    render: () => <ConditionsTable now={now} conditions={[]} />,
  },
];
