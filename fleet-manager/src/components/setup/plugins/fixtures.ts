import type { V1Namespace } from "@kubernetes/client-node";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import type { Environment } from "@/generated/crds/environment";
import type { Grant } from "@/generated/crds/grant";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { Resource } from "@/lib/k8s/kinds";
import { BUILTIN_CONTROLLER, ENVIRONMENT_LABEL } from "@/lib/labels";
import type { RegistrationDetailData } from "./detail-sources";
import type { RegistrationSources } from "./sources";

const SINCE = "2026-11-10T09:00:00Z";
const FLEET = "fleet.camunda.io/v1alpha1";
const PLUGINS = "plugins.camunda.io/v1alpha1";

/** condition returns a condition that changed at 2026-11-10T09:00:00Z. */
export function condition(
  type: string,
  status: "True" | "False" | "Unknown",
  reason: string,
  message = "",
): ConditionRow {
  return { type, status, reason, message, lastTransitionTime: SINCE };
}

/** pluginSection returns a section for one cluster claim, with selection One, unless spec says otherwise. */
export function pluginSection(
  name: string,
  spec: Partial<PluginSection["spec"]>,
): PluginSection {
  return {
    apiVersion: PLUGINS,
    kind: "PluginSection",
    metadata: { name },
    spec: {
      label: name,
      selection: "One",
      appliesTo: ["CamundaClusterClaim"],
      ...spec,
    },
  };
}

/**
 * pluginRegistration returns an optional registration with the label name
 * and both deletion policies, unless spec says otherwise. It has a status
 * only with conditions.
 */
export function pluginRegistration(
  name: string,
  spec: Partial<PluginRegistration["spec"]>,
  conditions?: ConditionRow[],
): PluginRegistration {
  return {
    apiVersion: PLUGINS,
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      label: name,
      sections: [],
      mode: "Optional",
      deletionPolicies: ["Delete", "Orphan"],
      serviceAccount: { name: "controller", namespace: "plugins" },
      ...spec,
    },
    ...(conditions && {
      status: {
        conditions: conditions.map((c) => ({
          message: "",
          reason: "",
          ...c,
          lastTransitionTime: String(c.lastTransitionTime ?? SINCE),
        })) as NonNullable<PluginRegistration["status"]>["conditions"],
      },
    }),
  };
}

/** pluginRequest returns a request of a cluster claim named after namespace, unless spec says otherwise. */
export function pluginRequest(
  namespace: string,
  name: string,
  spec: Partial<PluginRequest["spec"]>,
  status?: PluginRequest["status"],
): PluginRequest {
  return {
    apiVersion: PLUGINS,
    kind: "PluginRequest",
    metadata: { namespace, name },
    spec: {
      registration: "",
      section: "",
      claimRef: { kind: "CamundaClusterClaim", name: namespace },
      target: { kind: "CamundaCluster", name: namespace },
      deletionPolicy: "Delete",
      ...spec,
    },
    ...(status && { status }),
  };
}

/**
 * pluginGrant returns a Grant of the template access for the cluster
 * orders, with an empty statement, unless spec says otherwise. It has a
 * status only with conditions.
 */
export function pluginGrant(
  namespace: string,
  name: string,
  spec: Partial<Grant["spec"]>,
  labels: Record<string, string>,
  conditions?: ConditionRow[],
): Grant {
  return {
    apiVersion: PLUGINS,
    kind: "Grant",
    metadata: { namespace, name, labels },
    spec: {
      cluster: { name: "orders" },
      format: "aws-iam-statement",
      fragment: {},
      template: "access",
      ...spec,
    },
    ...(conditions && {
      status: {
        conditions: conditions.map((c) => ({
          message: "",
          ...c,
          lastTransitionTime: String(c.lastTransitionTime ?? SINCE),
        })) as NonNullable<Grant["status"]>["conditions"],
      },
    }),
  };
}

/** registrationEvent returns an event about the registration name at 2026-11-13T08:55:00Z. */
export function registrationEvent(
  name: string,
  type: "Normal" | "Warning",
  reason: string,
  note: string,
): Resource<"Event"> {
  return {
    apiVersion: "events.k8s.io/v1",
    kind: "Event",
    metadata: {
      name: `${name}.${reason}`,
      namespace: "default",
      uid: `${name}/${reason}`,
    },
    type,
    reason,
    note,
    eventTime: "2026-11-13T08:55:00Z",
    regarding: { apiVersion: PLUGINS, kind: "PluginRegistration", name },
  } as Resource<"Event">;
}

/**
 * detailData returns the first snapshot of the detail page of the built-in
 * registration name in registrationSources, with no Grant and no event.
 */
export function detailData(name: string): RegistrationDetailData {
  const sources = registrationSources();
  const registration = sources.registrations.find(
    (r) => r.metadata.name === name,
  );
  if (!registration) throw new Error(`no built-in registration ${name}`);
  return { name, registration, sources, grants: [], events: [] };
}

/** labelledNamespace returns a namespace with the environment label when environment is set. */
export function labelledNamespace(
  name: string,
  environment?: string,
): V1Namespace {
  return {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: {
      name,
      ...(environment && { labels: { [ENVIRONMENT_LABEL]: environment } }),
    },
  };
}

