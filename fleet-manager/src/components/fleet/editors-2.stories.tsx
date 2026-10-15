import type { Story } from "@/gallery/registry";
import { ResourcesEditor } from "./resources-editor";
import { SchedulingEditor } from "./scheduling-editor";
import { YamlEditor } from "./yaml-editor";

export const editorStories2: Story[] = [
  {
    component: "ResourcesEditor",
    name: "Set",
    render: () => (
      <ResourcesEditor
        label="Zeebe resources"
        defaultValue={{
          requests: { cpu: "500m", memory: "2Gi" },
          limits: { cpu: "2", memory: "4Gi" },
        }}
      />
    ),
  },
  {
    component: "ResourcesEditor",
    name: "Empty",
    render: () => <ResourcesEditor label="Gateway resources" />,
  },
  {
    component: "SchedulingEditor",
    name: "Tolerations",
    render: () => (
      <SchedulingEditor
        label="Tolerations"
        defaultValue={[
          {
            key: "dedicated",
            operator: "Equal",
            value: "camunda",
            effect: "NoSchedule",
          },
          {
            key: "node.kubernetes.io/unreachable",
            operator: "Exists",
            effect: "NoExecute",
            tolerationSeconds: 300,
          },
        ]}
      />
    ),
  },
  {
    component: "SchedulingEditor",
    name: "Empty",
    render: () => <SchedulingEditor label="Tolerations" />,
  },
  {
    component: "YamlEditor",
    name: "Node affinity",
    render: () => (
      <YamlEditor
        label="Node affinity"
        defaultValue={{
          requiredDuringSchedulingIgnoredDuringExecution: {
            nodeSelectorTerms: [
              {
                matchExpressions: [
                  { key: "pool", operator: "In", values: ["camunda"] },
                ],
              },
            ],
          },
        }}
      />
    ),
  },
  {
    component: "YamlEditor",
    name: "Empty",
    render: () => <YamlEditor label="Extra environment sources" />,
  },
];
