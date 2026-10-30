// This module runs in the browser and on the server, so it imports nothing.

/**
 * ENVIRONMENT_LABEL on a namespace names the Environment that the namespace
 * belongs to. It is LabelEnvironment of api/fleet/v1alpha1.
 */
export const ENVIRONMENT_LABEL = "fleet.camunda.io/environment";

/** CLAIM_LABEL names the claim that made an object. It is LabelClaim of api/fleet/v1alpha1. */
export const CLAIM_LABEL = "fleet.camunda.io/claim";

/** REVISION_ANNOTATION is AnnotationRevision of api/fleet/v1alpha1. */
export const REVISION_ANNOTATION = "fleet.camunda.io/revision";

/** SECTION_LABEL names the section of a PluginRequest. It is LabelSection of api/plugins/v1alpha1. */
export const SECTION_LABEL = "plugins.camunda.io/section";

/** ACCESS_SECTION is the section of the access request of a claim. It is SectionAccess of internal/pluginstartup. */
export const ACCESS_SECTION = "access";

/**
 * BUILTIN_CONTROLLER is the controller name of the built-in registrations.
 * It is ControllerName of internal/pluginstartup.
 */
export const BUILTIN_CONTROLLER = "camunda-operator";

/**
 * CLUSTER_LABEL on a pod holds labels.OwnerName of its CamundaCluster. It is
 * ClusterKey of pkg/labels.
 */
export const CLUSTER_LABEL = "camunda.io/cluster";

/** COMPONENT_LABEL on a pod names its component. It is ComponentKey of pkg/labels. */
export const COMPONENT_LABEL = "camunda.io/component";

/**
 * OPTIMIZE_UID_LABEL on a pod of a CamundaOptimize holds the UID of that
 * CamundaOptimize. It is OptimizeUIDKey of pkg/labels.
 */
export const OPTIMIZE_UID_LABEL = "camunda.io/optimize-uid";

/**
 * OPTIMIZE_COMPONENTS are the COMPONENT_LABEL values of the pods of a
 * CamundaOptimize. They are ComponentWebapp and ComponentImporter of
 * pkg/components/camundaoptimize.
 */
export const OPTIMIZE_COMPONENTS = [
  "optimize-webapp",
  "optimize-importer",
] as const;

/**
 * BACKUP_SCHEDULE_LABEL on a backup holds labels.OwnerName of the
 * BackupSchedule that made it. It is BackupScheduleKey of pkg/labels.
 */
export const BACKUP_SCHEDULE_LABEL = "camunda.io/backup-schedule";

/**
 * MANAGEMENT_CLUSTER_ANNOTATION on a CamundaCluster names the plane that
 * serves it, as "<namespace>/<name>". It is ManagementClusterKey of
 * pkg/labels.
 */
export const MANAGEMENT_CLUSTER_ANNOTATION = "camunda.io/management-cluster";

/** isManaged says whether a claim made the object: it carries CLAIM_LABEL with a value. */
export function isManaged(object: {
  metadata?: { labels?: Record<string, string> };
}): boolean {
  return Boolean(object.metadata?.labels?.[CLAIM_LABEL]);
}

// The label rules of Kubernetes: a name of at most 63 characters, and a key
// with an optional DNS subdomain prefix of at most 253 characters.
const NAME = /^[A-Za-z0-9]([-A-Za-z0-9_.]{0,61}[A-Za-z0-9])?$/;
const PREFIX =
  /^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$/;

/** isLabelKey says whether key is a valid Kubernetes label key: name or prefix/name. */
export function isLabelKey(key: string): boolean {
  const parts = key.split("/");
  if (parts.length > 2) return false;
  const name = parts.at(-1) ?? "";
  const prefix = parts.length === 2 ? parts[0] : undefined;
  return (
    NAME.test(name) &&
    (prefix === undefined || (prefix.length <= 253 && PREFIX.test(prefix)))
  );
}

/** isLabelValue says whether value is a valid Kubernetes label value. The empty value is valid. */
export function isLabelValue(value: string): boolean {
  return value === "" || NAME.test(value);
}

const DNS_LABEL = "[a-z0-9]([-a-z0-9]*[a-z0-9])?";
const DNS_LABEL_RE = new RegExp(`^${DNS_LABEL}$`);
const DNS_SUBDOMAIN_RE = new RegExp(`^${DNS_LABEL}(\\.${DNS_LABEL})*$`);

/** isDNSLabel says whether value is a DNS-1123 label, such as a namespace name. */
export function isDNSLabel(value: string): boolean {
  return value.length <= 63 && DNS_LABEL_RE.test(value);
}

/** isDNSSubdomain says whether value is a DNS-1123 subdomain, such as the name of most objects. */
export function isDNSSubdomain(value: string): boolean {
  return value.length <= 253 && DNS_SUBDOMAIN_RE.test(value);
}

/** LabelSelector is a metav1.LabelSelector. */
export type LabelSelector = {
  matchLabels?: Record<string, string>;
  matchExpressions?: { key: string; operator: string; values?: string[] }[];
};

/**
 * labelSelectorString writes selector in the text form of a Kubernetes
 * label selector, as in "app=web,tier in (a,b),!legacy". No selector and an
 * empty one give "", which selects every object. It throws for an operator
 * other than In, NotIn, Exists, and DoesNotExist.
 */
export function labelSelectorString(
  selector: LabelSelector | undefined,
): string {
  const labels = Object.entries(selector?.matchLabels ?? {})
    .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
    .map(([key, value]) => `${key}=${value}`);
  const expressions = (selector?.matchExpressions ?? []).map(
    ({ key, operator, values = [] }) => {
      // A set with only the empty value reads as "key in ()", which a
      // selector parser refuses, so it is written as key= or key!=.
      const empty = values.length === 1 && values[0] === "";
      switch (operator) {
        case "In":
          if (empty) return `${key}=`;
          return `${key} in (${values.join(",")})`;
        case "NotIn":
          if (empty) return `${key}!=`;
          return `${key} notin (${values.join(",")})`;
        case "Exists":
          return key;
        case "DoesNotExist":
          return `!${key}`;
        default:
          throw new Error(
            `The label selector operator ${operator} of key ${key} is not valid`,
          );
      }
    },
  );
  return [...labels, ...expressions].join(",");
}

/**
 * matchesSelector says whether labels match selector, as Kubernetes
 * decides it. An unset or empty selector matches every object. NotIn and
 * DoesNotExist match an object without the key. An unknown operator
 * matches nothing.
 */
export function matchesSelector(
  selector: LabelSelector | undefined,
  labels: Record<string, string> | undefined,
): boolean {
  const have = labels ?? {};
  const matchLabels = Object.entries(selector?.matchLabels ?? {}).every(
    ([k, v]) => Object.hasOwn(have, k) && have[k] === v,
  );
  return (
    matchLabels &&
    (selector?.matchExpressions ?? []).every(
      ({ key, operator, values = [] }) => {
        const present = Object.hasOwn(have, key);
        switch (operator) {
          case "In":
            return present && values.includes(have[key]);
          case "NotIn":
            return !present || !values.includes(have[key]);
          case "Exists":
            return present;
          case "DoesNotExist":
            return !present;
          default:
            return false;
        }
      },
    )
  );
}
