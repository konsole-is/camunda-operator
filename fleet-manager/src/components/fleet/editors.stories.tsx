import type { Story } from "@/gallery/registry";
import { EnvEditor } from "./env-editor";
import { KeyValueEditor } from "./key-value-editor";
import { SecretRefPicker } from "./secret-ref-picker";

const secrets = [
  { name: "orders-db", keys: ["password", "username"] },
  { name: "smtp", keys: ["token"] },
];

export const editorStories: Story[] = [
  {
    component: "EnvEditor",
    name: "Values and Secrets",
    render: () => (
      <EnvEditor
        label="Environment variables"
        secrets={secrets}
        defaultValue={[
          { name: "JAVA_TOOL_OPTIONS", value: "-Xmx2g" },
          {
            name: "DB_PASSWORD",
            valueFrom: { secretKeyRef: { name: "orders-db", key: "password" } },
          },
          {
            name: "MODE",
            valueFrom: { configMapKeyRef: { name: "settings", key: "mode" } },
          },
        ]}
      />
    ),
  },
  {
    component: "EnvEditor",
    name: "Empty",
    render: () => <EnvEditor label="Environment variables" secrets={secrets} />,
  },
  {
    component: "KeyValueEditor",
    name: "Labels",
    render: () => (
      <KeyValueEditor
        label="Pod labels"
        format="label"
        defaultValue={{ team: "payments", tier: "gold" }}
      />
    ),
  },
  {
    component: "KeyValueEditor",
    name: "Empty",
    render: () => <KeyValueEditor label="Annotations" />,
  },
  {
    component: "SecretRefPicker",
    name: "Picked",
    render: () => (
      <SecretRefPicker
        label="Client secret"
        secrets={secrets}
        defaultValue={{ name: "smtp", key: "token" }}
      />
    ),
  },
  {
    component: "SecretRefPicker",
    name: "No Secrets",
    render: () => <SecretRefPicker label="Client secret" secrets={[]} />,
  },
];