/** fleetEnvironment returns an Environment with spec, empty by default. */
export function fleetEnvironment(
  name: string,
  spec: Partial<Environment["spec"]> = {},
): Environment {
  return {
    apiVersion: FLEET,
    kind: "Environment",
    metadata: { name },
    spec,
  };
}

const HEALTHY = [
  condition("Accepted", "True", "Healthy", "All checks passed"),
  condition(
    "Ready",
    "True",
    "Healthy",
    "The operator serves the requests of this registration",
  ),
];

const SECONDARY_STORAGE_OUTPUT = {
  key: "secondaryStorageConfig",
  description: "The name of the SecondaryStorageConfig that the cluster reads.",
};

const DATABASE_OUTPUTS = [
  {
    key: "keycloakDatabaseConfig",
    description: "The name of the DatabaseConfig of Keycloak.",
  },
  {
    key: "identityDatabaseConfig",
    description: "The name of the DatabaseConfig of Management Identity.",
  },
  {
    key: "webModelerDatabaseConfig",
    description: "The name of the DatabaseConfig of Web Modeler.",
  },
];

function builtin(
  name: string,
  label: string,
  section: string,
  spec: Partial<PluginRegistration["spec"]> = {},
): PluginRegistration {
  return pluginRegistration(
    name,
    {
      label,
      sections: [section],
      controller: { name: BUILTIN_CONTROLLER, version: "0.0.0-dev" },
      serviceAccount: {
        name: "camunda-operator-controller-manager",
        namespace: "camunda-system",
      },
      ...spec,
    },
    HEALTHY,
  );
}

// The core sections that the operator writes at startup, as the scenario
// claim-ready holds them.
const CORE_SECTIONS: PluginSection[] = [
  pluginSection("cloud-identity", {
    label: "Cloud identity",
    order: 10,
    emptyOptionLabel: "No cloud identity",
    slot: "workloadIdentity",
  }),
  pluginSection("encryption-key", {
    label: "Encryption key",
    order: 20,
    emptyOptionLabel: "No key",
  }),
  pluginSection("volume-encryption", {
    label: "Volume encryption",
    order: 30,
    emptyOptionLabel: "Unencrypted volumes",
    slot: "volumeEncryption",
  }),
  pluginSection("backup-storage", {
    label: "Backup storage",
    order: 40,
    emptyOptionLabel: "No backups",
    slot: "backupStorage",
  }),
  pluginSection("secondary-storage", {
    label: "Secondary storage",
    order: 50,
    slot: "secondaryStorage",
  }),
  pluginSection("access", {
    label: "Access",
    order: 60,
    emptyOptionLabel: "No access from outside",
    slot: "externalAccess",
    appliesTo: ["CamundaClusterClaim", "CamundaManagementClusterClaim"],
  }),
  pluginSection("connectors-permissions", {
    label: "Connectors permissions",
    order: 70,
    emptyOptionLabel: "No cloud permissions",
    slot: "connectorsIdentity",
  }),
  pluginSection("management-database", {
    label: "Databases",
    order: 80,
    slot: "managementDatabases",
    appliesTo: ["CamundaManagementClusterClaim"],
  }),
];

/**
 * registrationSources returns the sources of the Plugins page after a fresh
 * install: the core sections, the six built-in registrations, the
 * Environment production with the namespace team-orders, and one request of
 * existing-secondary-storage there. overrides replaces whole lists.
 */
export function registrationSources(
  overrides: Partial<RegistrationSources> = {},
): RegistrationSources {
  return {
    sections: CORE_SECTIONS.map((s) => structuredClone(s)),
    registrations: [
      builtin(
        "elasticsearch",
        "New Elasticsearch cluster",
        "secondary-storage",
        {
          mode: "Default",
          claimEditable: ["overrides"],
          outputs: [SECONDARY_STORAGE_OUTPUT],
        },
      ),
      builtin("database-server", "New PostgreSQL server", "secondary-storage", {
        claimEditable: ["overrides"],
        outputs: [SECONDARY_STORAGE_OUTPUT],
      }),
      builtin(
        "existing-secondary-storage",
        "Existing secondary storage",
        "secondary-storage",
        { outputs: [SECONDARY_STORAGE_OUTPUT] },
      ),
      builtin("existing-object-storage", "Existing bucket", "backup-storage", {
        outputs: [
          {
            key: "objectStorageConfig",
            description:
              "The name of the ObjectStorageConfig that the cluster writes its backups to.",
          },
        ],
      }),
      builtin(
        "management-database-server",
        "New PostgreSQL server",
        "management-database",
        { mode: "Default", outputs: DATABASE_OUTPUTS },
      ),
      builtin(
        "existing-database-configs",
        "Existing databases",
        "management-database",
        { outputs: DATABASE_OUTPUTS },
      ),
    ],
    requests: [
      pluginRequest("team-orders", "orders-secondary-storage", {
        registration: "existing-secondary-storage",
        section: "secondary-storage",
        claimRef: { kind: "CamundaClusterClaim", name: "orders" },
        target: { kind: "CamundaCluster", name: "orders" },
      }),
    ],
    namespaces: [
      labelledNamespace("camunda-system"),
      labelledNamespace("team-orders", "production"),
    ],
    environments: [
      fleetEnvironment("production", { displayName: "Production" }),
    ],
    ...overrides,
  };
}
